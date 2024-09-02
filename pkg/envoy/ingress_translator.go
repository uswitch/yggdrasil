package envoy

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

type UpstreamInfo struct {
	RuleHost    string
	Upstream    string
	Namespace   string
	Class       string
	ClusterName string
	IngressName string
}

var previousUpstreams = make(map[string]UpstreamInfo)

func sortCluster(clusters []*cluster) {
	sort.Slice(clusters, func(i int, j int) bool {
		return clusters[i].identity() < clusters[j].identity()
	})
}

func ClustersEquals(a, b []*cluster) bool {
	if len(a) != len(b) {
		return false
	}

	sortCluster(a)
	sortCluster(b)

	for idx, cluster := range a {
		if !cluster.Equals(b[idx]) {
			return false
		}
	}

	return true
}

func sortVirtualHosts(hosts []*virtualHost) {
	sort.Slice(hosts, func(i int, j int) bool {
		return hosts[i].Host < hosts[j].Host
	})
}

func VirtualHostsEquals(a, b []*virtualHost) bool {
	if len(a) != len(b) {
		return false
	}

	sortVirtualHosts(a)
	sortVirtualHosts(b)

	for idx, hosts := range a {
		if !hosts.Equals(b[idx]) {
			return false
		}
	}

	return true
}

type envoyConfiguration struct {
	VirtualHosts []*virtualHost
	Clusters     []*cluster
	AccessLog    string
}

type virtualHost struct {
	Host            string
	UpstreamCluster string
	Timeout         time.Duration
	PerTryTimeout   time.Duration
	TlsKey          string
	TlsCert         string
	RetryOn         string
}

func (v *virtualHost) Equals(other *virtualHost) bool {
	if other == nil {
		return false
	}

	return v.Host == other.Host &&
		v.Timeout == other.Timeout &&
		v.UpstreamCluster == other.UpstreamCluster &&
		v.PerTryTimeout == other.PerTryTimeout &&
		v.TlsKey == other.TlsKey &&
		v.TlsCert == other.TlsCert &&
		v.RetryOn == other.RetryOn
}

type LBHost struct {
	Host   string
	Weight uint32
}

type cluster struct {
	Name            string
	VirtualHost     string
	HealthCheckPath string
	HealthCheckHost string // with Wildcard, the HealthCheck host can be different than the VirtualHost
	HttpVersion     string
	Timeout         time.Duration
	Hosts           []LBHost
}

func (c *cluster) identity() string {
	return c.Name
}

func (c *cluster) Equals(other *cluster) bool {
	if other == nil {
		return false
	}

	if c.Name != other.Name {
		return false
	}

	if c.Timeout != other.Timeout {
		return false
	}

	if c.VirtualHost != other.VirtualHost {
		return false
	}

	if c.HealthCheckHost != other.HealthCheckHost {
		return false
	}

	if c.HealthCheckPath != other.HealthCheckPath {
		return false
	}

	if len(c.Hosts) != len(other.Hosts) {
		return false
	}

	if c.HttpVersion != other.HttpVersion {
		return false
	}

	sort.Slice(c.Hosts[:], func(i, j int) bool {
		return c.Hosts[i].Host < c.Hosts[j].Host
	})
	sort.Slice(other.Hosts[:], func(i, j int) bool {
		return other.Hosts[i].Host < other.Hosts[j].Host
	})

	for i, host := range c.Hosts {
		if host != other.Hosts[i] {
			return false
		}
	}

	return true
}

func (cfg *envoyConfiguration) equals(oldCfg *envoyConfiguration) (vmatch bool, cmatch bool) {
	if oldCfg == nil {
		return false, false
	}
	return VirtualHostsEquals(cfg.VirtualHosts, oldCfg.VirtualHosts), ClustersEquals(cfg.Clusters, oldCfg.Clusters)
}

func classFilter(ingresses []*k8s.Ingress, ingressClass []string) (is []*k8s.Ingress) {
	for _, i := range ingresses {
		for _, class := range ingressClass {
			if i.Annotations["kubernetes.io/ingress.class"] == class ||
				(i.Class != nil && *i.Class == class) {
				is = append(is, i)
			}
		}
	}
	matchingIngresses.Set(float64(len(is)))
	return is
}

func validIngressFilter(ingresses []*k8s.Ingress) (vi []*k8s.Ingress) {
Ingress:
	for _, i := range ingresses {
		for _, u := range i.Upstreams {
			if u != "" {
				for _, h := range i.RulesHosts {
					if h != "" {
						vi = append(vi, i)
						continue Ingress
					}
				}
				logrus.Debugf("no host found in ingress config for: %+v in namespace: %+v", i.Name, i.Namespace)
				continue Ingress
			}
		}
		logrus.Debugf("no hostname or ip for loadbalancer found in ingress config for: %+v in namespace: %+v", i.Name, i.Namespace)
	}

	return vi
}

