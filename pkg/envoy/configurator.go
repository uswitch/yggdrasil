package envoy

import (
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tcache "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/sirupsen/logrus"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
)

type Certificate struct {
	Hosts []string `json:"hosts"`
	Cert  string   `json:"cert"`
	Key   string   `json:"key"`
}

type UpstreamHealthCheck struct {
	Timeout            time.Duration `json:"timeout"`
	Interval           time.Duration `json:"interval"`
	UnhealthyThreshold uint32        `json:"unhealthyThreshold"`
	HealthyThreshold   uint32        `json:"healtyThreshold"`
}

type DefaultTimeouts struct {
	Cluster time.Duration
	Route   time.Duration
	PerTry  time.Duration
}

type HttpExtAuthz struct {
	Cluster             string        `json:"cluster"`
	Timeout             time.Duration `json:"timeout"`
	MaxRequestBytes     uint32        `json:"maxRequestBytes"`
	AllowPartialMessage bool          `json:"allowPartialMessage"`
	PackAsBytes         bool          `json:"packAsBytes"`
	FailureModeAllow    bool          `json:"FailureModeAllow"`
}

type HttpGrpcLogger struct {
	Name            string        `json:"name"`
	Cluster         string        `json:"cluster"`
	Timeout         time.Duration `json:"timeout"`
	RequestHeaders  []string      `json:"requestHeaders"`
	ResponseHeaders []string      `json:"responseHeaders"`
}

type AccessLogger struct {
	Format map[string]interface{} `json:"format"`
}

// KubernetesConfigurator takes a given Ingress Class and lister to find only ingresses of that class
type KubernetesConfigurator struct {
	ingressClasses             []string
	nodeID                     string
	syncSecrets                bool
	accessLog                  string
	certificates               []Certificate
	trustCA                    string
	upstreamPort               uint32
	envoyListenPort            uint32
	envoyListenerIpv4Address   []string
	outlierPercentage          int32
	hostSelectionRetryAttempts int64
	upstreamHealthCheck        UpstreamHealthCheck
	useRemoteAddress           bool
	httpExtAuthz               HttpExtAuthz
	httpGrpcLogger             HttpGrpcLogger
	defaultTimeouts            DefaultTimeouts
	accessLogger               AccessLogger
	defaultRetryOn             string
	tracingProvider            string
	alpnProtocols              []string

	previousConfig  *envoyConfiguration
	listenerVersion string
	clusterVersion  string
	sync.Mutex
}

// NewKubernetesConfigurator returns a Kubernetes configurator given a lister and ingress class
func NewKubernetesConfigurator(nodeID string, certificates []Certificate, ca string, ingressClasses []string, accessLog string, options ...option) *KubernetesConfigurator {
	c := &KubernetesConfigurator{ingressClasses: ingressClasses, nodeID: nodeID, certificates: certificates, trustCA: ca, accessLog: accessLog}
	for _, opt := range options {
		opt(c)
	}
	return c
}

func (c *KubernetesConfigurator) ValidateAndFormatPath() {
	if c.accessLog == "" {
		logrus.Fatal("accessLog path cannot be empty")
	}

	// Clean the path and make it absolute
	c.accessLog = filepath.Clean(c.accessLog)
	absolutePath, err := filepath.Abs(c.accessLog)
	if err != nil {
		logrus.Fatalf("invalid path: %v", err)
	}
	c.accessLog = absolutePath

	// Ensure the path ends with a directory separator if it's a directory
	if strings.HasSuffix(c.accessLog, string(filepath.Separator)) {
		c.accessLog = string(filepath.Separator)
	}
}

// Generate creates a new snapshot
func (c *KubernetesConfigurator) Generate(ingresses []*k8s.Ingress, secrets []*v1.Secret) (cache.Snapshot, error) {
	c.Lock()
	defer c.Unlock()

	validIngresses := validIngressFilter(classFilter(ingresses, c.ingressClasses))
	config := translateIngresses(validIngresses, c.syncSecrets, secrets, c.defaultTimeouts, c.accessLog)

	vmatch, cmatch := config.equals(c.previousConfig)

	clusters := c.generateClusters(config)
	listeners, err := c.generateListeners(config)
	if err != nil {
		return cache.Snapshot{}, err
	}

	if !vmatch {
		c.listenerVersion = time.Now().String()
		listenerUpdates.Inc()
	}

	if !cmatch {
		c.clusterVersion = time.Now().String()
		clusterUpdates.Inc()
	}
	c.previousConfig = config

	snap := cache.Snapshot{}
	snap.Resources[tcache.Cluster] = cache.NewResources(c.clusterVersion, []tcache.Resource(clusters))
	snap.Resources[tcache.Listener] = cache.NewResources(c.listenerVersion, []tcache.Resource(listeners))
	return snap, nil
}

// NodeID returns the NodeID
func (c *KubernetesConfigurator) NodeID() string {
	return c.nodeID

}

var errNoCertificateMatch = errors.New("no certificate match")

func compareHosts(pattern, host string) bool {
	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) == len(hostParts) {
		for i := range patternParts {
			if patternParts[i] != "*" && patternParts[i] != hostParts[i] {
				return false
			}
		}
		return true
	}

	return false
}

func (c *KubernetesConfigurator) matchCertificateIndices(virtualHost *virtualHost) ([]int, error) {
	matchedIndicies := []int{}

	for idx, certificate := range c.certificates {
		for _, host := range certificate.Hosts {
			if host == "*" || compareHosts(host, virtualHost.Host) { // star matches everything unlike *.thing.com which only matches one level
				matchedIndicies = append(matchedIndicies, idx)
			}
		}

	}

	if len(matchedIndicies) > 0 {
		return matchedIndicies, nil
	}

	return []int{}, errNoCertificateMatch
}

