package envoy

import (
	"fmt"
	"sort"
	"strings"
	"time"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/networking/v1"
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
	Routes        []*Localroute
	Timeout       time.Duration
	PerTryTimeout time.Duration
}

type Localroute struct {
	UpstreamCluster string
	Route           *envoy_route_v3.RouteMatch
}

type key struct {
	host string
	path string
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

func classFilter(ingresses []v1.Ingress, ingressClass []string) []v1.Ingress {
	is := make([]v1.Ingress, 0)
	for _, i := range ingresses {
		//	fmt.Println(i.Spec.IngressClassName)
		if i.Spec.IngressClassName != nil {
			for _, class := range ingressClass {
				if i.GetAnnotations()["kubernetes.io/ingress.class"] == class || *i.Spec.IngressClassName == class {
					is = append(is, i)
				} else {
					logrus.Debugf("the ingress class of %s is not %d", i.Name, &class)
				}
			}
		}
	}
	matchingIngresses.Set(float64(len(is)))
	return is
}

func validIngressFilter(ingresses []v1.Ingress) []v1.Ingress {
	vi := make([]v1.Ingress, 0)

Ingress:
	for _, i := range ingresses {
		for _, j := range i.Status.LoadBalancer.Ingress {
			if j.Hostname != "" || j.IP != "" {
				for _, k := range i.Spec.Rules {
					if k.Host != "" {
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

func newVirtualHost(host string) *virtualHost {
	return &virtualHost{
		Host:          host,
		Timeout:       (15 * time.Second),
		PerTryTimeout: (5 * time.Second),
	}
}
func newCluster(host string) *cluster {
	clusterName := strings.Replace(host, ".", "_", -1)
	return &cluster{
		Name:            clusterName,
		VirtualHost:     host,
		Hosts:           []string{},
		Timeout:         (30 * time.Second),
		HealthCheckPath: "",
	}
}
func (ing *cluster) addUpstream(host string) {
	ing.Hosts = append(ing.Hosts, host)
}

func (ing *cluster) addHealthCheckPath(path string) {
	ing.HealthCheckPath = path
}

func (ing *virtualHost) addVhostTimeout(timeout time.Duration) {
	ing.Timeout = timeout
	ing.PerTryTimeout = timeout
}

func (ing *virtualHost) addlocalroute(clusternmae string, route *envoy_route_v3.RouteMatch) {
	make := Localroute{Route: route, UpstreamCluster: clusternmae}
	ing.Routes = append(ing.Routes, &make)
}

func (ing *cluster) addclustername(clustername string) {
	ing.Name = clustername
}

func translateIngresses(ingresses []v1.Ingress) *envoyConfiguration {
	cfg := &envoyConfiguration{}

	clusters := map[key]*cluster{}
	virtualhosts := map[string]*virtualHost{}

	for _, i := range ingresses {
		for _, j := range i.Status.LoadBalancer.Ingress {
			for _, rule := range i.Spec.Rules {

				_, ok := virtualhosts[rule.Host]
				if !ok {
					virtualhosts[rule.Host] = newVirtualHost(rule.Host)
				}

				virtualHost := virtualhosts[rule.Host]

				for _, httppath := range httppaths(rule) {

					_, exist := clusters[key{host: rule.Host, path: httppath.Path}]
					if !exist {
						clusters[key{host: rule.Host, path: httppath.Path}] = newCluster(rule.Host)
					}
					cluster := clusters[key{host: rule.Host, path: httppath.Path}]

					path := stringOrDefault(httppath.Path, "/")
					// Default to implementation specific path matching if not set.
					pathType := derefPathTypeOr(httppath.PathType, v1.PathTypeImplementationSpecific)
					clustername := fmt.Sprint(strings.Replace(rule.Host, ".", "_", -1), stringTohash(rule))
					virtualHost.addlocalroute(clustername, RouteMatch(Pathtranslate(path, pathType)))
					cluster.addclustername(clustername)
					fmt.Println(clustername, path)
					if j.Hostname != "" {
						cluster.addUpstream(j.Hostname)
					} else {
						cluster.addUpstream(j.IP)
					}

					if i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"] != "" {
						cluster.addHealthCheckPath(i.GetAnnotations()["yggdrasil.uswitch.com/healthcheck-path"])
					}
				}

				if i.GetAnnotations()["yggdrasil.uswitch.com/timeout"] != "" {
					timeout, err := time.ParseDuration(i.GetAnnotations()["yggdrasil.uswitch.com/timeout"])
					if err == nil {
						virtualHost.addVhostTimeout(timeout)
					}
				}
			}
		}
	}

	for _, cluster := range clusters {
		cfg.Clusters = append(cfg.Clusters, cluster)
	}
	for _, virtualhost := range virtualhosts {
		cfg.VirtualHosts = append(cfg.VirtualHosts, virtualhost)
	}
	numVhosts.Set(float64(len(cfg.VirtualHosts)))
	numClusters.Set(float64(len(cfg.Clusters)))

	return cfg
}