type envoyIngress struct {
	vhost   *virtualHost
	cluster *cluster
}

func newEnvoyIngress(host string, timeouts DefaultTimeouts) *envoyIngress {
	clusterName := strings.Replace(host, ".", "_", -1)
	return &envoyIngress{
		vhost: &virtualHost{
			Host:            host,
			UpstreamCluster: clusterName,
			Timeout:         timeouts.Route,
			PerTryTimeout:   timeouts.PerTry,
		},
		cluster: &cluster{
			Name:            clusterName,
			VirtualHost:     host,
			Hosts:           []LBHost{},
			Timeout:         timeouts.Cluster,
			HealthCheckPath: "",
			HealthCheckHost: host,
		},
	}
}

func (ing *envoyIngress) addUpstream(host string, weight uint32) {
	// Check if the host is already in the list
	// If we wan't to avoid using a for loop, maybe we could implement a Map for a faster lookup.
	// time complexity O(1) vs 0(n) for each iteration.
	for _, h := range ing.cluster.Hosts {
		if h.Host == host {
			// Host found, so we don't add the duplicate
			logrus.Debugf("Duplicate host found for upstream, not adding : %s for cluster : %s", host, ing.cluster.Name)
			return
		}
	}

	// No duplicate found, append the new host
	ing.cluster.Hosts = append(ing.cluster.Hosts, LBHost{Host: host, Weight: weight})
	logrus.Debugf("Host added on upstream list : %s for cluster : %s", host, ing.cluster.Name)
}

func (ing *envoyIngress) addHealthCheckPath(path string) {
	ing.cluster.HealthCheckPath = path
}

func (ing *envoyIngress) removeHealthCheckPath() {
	ing.cluster.HealthCheckPath = ""
}

func (ing *envoyIngress) addHealthCheckHost(host string) {
	ing.cluster.HealthCheckHost = host
}

func (ing *envoyIngress) removeHealthCheckHost() {
	ing.cluster.HealthCheckHost = ""
}

func (ing *envoyIngress) addTimeout(timeout time.Duration) {
	ing.cluster.Timeout = timeout
	ing.vhost.Timeout = timeout
	ing.vhost.PerTryTimeout = timeout
}

func (ing *envoyIngress) setClusterTimeout(timeout time.Duration) {
	ing.cluster.Timeout = timeout
}

func (ing *envoyIngress) setRouteTimeout(timeout time.Duration) {
	ing.vhost.Timeout = timeout
}

func (ing *envoyIngress) setPerTryTimeout(timeout time.Duration) {
	ing.vhost.PerTryTimeout = timeout
}

func (ing *envoyIngress) setUpstreamHttpVersion(version string) {
	ing.cluster.HttpVersion = version
}

// hostMatch returns true if tlsHost and ruleHost match, with wildcard support
//
// *.a.b ruleHost accepts tlsHost *.a.b but not a.a.b or a.b or a.a.a.b
// a.a.b ruleHost accepts tlsHost a.a.b and *.a.b but not *.a.a.b
func hostMatch(ruleHost, tlsHost string) bool {
	// TODO maybe cache the results for speedup
	pattern := strings.ReplaceAll(strings.ReplaceAll(tlsHost, ".", "\\."), "*", "(?:\\*|[a-z0-9][a-z0-9-_]*)")
	matched, err := regexp.MatchString("^"+pattern+"$", ruleHost)
	if err != nil {
		logrus.Errorf("error in ingress hostname comparison: %s", err.Error())
		return false
	}
	return matched
}

// getHostTlsSecret returns the tls secret configured for a given ingress host
func getHostTlsSecret(ingress *k8s.Ingress, host string, secrets []*v1.Secret) (*v1.Secret, error) {
	for _, tls := range ingress.TLS {
		// TODO prefer a.a.b tls secret over *.a.b for host a.a.b when both are configured
		if hostMatch(host, tls.Host) {
			for _, secret := range secrets {
				if secret.Namespace == ingress.Namespace &&
					secret.Name == tls.SecretName {
					return secret, nil
				}
			}
			return nil, fmt.Errorf("secret %s/%s not found for host '%s'", ingress.Namespace, tls.SecretName, host)
		}
	}
	return nil, fmt.Errorf("ingress %s/%s - %s has no tls secret configured", ingress.Namespace, ingress.Name, host)
}