func (c *KubernetesConfigurator) generateListeners(config *envoyConfiguration) ([]tcache.Resource, error) {
	var filterChains []*listener.FilterChain
	var err error
	if c.syncSecrets {
		filterChains, err = c.generateDynamicTLSFilterChains(config)
	} else if len(c.certificates) > 0 {
		filterChains, err = c.generateTLSFilterChains(config)
	} else {
		filterChains, err = c.generateHTTPFilterChain(config)
	}
	if err != nil {
		return []tcache.Resource{}, err
	}
	listener, err := makeListener(filterChains, c.envoyListenerIpv4Address, c.envoyListenPort)
	return []tcache.Resource{listener}, err
}

func (c *KubernetesConfigurator) generateDynamicTLSFilterChains(config *envoyConfiguration) ([]*listener.FilterChain, error) {
	filterChains := []*listener.FilterChain{}

	allVhosts := []*route.VirtualHost{}

	for _, virtualHost := range config.VirtualHosts {
		envoyVhost, err := makeVirtualHost(virtualHost, c.hostSelectionRetryAttempts, c.defaultRetryOn)
		if err != nil {
			return nil, err
		}
		allVhosts = append(allVhosts, envoyVhost)

		if virtualHost.TlsCert == "" || virtualHost.TlsKey == "" {
			if len(c.certificates) == 0 {
				logrus.Warnf("skipping vhost because of no certificate: %s", virtualHost.Host)
			} else {
				logrus.Infof("using default certificate for %s", virtualHost.Host)
			}
			continue
		}
		certificate := Certificate{
			Hosts: []string{virtualHost.Host},
			Cert:  virtualHost.TlsCert,
			Key:   virtualHost.TlsKey,
		}
		filterChain, err := c.makeFilterChain(certificate, []*route.VirtualHost{envoyVhost}, config.AccessLog)
		if err != nil {
			logrus.Warnf("error making filter chain: %v", err)
		}
		filterChains = append(filterChains, &filterChain)
	}

	if len(c.certificates) == 1 {
		defaultCert := Certificate{
			Hosts: []string{"*"},
			Cert:  c.certificates[0].Cert,
			Key:   c.certificates[0].Key,
		}
		if defaultFC, err := c.makeFilterChain(defaultCert, allVhosts, config.AccessLog); err != nil {
			logrus.Warnf("error making default filter chain: %v", err)
		} else {
			filterChains = append(filterChains, &defaultFC)
		}
	}

	return filterChains, nil
}

func (c *KubernetesConfigurator) generateHTTPFilterChain(config *envoyConfiguration) ([]*listener.FilterChain, error) {
	virtualHosts := []*route.VirtualHost{}
	for _, virtualHost := range config.VirtualHosts {
		vhost, err := makeVirtualHost(virtualHost, c.hostSelectionRetryAttempts, c.defaultRetryOn)
		if err != nil {
			return nil, err
		}
		virtualHosts = append(virtualHosts, vhost)
	}

	httpConnectionManager, err := c.makeConnectionManager(virtualHosts, config.AccessLog)
	if err != nil {
		return nil, err
	}
	anyHttpConfig, err := anypb.New(httpConnectionManager)
	if err != nil {
		log.Fatalf("failed to marshal HTTP config struct to typed struct: %s", err)
	}
	return []*listener.FilterChain{
		{
			Filters: []*listener.Filter{
				{
					Name:       "envoy.filters.network.http_connection_manager",
					ConfigType: &listener.Filter_TypedConfig{TypedConfig: anyHttpConfig},
				},
			},
		},
	}, nil
}

func (c *KubernetesConfigurator) generateTLSFilterChains(config *envoyConfiguration) ([]*listener.FilterChain, error) {
	virtualHostsForCertificates := make([][]*route.VirtualHost, len(c.certificates))

	for _, virtualHost := range config.VirtualHosts {
		certificateIndicies, err := c.matchCertificateIndices(virtualHost)
		if err != nil {
			log.Printf("error matching certificate for '%s': %v", virtualHost.Host, err)
		} else {
			for _, idx := range certificateIndicies {
				vhost, err := makeVirtualHost(virtualHost, c.hostSelectionRetryAttempts, c.defaultRetryOn)
				if err != nil {
					return nil, err
				}
				virtualHostsForCertificates[idx] = append(virtualHostsForCertificates[idx], vhost)
			}
		}
	}

	filterChains := []*listener.FilterChain{}
	for idx, certificate := range c.certificates {
		virtualHosts := virtualHostsForCertificates[idx]

		if len(virtualHosts) == 0 {
			continue
		}

		filterChain, err := c.makeFilterChain(certificate, virtualHosts, config.AccessLog)
		if err != nil {
			log.Printf("error making filter chain: %v", err)
		}

		filterChains = append(filterChains, &filterChain)
	}
	return filterChains, nil
}

func (c *KubernetesConfigurator) generateClusters(config *envoyConfiguration) []tcache.Resource {
	clusters := []tcache.Resource{}

	for _, cluster := range config.Clusters {
		addresses := makeAddresses(cluster.Hosts, c.upstreamPort)
		cluster := makeCluster(*cluster, c.trustCA, c.upstreamHealthCheck, c.outlierPercentage, addresses)
		clusters = append(clusters, cluster)
	}

	return clusters
}
