package envoy

import (
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uswitch/yggdrasil/pkg/k8s"
)

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
}

type virtualHost struct {
	Host            string
	UpstreamCluster string
	Timeout         time.Duration
	PerTryTimeout   time.Duration
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
		v.RetryOn == other.RetryOn
}

type cluster struct {
	Name            string
	VirtualHost     string
	HealthCheckPath string
	Timeout         time.Duration
	Hosts           []string
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

	if c.HealthCheckPath != other.HealthCheckPath {
		return false
	}

	if len(c.Hosts) != len(other.Hosts) {
		return false
	}

	sort.Strings(c.Hosts)
	sort.Strings(other.Hosts)

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

func newEnvoyIngress(host string) *envoyIngress {
	clusterName := strings.Replace(host, ".", "_", -1)
	return &envoyIngress{
		vhost: &virtualHost{
			Host:            host,
			UpstreamCluster: clusterName,
			Timeout:         (15 * time.Second),
			PerTryTimeout:   (5 * time.Second),
		},
		cluster: &cluster{
			Name:            clusterName,
			VirtualHost:     host,
			Hosts:           []string{},
			Timeout:         (30 * time.Second),
			HealthCheckPath: "",
		},
	}
}

func (ing *envoyIngress) addUpstream(host string) {
	ing.cluster.Hosts = append(ing.cluster.Hosts, host)
}

func (ing *envoyIngress) addHealthCheckPath(path string) {
	ing.cluster.HealthCheckPath = path
}

func (ing *envoyIngress) addTimeout(timeout time.Duration) {
	ing.cluster.Timeout = timeout
	ing.vhost.Timeout = timeout
	ing.vhost.PerTryTimeout = timeout
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

func translateIngresses(ingresses []*k8s.Ingress) *envoyConfiguration {
	cfg := &envoyConfiguration{}
	envoyIngresses := map[string]*envoyIngress{}

	for _, i := range ingresses {
		for _, j := range i.Upstreams {
			for _, ruleHost := range i.RulesHosts {
				_, ok := envoyIngresses[ruleHost]
				if !ok {
					envoyIngresses[ruleHost] = newEnvoyIngress(ruleHost)
				}

				envoyIngress := envoyIngresses[ruleHost]

				envoyIngress.addUpstream(j)

				if i.Annotations["yggdrasil.uswitch.com/healthcheck-path"] != "" {
					envoyIngress.addHealthCheckPath(i.Annotations["yggdrasil.uswitch.com/healthcheck-path"])
				}

				if i.Annotations["yggdrasil.uswitch.com/timeout"] != "" {
					timeout, err := time.ParseDuration(i.Annotations["yggdrasil.uswitch.com/timeout"])
					if err == nil {
						envoyIngress.addTimeout(timeout)
					}
				}
				envoyIngress.addRetryOn(i)
			}
		}
	}

	for _, ingress := range envoyIngresses {
		cfg.Clusters = append(cfg.Clusters, ingress.cluster)
		cfg.VirtualHosts = append(cfg.VirtualHosts, ingress.vhost)
	}

	numVhosts.Set(float64(len(cfg.VirtualHosts)))
	numClusters.Set(float64(len(cfg.Clusters)))

	return cfg
}
