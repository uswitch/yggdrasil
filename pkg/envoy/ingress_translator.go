package envoy

import (
	"sort"
	"time"

	"k8s.io/api/extensions/v1beta1"
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
	Host          string
	Timeout       time.Duration
	PerTryTimeout time.Duration
}

func (v *virtualHost) Equals(other *virtualHost) bool {
	if other == nil {
		return false
	}

	return v.Host == other.Host &&
		v.Timeout == other.Timeout &&
		v.PerTryTimeout == other.PerTryTimeout
}

type cluster struct {
	Name            string
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

func classFilter(ingresses []v1beta1.Ingress, ingressClass []string) []v1beta1.Ingress {
	is := make([]v1beta1.Ingress, 0)

	for _, i := range ingresses {
		for _, class := range ingressClass {
			if i.GetAnnotations()["kubernetes.io/ingress.class"] == class {
				is = append(is, i)
			}
		}
	}
	matchingIngresses.Set(float64(len(is)))
	return is
}

type envoyIngress struct {
	vhost   *virtualHost
	cluster *cluster
}

func newEnvoyIngress(host string) *envoyIngress {
	return &envoyIngress{
		vhost: &virtualHost{
			Host:          host,
			Timeout:       (15 * time.Second),
			PerTryTimeout: (5 * time.Second),
		},
		cluster: &cluster{
			Name:            host,
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

func translateIngresses(ingresses []v1beta1.Ingress) *envoyConfiguration {
	cfg := &envoyConfiguration{}
	envoyIngresses := map[string]*envoyIngress{}

	for _, i := range ingresses {
		for _, j := range i.Status.LoadBalancer.Ingress {
			for _, rule := range i.Spec.Rules {
				_, ok := envoyIngresses[rule.Host]
				if !ok {
					envoyIngresses[rule.Host] = newEnvoyIngress(rule.Host)
				}

				envoyIngress := envoyIngresses[rule.Host]
				envoyIngress.addUpstream(j.Hostname)

				if i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"] != "" {
					envoyIngress.addHealthCheckPath(i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"])
				}

				if i.GetAnnotations()["yggdrasil.uswitch.com/timeout"] != "" {
					timeout, err := time.ParseDuration(i.GetAnnotations()["yggdrasil.uswitch.com/timeout"])
					if err == nil {
						envoyIngress.addTimeout(timeout)
					}
				}
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