// validateTlsSecret checks that the given secret holds valid tls certificate and key
func validateTlsSecret(secret *v1.Secret) (bool, error) {
	tlsCert, certOk := secret.Data["tls.crt"]
	tlsKey, keyOk := secret.Data["tls.key"]

	if !certOk || !keyOk {
		logrus.Infof("skipping certificate %s/%s: missing 'tls.crt' or 'tls.key'", secret.Namespace, secret.Name)
		return false, nil
	}
	if len(tlsCert) == 0 || len(tlsKey) == 0 {
		logrus.Infof("skipping certificate %s/%s: empty 'tls.crt' or 'tls.key'", secret.Namespace, secret.Name)
		return false, nil
	}

	// discard P-384 EC private keys
	// see https://github.com/envoyproxy/envoy/issues/10855
	block, _ := pem.Decode(tlsCert)
	if block == nil {
		return false, fmt.Errorf("error parsing x509 certificate - no PEM block found")
	}
	x509crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("error parsing x509 certificate: %s", err.Error())
	}
	if x509crt.PublicKeyAlgorithm == x509.ECDSA {
		ecdsaPub, ok := x509crt.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return false, fmt.Errorf("error in *ecdsa.PublicKey type assertion")
		}
		if ecdsaPub.Curve.Params().BitSize > 256 {
			logrus.Infof("skipping ECDSA %s certificate %s/%s: only P-256 certificates are supported", ecdsaPub.Curve.Params().Name, secret.Namespace, secret.Name)
			return false, nil
		}
	}
	return true, nil
}

func (envoyIng *envoyIngress) addRetryOn(ingress *k8s.Ingress) {
	if ingress.Annotations["yggdrasil.uswitch.com/retry-on"] != "" {
		retryOn := ingress.Annotations["yggdrasil.uswitch.com/retry-on"]
		if !ValidateEnvoyRetryOn(retryOn) {
			logrus.Warnf("invalid retry-on parameter for ingress %s/%s: %s", ingress.Namespace, ingress.Name, retryOn)
			return
		}
		envoyIng.vhost.RetryOn = retryOn
	}
}

// isWildcard checks if the given host rule is a wildcard.
func isWildcard(ruleHost string) bool {
	// Check if the ruleHost starts with '*.'
	return strings.HasPrefix(ruleHost, "*.")
}

func validateSubdomain(ruleHost, host string) bool {
	if strings.HasPrefix(ruleHost, "*.") {
		ruleHost = ruleHost[2:]
	}
	return strings.HasSuffix(host, ruleHost)
}

