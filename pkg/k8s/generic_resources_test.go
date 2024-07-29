package k8s

import (
	"testing"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func TestConvertExtensionsV1beta1Ingress(t *testing.T) {
	ev1b1 := &extensionsv1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "bar",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{Host: "foobar.io"},
				{Host: "barfoo.io"},
			},
			TLS: []extensionsv1beta1.IngressTLS{
				{
					Hosts:      []string{"foobar.io", "barfoo.io"},
					SecretName: "tls-boofar",
				},
			},
		},
		Status: extensionsv1beta1.IngressStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
					{IP: "5.6.7.8"},
				},
			},
		},
	}
	gen, err := convertToGenericIngress(ev1b1, false, "")
	if err != nil {
		t.Error(err)
	}

	if gen.Name != "foo" ||
		gen.Namespace != "bar" ||
		gen.Annotations["foo"] != "bar" ||
		gen.Class != nil ||
		!testEq(gen.RulesHosts, []string{"foobar.io", "barfoo.io"}) ||
		len(gen.TLS) != 2 ||
		gen.TLS["foobar.io"].Host != "foobar.io" ||
		gen.TLS["foobar.io"].SecretName != "tls-boofar" ||
		gen.TLS["barfoo.io"].Host != "barfoo.io" ||
		gen.TLS["barfoo.io"].SecretName != "tls-boofar" ||
		len(gen.Upstreams) != 2 ||
		gen.Upstreams[0] != "1.2.3.4" ||
		gen.Upstreams[1] != "5.6.7.8" {
		t.Error("extensions v1beta1 ingress conversion error")
	}
}

func TestConvertNetworkingV1beta1Ingress(t *testing.T) {
	nv1b1 := &networkingv1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "bar",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{Host: "foobar.io"},
				{Host: "barfoo.io"},
			},
			TLS: []networkingv1beta1.IngressTLS{
				{
					Hosts:      []string{"foobar.io", "barfoo.io"},
					SecretName: "tls-boofar",
				},
			},
		},
		Status: networkingv1beta1.IngressStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
					{IP: "5.6.7.8"},
				},
			},
		},
	}
	gen, err := convertToGenericIngress(nv1b1, false, "")
	if err != nil {
		t.Error(err)
	}

	if gen.Name != "foo" ||
		gen.Namespace != "bar" ||
		gen.Annotations["foo"] != "bar" ||
		gen.Class != nil ||
		!testEq(gen.RulesHosts, []string{"foobar.io", "barfoo.io"}) ||
		len(gen.TLS) != 2 ||
		gen.TLS["foobar.io"].Host != "foobar.io" ||
		gen.TLS["foobar.io"].SecretName != "tls-boofar" ||
		gen.TLS["barfoo.io"].Host != "barfoo.io" ||
		gen.TLS["barfoo.io"].SecretName != "tls-boofar" ||
		len(gen.Upstreams) != 2 ||
		gen.Upstreams[0] != "1.2.3.4" ||
		gen.Upstreams[1] != "5.6.7.8" {
		t.Error("networking.k8s.io v1beta1 ingress conversion error")
	}
}

func TestConvertNetworkingV1Ingress(t *testing.T) {
	className := "cl4ss"
	nv1 := &networkingv1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "bar",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{
				{Host: "foobar.io"},
				{Host: "barfoo.io"},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"foobar.io", "barfoo.io"},
					SecretName: "tls-boofar",
				},
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
					{IP: "5.6.7.8"},
				},
			},
		},
	}
	gen, err := convertToGenericIngress(nv1, false, "")
	if err != nil {
		t.Error(err)
	}

	if gen.Name != "foo" ||
		gen.Namespace != "bar" ||
		gen.Class == nil ||
		*gen.Class != "cl4ss" ||
		gen.Annotations["foo"] != "bar" ||
		!testEq(gen.RulesHosts, []string{"foobar.io", "barfoo.io"}) ||
		len(gen.TLS) != 2 ||
		gen.TLS["foobar.io"].Host != "foobar.io" ||
		gen.TLS["foobar.io"].SecretName != "tls-boofar" ||
		gen.TLS["barfoo.io"].Host != "barfoo.io" ||
		gen.TLS["barfoo.io"].SecretName != "tls-boofar" ||
		len(gen.Upstreams) != 2 ||
		gen.Upstreams[0] != "1.2.3.4" ||
		gen.Upstreams[1] != "5.6.7.8" {
		t.Error("networking.k8s.io v1beta1 ingress conversion error")
	}
}

func TestCompareConvertedV1V1beta1Ingresses(t *testing.T) {
	ev1b1 := &extensionsv1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "bar",
			Annotations: map[string]string{"kubernetes.io/ingress.class": "cl4ss"},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{Host: "foobar.io"},
				{Host: "barfoo.io"},
			},
			TLS: []extensionsv1beta1.IngressTLS{
				{
					Hosts:      []string{"foobar.io", "barfoo.io"},
					SecretName: "tls-boofar",
				},
			},
		},
		Status: extensionsv1beta1.IngressStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
					{IP: "5.6.7.8"},
				},
			},
		},
	}
	genv1b1, err := convertToGenericIngress(ev1b1, false, "")
	if err != nil {
		t.Error(err)
	}

	className := "cl4ss"
	ev1 := &networkingv1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "bar",
			Annotations: map[string]string{"kubernetes.io/ingress.class": "cl4ss"},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{
				{Host: "foobar.io"},
				{Host: "barfoo.io"},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"foobar.io", "barfoo.io"},
					SecretName: "tls-boofar",
				},
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
					{IP: "5.6.7.8"},
				},
			},
		},
	}
	genv1, err := convertToGenericIngress(ev1, false, "")
	if err != nil {
		t.Error(err)
	}

	if !GenericIngressEqual(genv1b1, genv1) {
		t.Error("ingress from v1beta1 not equal to one in v1, expected equality")
	}
}

func testEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
