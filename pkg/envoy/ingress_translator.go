package envoy

import (
	"sort"

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
	Host string
}

func (v *virtualHost) Equals(other *virtualHost) bool {
	if other == nil {
		return false
	}

	return v.Host == other.Host
}

type cluster struct {
	Name            string
	HealthCheckPath string
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

func classFilter(ingresses []v1beta1.Ingress, ingressClass string) []v1beta1.Ingress {
	is := make([]v1beta1.Ingress, 0)

	for _, i := range ingresses {
		if i.GetAnnotations()["kubernetes.io/ingress.class"] == ingressClass {
			is = append(is, i)
		}
	}

	return is
}

func translateIngresses(ingresses []v1beta1.Ingress) *envoyConfiguration {
	cfg := &envoyConfiguration{}
	ingressToStatusHosts := map[string][]string{}
	ingressHealthChecks := map[string]string{}

	for _, i := range ingresses {
		for _, j := range i.Status.LoadBalancer.Ingress {
			for _, rule := range i.Spec.Rules {
				ingressToStatusHosts[rule.Host] = append(ingressToStatusHosts[rule.Host], j.Hostname)
				if i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"] != "" {
					ingressHealthChecks[rule.Host] = i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"]
				} else {
					ingressHealthChecks[rule.Host] = "/"
				}
			}
		}
	}

	for ingress, hosts := range ingressToStatusHosts {
		cfg.Clusters = append(cfg.Clusters, &cluster{Name: ingress, Hosts: hosts, HealthCheckPath: ingressHealthChecks[ingress]})
		cfg.VirtualHosts = append(cfg.VirtualHosts, &virtualHost{Host: ingress})
	}

	return cfg
}
