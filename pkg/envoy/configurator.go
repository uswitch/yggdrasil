package envoy

import (
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"k8s.io/api/extensions/v1beta1"
)

type Certificate struct {
	Hosts []string `json:"hosts"`
	Cert  string   `json:"cert"`
	Key   string   `json:"key"`
}

//KubernetesConfigurator takes a given Ingress Class and lister to find only ingresses of that class
type KubernetesConfigurator struct {
	ingressClasses []string
	nodeID         string
	certificate    Certificate
	trustCA        string

	previousConfig  *envoyConfiguration
	listenerVersion string
	clusterVersion  string
	sync.Mutex
}

//NewKubernetesConfigurator returns a Kubernetes configurator given a lister and ingress class
func NewKubernetesConfigurator(nodeID string, certificate Certificate, ca string, ingressClasses []string) *KubernetesConfigurator {
	return &KubernetesConfigurator{ingressClasses: ingressClasses, nodeID: nodeID, certificate: certificate, trustCA: ca}
}

//Generate creates a new snapshot
func (c *KubernetesConfigurator) Generate(ingresses []v1beta1.Ingress) cache.Snapshot {
	config := translateIngresses(classFilter(ingresses, c.ingressClasses))
	return c.generateSnapshot(config)
}

//NodeID returns the NodeID
func (c *KubernetesConfigurator) NodeID() string {
	return c.nodeID
}

func (c *KubernetesConfigurator) generateSnapshot(config *envoyConfiguration) cache.Snapshot {
	c.Lock()
	defer c.Unlock()

	vmatch, cmatch := config.equals(c.previousConfig)

	virtualHosts := []route.VirtualHost{}
	for _, virtualHost := range config.VirtualHosts {
		virtualHosts = append(virtualHosts, makeVirtualHost(virtualHost))
	}
	listener := makeListener(virtualHosts, c.certificate.Cert, c.certificate.Key)

	clusterItems := []cache.Resource{}
	for _, cluster := range config.Clusters {
		addresses := makeAddresses(cluster.Hosts)
		cluster := makeCluster(cluster.Name, c.trustCA, cluster.HealthCheckPath, cluster.Timeout, addresses)
		clusterItems = append(clusterItems, cluster)
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
	clusters := cache.NewResources(c.clusterVersion, clusterItems)
	listeners := cache.NewResources(c.listenerVersion, listener)
	return cache.Snapshot{
		Clusters:  clusters,
		Listeners: listeners,
	}
}
