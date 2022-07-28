package envoy

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	v1 "k8s.io/api/core/v1"
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
	TlsKey          string
	TlsCert         string
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
		v.TlsCert == other.TlsCert
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
		logrus.Debugf("skipping certificate %s/%s: missing 'tls.crt' or 'tls.key'", secret.Namespace, secret.Name)
		return false, nil
	}
	if len(tlsCert) == 0 || len(tlsKey) == 0 {
		logrus.Debugf("skipping certificate %s/%s: empty 'tls.crt' or 'tls.key'", secret.Namespace, secret.Name)
		return false, nil
	}

	// discard P-384 EC private keys
	// see https://github.com/envoyproxy/envoy/issues/10855
	block, _ := pem.Decode(tlsCert)
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

func translateIngresses(ingresses []*k8s.Ingress, syncSecrets bool, secrets []*v1.Secret) *envoyConfiguration {
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

				if syncSecrets && envoyIngress.vhost.TlsKey == "" && envoyIngress.vhost.TlsCert == "" {
					if hostTlsSecret, err := getHostTlsSecret(i, ruleHost, secrets); err != nil {
						logrus.Debug(err.Error())
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
	}

	for _, ingress := range envoyIngresses {
		cfg.Clusters = append(cfg.Clusters, ingress.cluster)
		cfg.VirtualHosts = append(cfg.VirtualHosts, ingress.vhost)
	}

	numVhosts.Set(float64(len(cfg.VirtualHosts)))
	numClusters.Set(float64(len(cfg.Clusters)))

	return cfg
}
