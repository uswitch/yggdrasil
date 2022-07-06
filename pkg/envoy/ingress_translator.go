package envoy

import (
	"fmt"
	"regexp"
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

type virtuhalhostroute struct {
	PathMatchCondition string
}
type virtualHost struct {
	Host            string
	UpstreamCluster string
	Timeout         time.Duration
	PerTryTimeout   time.Duration
}

type Route struct {

	// PathMatchCondition specifies a MatchCondition to match on the request path.
	// Must not be nil.
	PathMatchCondition MatchCondition
}
type MatchCondition interface {
	fmt.Stringer
}

// PrefixMatchType represents different types of prefix matching alternatives.
type PrefixMatchType int

const (
	// PrefixMatchString represents a prefix match that functions like a
	// string prefix match, i.e. prefix /foo matches /foobar
	PrefixMatchString PrefixMatchType = iota
	// PrefixMatchSegment represents a prefix match that only matches full path
	// segments, i.e. prefix /foo matches /foo/bar but not /foobar
	PrefixMatchSegment
)

var prefixMatchTypeToName = map[PrefixMatchType]string{
	PrefixMatchString:  "string",
	PrefixMatchSegment: "segment",
}

// PrefixMatchCondition matches the start of a URL.
type PrefixMatchCondition struct {
	Prefix          string
	PrefixMatchType PrefixMatchType
}

func (ec *ExactMatchCondition) String() string {
	return "exact: " + ec.Path
}

// ExactMatchCondition matches the entire path of a URL.
type ExactMatchCondition struct {
	Path string
}

// RegexMatchCondition matches the URL by regular expression.
type RegexMatchCondition struct {
	Regex string
}

func (v *virtualHost) Equals(other *virtualHost) bool {
	if other == nil {
		return false
	}

	return v.Host == other.Host &&
		v.Timeout == other.Timeout &&
		v.UpstreamCluster == other.UpstreamCluster &&
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

func translateIngresses(ingresses []v1.Ingress) *envoyConfiguration {
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

				for _, httppath := range httppaths(rule) {
					path := stringOrDefault(httppath.Path, "/")
					// Default to implementation specific path matching if not set.
					pathType := derefPathTypeOr(httppath.PathType, v1.PathTypeImplementationSpecific)
					fmt.Println(path, pathType)
				}

				if j.Hostname != "" {
					envoyIngress.addUpstream(j.Hostname)
				} else {
					envoyIngress.addUpstream(j.IP)
				}

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

func httppaths(rule v1.IngressRule) []v1.HTTPIngressPath {
	if rule.IngressRuleValue.HTTP == nil {
		// rule.IngressRuleValue.HTTP value is optional.
		return nil
	}
	return rule.IngressRuleValue.HTTP.Paths
}

func stringOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func derefPathTypeOr(ptr *v1.PathType, def v1.PathType) v1.PathType {
	if ptr != nil {
		return *ptr
	}
	return def
}

func Pathtranslate(path string, pathtype v1.PathType) (*Route, error) {
	r := &Route{
		PathMatchCondition: nil,
	}
	switch pathtype {
	case v1.PathTypePrefix:
		prefixMatchType := PrefixMatchSegment
		// An "all paths" prefix should be treated as a generic string prefix
		// match.
		if path == "/" {
			prefixMatchType = PrefixMatchString
		} else {
			// Strip trailing slashes. Ensures /foo matches prefix /foo/
			path = strings.TrimRight(path, "/")
		}
		r.PathMatchCondition = &PrefixMatchCondition{Prefix: path, PrefixMatchType: prefixMatchType}
	case v1.PathTypeExact:
		r.PathMatchCondition = &ExactMatchCondition{Path: path}
	case v1.PathTypeImplementationSpecific:
		// If a path "looks like" a regex we give a regex path match.
		// Otherwise you get a string prefix match.
		if strings.ContainsAny(path, "^+*[]%") {
			// validate the regex
			if err := ValidateRegex(path); err != nil {
				return nil, err
			}
			r.PathMatchCondition = &RegexMatchCondition{Regex: path}
		} else {
			r.PathMatchCondition = &PrefixMatchCondition{Prefix: path, PrefixMatchType: PrefixMatchString}
		}
	}
	return r, nil
}

// RouteMatch creates a *envoy_route_v3.RouteMatch for the supplied *dag.Route.
func RouteMatch(route Route) *envoy_route_v3.RouteMatch {
	switch c := route.PathMatchCondition.(type) {
	case *RegexMatchCondition:
		return &envoy_route_v3.RouteMatch{
			PathSpecifier: &envoy_route_v3.RouteMatch_SafeRegex{
				// Add an anchor since we at the very least have a / as a string literal prefix.
				// Reduces regex program size so Envoy doesn't reject long prefix matches.
				SafeRegex: SafeRegexMatch("^" + c.Regex),
			},
		}
	case *PrefixMatchCondition:
		switch c.PrefixMatchType {
		case PrefixMatchSegment:
			return &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_PathSeparatedPrefix{
					PathSeparatedPrefix: c.Prefix,
				},
			}
		case PrefixMatchString:
			fallthrough
		default:
			return &envoy_route_v3.RouteMatch{
				PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
					Prefix: c.Prefix,
				},
			}
		}
	case *ExactMatchCondition:
		return &envoy_route_v3.RouteMatch{
			PathSpecifier: &envoy_route_v3.RouteMatch_Path{
				Path: c.Path,
			},
		}
	default:
		return &envoy_route_v3.RouteMatch{}
	}
}

// ValidateRegex returns an error if the supplied
// RE2 regex syntax is invalid.
func ValidateRegex(regex string) error {
	_, err := regexp.Compile(regex)
	return err
}

func (rc *RegexMatchCondition) String() string {
	return "regex: " + rc.Regex
}

func (pc *PrefixMatchCondition) String() string {
	str := "prefix: " + pc.Prefix
	if typeStr, ok := prefixMatchTypeToName[pc.PrefixMatchType]; ok {
		str += " type: " + typeStr
	}
	return str
}