func translateIngresses(ingresses []*k8s.Ingress, syncSecrets bool, secrets []*v1.Secret, timeouts DefaultTimeouts, accessLog string) *envoyConfiguration {
	cfg := &envoyConfiguration{}
	envoyIngresses := map[string]*envoyIngress{}
	ruleHostToIngresses := map[string][]*k8s.Ingress{}
	currentUpstreams := make(map[string]UpstreamInfo)

	for _, i := range ingresses {
		for _, ruleHost := range i.RulesHosts {
			ruleHostToIngresses[ruleHost] = append(ruleHostToIngresses[ruleHost], i)
		}
	}

	for ruleHost, ingressList := range ruleHostToIngresses {
		isWildcard := isWildcard(ruleHost)

		if _, ok := envoyIngresses[ruleHost]; !ok {
			envoyIngresses[ruleHost] = newEnvoyIngress(ruleHost, timeouts)
		}

		envoyIngress := envoyIngresses[ruleHost]

		// Determine if any ingress is not in maintenance mode
		hasNonMaintenance := false
		for _, ingress := range ingressList {
			if !ingress.Maintenance {
				hasNonMaintenance = true
				break
			}
		}

		// Add upstreams based on maintenance status
		for _, ingress := range ingressList {
			for _, j := range ingress.Upstreams {
				// Skip this upstream if cluster is in maintenance but keep it if no other cluster can serve it
				if !hasNonMaintenance || !ingress.Maintenance {
					// Check if the upstream is already added
					exists := false
					for _, host := range envoyIngress.cluster.Hosts {
						if host.Host == j {
							exists = true
							break
						}
					}
					if exists {
						continue // skip if the upstream already exists
					}

					class := "none"
					if ingress.Class != nil {
						class = *ingress.Class
					}

					// Add upstream
					if weight64, err := strconv.ParseUint(ingress.Annotations["yggdrasil.uswitch.com/weight"], 10, 32); err == nil {
						if weight64 != 0 {
							envoyIngress.addUpstream(j, uint32(weight64))
						}
					} else {
						envoyIngress.addUpstream(j, 1)
					}
					upstreamKey := fmt.Sprintf("%s-%s", ruleHost, j)
					currentUpstreams[upstreamKey] = UpstreamInfo{
						RuleHost:    strings.ReplaceAll(ruleHost, ".", "_"),
						Upstream:    j,
						Namespace:   ingress.Namespace,
						Class:       class,
						ClusterName: ingress.KubernetesClusterName,
						IngressName: ingress.Name,
					}

					EnvoyUpstreamInfo.WithLabelValues(strings.ReplaceAll(ruleHost, ".", "_"), j, ingress.Namespace, class, ingress.KubernetesClusterName, ingress.Name).Set(float64(1))
				} else {
					logrus.Warnf("Endpoint is in maintenance mode, upstream %s will not be added for host %s", j, ruleHost)
				}
			}

			if isWildcard {
				if ingress.Annotations["yggdrasil.uswitch.com/healthcheck-host"] != "" {
					envoyIngress.addHealthCheckHost(ingress.Annotations["yggdrasil.uswitch.com/healthcheck-host"])
					if !validateSubdomain(ruleHost, envoyIngress.cluster.HealthCheckHost) {
						logrus.Warnf("Healthcheck %s is not on the same subdomain for %s, annotation will be skipped", envoyIngress.cluster.HealthCheckHost, ruleHost)
						envoyIngress.cluster.HealthCheckHost = ruleHost
					}
				} else {
					logrus.Warnf("Be careful, healthcheck can't work for wildcard host : %s", envoyIngress.cluster.HealthCheckHost)
				}
			}

			if ingress.Annotations["yggdrasil.uswitch.com/healthcheck-path"] != "" {
				envoyIngress.addHealthCheckPath(ingress.Annotations["yggdrasil.uswitch.com/healthcheck-path"])
			}

			if ingress.Annotations["yggdrasil.uswitch.com/timeout"] != "" {
				timeout, err := time.ParseDuration(ingress.Annotations["yggdrasil.uswitch.com/timeout"])
				if err == nil {
					envoyIngress.addTimeout(timeout)
				}
			}

			if ingress.Annotations["yggdrasil.uswitch.com/cluster-timeout"] != "" {
				timeout, err := time.ParseDuration(ingress.Annotations["yggdrasil.uswitch.com/cluster-timeout"])
				if err == nil {
					envoyIngress.setClusterTimeout(timeout)
				}
			}

			if ingress.Annotations["yggdrasil.uswitch.com/route-timeout"] != "" {
				timeout, err := time.ParseDuration(ingress.Annotations["yggdrasil.uswitch.com/route-timeout"])
				if err == nil {
					envoyIngress.setRouteTimeout(timeout)
				}
			}

			if ingress.Annotations["yggdrasil.uswitch.com/per-try-timeout"] != "" {
				timeout, err := time.ParseDuration(ingress.Annotations["yggdrasil.uswitch.com/per-try-timeout"])
				if err == nil {
					envoyIngress.setPerTryTimeout(timeout)
				}
			}

			if ingress.Annotations["yggdrasil.uswitch.com/upstream-http-version"] != "" {
				// TODO validate, add error path
				envoyIngress.setUpstreamHttpVersion(ingress.Annotations["yggdrasil.uswitch.com/upstream-http-version"])
			}

			envoyIngress.addRetryOn(ingress)

			if syncSecrets && envoyIngress.vhost.TlsKey == "" && envoyIngress.vhost.TlsCert == "" {
				if hostTlsSecret, err := getHostTlsSecret(ingress, ruleHost, secrets); err != nil {
					logrus.Infof(err.Error())
				} else {
					valid, err := validateTlsSecret(hostTlsSecret)
					if err != nil {
						logrus.Warnf("secret %s/%s is not valid: %s", hostTlsSecret.Namespace, hostTlsSecret.Name, err.Error())
					} else if valid {
						envoyIngress.vhost.TlsKey = string(hostTlsSecret.Data["tls.key"])
						envoyIngress.vhost.TlsCert = string(hostTlsSecret.Data["tls.crt"])
					}
				}
			}
		}
	}

	// Identify and remove upstreams that no longer exist
	for upstreamKey, info := range previousUpstreams {
		if _, exists := currentUpstreams[upstreamKey]; !exists {
			EnvoyUpstreamInfo.DeleteLabelValues(info.RuleHost, info.Upstream, info.Namespace, info.Class, info.ClusterName, info.IngressName)
		}
	}

	// Update the previous state
	previousUpstreams = currentUpstreams

	for _, ingress := range envoyIngresses {
		cfg.Clusters = append(cfg.Clusters, ingress.cluster)
		cfg.VirtualHosts = append(cfg.VirtualHosts, ingress.vhost)
		cfg.AccessLog = accessLog
	}

	numVhosts.Set(float64(len(cfg.VirtualHosts)))
	numClusters.Set(float64(len(cfg.Clusters)))

	return cfg
}
