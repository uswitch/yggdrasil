package envoy

import (
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/uswitch/yggdrasil/pkg/k8s"
)

//KubernetesConfigurator takes a given Ingress Class and lister to find only ingresses of that class
type KubernetesConfigurator struct {
	lister       k8s.IngressLister
	ingressClass string
	nodeID       string
	cert         string
	key          string
	trustCA      string

	previousConfig  *envoyConfiguration
	listenerVersion string
	clusterVersion  string
	sync.Mutex
}

//NewKubernetesConfigurator returns a Kubernetes configurator given a lister and ingress class
func NewKubernetesConfigurator(lister k8s.IngressLister, ingressClass, nodeID, cert, key, ca string) *KubernetesConfigurator {
	return &KubernetesConfigurator{lister: lister, ingressClass: ingressClass, nodeID: nodeID, cert: cert, key: key, trustCA: ca}
}

//Generate creates a new snapshot
func (c *KubernetesConfigurator) Generate() (cache.Snapshot, error) {
	ingresses, err := c.lister.List()

	if err != nil {
		return cache.Snapshot{}, err
	}

	config := translateIngresses(classFilter(ingresses, c.ingressClass))
	return c.generateSnapshot(config), nil
}

//NodeID returns the NodeID
func (c *KubernetesConfigurator) NodeID() string {
	return c.nodeID
}

func (c *KubernetesConfigurator) generateSnapshot(config *envoyConfiguration) cache.Snapshot {
	c.Lock()
	defer c.Unlock()

	vmatch, cmatch := config.equals(c.previousConfig)

	timeout := time.Second * 5
	virtualHosts := []route.VirtualHost{}
	for _, virtualHost := range config.VirtualHosts {
		virtualHosts = append(virtualHosts, makeVirtualHost(virtualHost.Host, timeout))
	}
	listener := makeListener(virtualHosts, c.cert, c.key)

	clusterItems := []cache.Resource{}
	for _, cluster := range config.Clusters {
		addresses := makeAddresses(cluster.Hosts)
		cluster := makeCluster(cluster.Name, c.trustCA, cluster.HealthCheckPath, addresses)
		clusterItems = append(clusterItems, cluster)
	}

	if !vmatch {
		c.listenerVersion = time.Now().String()
	}

	if !cmatch {
		c.clusterVersion = time.Now().String()
	}
	c.previousConfig = config
	clusters := cache.NewResources(c.clusterVersion, clusterItems)
	listeners := cache.NewResources(c.listenerVersion, listener)
	return cache.Snapshot{
		Clusters:  clusters,
		Listeners: listeners,
	}
}
