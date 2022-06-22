package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
)

// Ingress is the version-agnostic description of an ingress
type Ingress struct {
	Namespace   string
	Name        string
	Class       *string
	Annotations map[string]string
	RulesHosts  []string
	Upstreams   []string
	TLS         map[string]*IngressTLS
}

// IngressTLS describes the transport layer security associated with an Ingress.
type IngressTLS struct {
	Host       string
	SecretName string
}

// IngressRule is useful data from k8s structures about ingress rule
type IngressRule struct {
	Host string
}

// Get ingresses from stores and convert them to apiGroup-agnostic ingresses
func (a *Aggregator) GetGenericIngresses() ([]*Ingress, error) {
	ing := make([]*Ingress, 0)
	for _, store := range a.ingressStores {
		ingresses := store.List()
		for _, obj := range ingresses {
			genericIng, err := convertToGenericIngress(obj)
			if err != nil {
				return nil, err
			}
			ing = append(ing, genericIng)
		}
	}
	return ing, nil
}

// Convert k8s ingress to apiGroup-agnostic ingress
func convertToGenericIngress(ing interface{}) (ingress *Ingress, err error) {
	switch t := ing.(type) {
	case *extensionsv1beta1.Ingress:
		i, ok := ing.(*extensionsv1beta1.Ingress)
		if !ok {
			return nil, fmt.Errorf("unexpected object in store: %+v", ing)
		}
		ingress = convertExtensionsv1beta1Ingress(i)
	case *networkingv1beta1.Ingress:
		i, ok := ing.(*networkingv1beta1.Ingress)
		if !ok {
			return nil, fmt.Errorf("unexpected object in store: %+v", ing)
		}
		ingress = convertNetworkingv1beta1Ingress(i)
	case *networkingv1.Ingress:
		i, ok := ing.(*networkingv1.Ingress)
		if !ok {
			return nil, fmt.Errorf("unexpected object in store: %+v", ing)
		}
		ingress = convertNetworkingv1Ingress(i)
	default:
		err = fmt.Errorf("unrecognized type for: %T", t)
	}
	return
}

func convertExtensionsv1beta1Ingress(i *extensionsv1beta1.Ingress) *Ingress {
	return &Ingress{
		Namespace:   i.Namespace,
		Name:        i.Name,
		Class:       i.Spec.IngressClassName,
		Annotations: i.Annotations,
		RulesHosts: func(rules *[]extensionsv1beta1.IngressRule) (hosts []string) {
			for _, rule := range *rules {
				hosts = append(hosts, rule.Host)
			}
			return
		}(&i.Spec.Rules),
		Upstreams: func(i *[]v1.LoadBalancerIngress) (upstreams []string) {
			for _, j := range *i {
				if j.Hostname != "" {
					upstreams = append(upstreams, j.Hostname)
				} else {
					upstreams = append(upstreams, j.IP)
				}
			}
			return
		}(&i.Status.LoadBalancer.Ingress),
		TLS: func(itls []extensionsv1beta1.IngressTLS) (tls map[string]*IngressTLS) {
			tls = make(map[string]*IngressTLS)
			for _, t := range itls {
				for _, h := range t.Hosts {
					tls[h] = &IngressTLS{
						Host:       h,
						SecretName: t.SecretName,
					}
				}
			}
			return
		}(i.Spec.TLS),
	}
}

func convertNetworkingv1beta1Ingress(i *networkingv1beta1.Ingress) *Ingress {
	return &Ingress{
		Namespace:   i.Namespace,
		Name:        i.Name,
		Class:       i.Spec.IngressClassName,
		Annotations: i.Annotations,
		RulesHosts: func(rules *[]networkingv1beta1.IngressRule) (hosts []string) {
			for _, rule := range *rules {
				hosts = append(hosts, rule.Host)
			}
			return
		}(&i.Spec.Rules),
		Upstreams: func(i *[]v1.LoadBalancerIngress) (upstreams []string) {
			for _, j := range *i {
				if j.Hostname != "" {
					upstreams = append(upstreams, j.Hostname)
				} else {
					upstreams = append(upstreams, j.IP)
				}
			}
			return
		}(&i.Status.LoadBalancer.Ingress),
		TLS: func(itls []networkingv1beta1.IngressTLS) (tls map[string]*IngressTLS) {
			tls = make(map[string]*IngressTLS)
			for _, t := range itls {
				for _, h := range t.Hosts {
					tls[h] = &IngressTLS{
						Host:       h,
						SecretName: t.SecretName,
					}
				}
			}
			return
		}(i.Spec.TLS),
	}
}

func convertNetworkingv1Ingress(i *networkingv1.Ingress) *Ingress {
	return &Ingress{
		Namespace:   i.Namespace,
		Name:        i.Name,
		Class:       i.Spec.IngressClassName,
		Annotations: i.Annotations,
		RulesHosts: func(rules *[]networkingv1.IngressRule) (hosts []string) {
			for _, rule := range *rules {
				hosts = append(hosts, rule.Host)
			}
			return
		}(&i.Spec.Rules),
		Upstreams: func(i *[]v1.LoadBalancerIngress) (upstreams []string) {
			for _, j := range *i {
				if j.Hostname != "" {
					upstreams = append(upstreams, j.Hostname)
				} else {
					upstreams = append(upstreams, j.IP)
				}
			}
			return
		}(&i.Status.LoadBalancer.Ingress),
		TLS: func(itls []networkingv1.IngressTLS) (tls map[string]*IngressTLS) {
			tls = make(map[string]*IngressTLS)
			for _, t := range itls {
				for _, h := range t.Hosts {
					tls[h] = &IngressTLS{
						Host:       h,
						SecretName: t.SecretName,
					}
				}
			}
			return
		}(i.Spec.TLS),
	}
}
