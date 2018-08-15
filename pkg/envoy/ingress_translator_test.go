package envoy

import (
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVirtualHostEquality(t *testing.T) {
	a := &virtualHost{Host: "foo"}
	b := &virtualHost{Host: "foo"}

	if !a.Equals(b) {
		t.Error()
	}

	c := &virtualHost{Host: ""}
	if a.Equals(c) {
		t.Error()
	}
}

func TestClusterEquality(t *testing.T) {
	a := &cluster{Name: "foo", Hosts: []string{"host1", "host2"}}
	b := &cluster{Name: "foo", Hosts: []string{"host1", "host2"}}

	if !a.Equals(b) {
		t.Error()
	}

	c := &cluster{Name: "bar", Hosts: []string{"host1", "host2"}}
	if a.Equals(c) {
		t.Error("clusters have different names, expected not to be equal")
	}

	d := &cluster{Name: "foo", Hosts: []string{"host1"}} // missing host2
	if a.Equals(d) {
		t.Error("clusters have different hosts, should be different")
	}

	e := &cluster{Name: "foo", Hosts: []string{"bad1", "bad2"}}
	if a.Equals(e) {
		t.Error("cluster hosts are different, shouldn't be equal")
	}

	f := &cluster{Name: "foo"}
	if a.Equals(f) {
		t.Error("no hosts set")
	}
}

func TestEqualityClusters(t *testing.T) {
	c1 := []*cluster{&cluster{Name: "foo"}, &cluster{Name: "bar"}}
	c2 := []*cluster{&cluster{Name: "foo"}, &cluster{Name: "bar"}}

	if !ClustersEquals(c1, c2) {
		t.Error("expected equal clusters")
	}

	c3 := []*cluster{&cluster{Name: "foo"}, &cluster{Name: "baz"}}
	if ClustersEquals(c1, c3) {
		t.Error("clusters have different names, shouldn't be equal")
	}
}

func TestEqualityVirtualHosts(t *testing.T) {
	a := []*virtualHost{&virtualHost{Host: "foo.com"}, &virtualHost{Host: "bar.com"}}
	b := []*virtualHost{&virtualHost{Host: "foo.com"}, &virtualHost{Host: "baz.com"}}

	if VirtualHostsEquals(a, b) {
		t.Error("have different hosts, shouldn't be equal")
	}
}

func TestEquals(t *testing.T) {
	ingress := newIngress("foo.app.com", "foo.cluster.com")
	ingress2 := newIngress("bar.app.com", "foo.bar.com")
	c := translateIngresses([]v1beta1.Ingress{ingress, ingress2})
	c2 := translateIngresses([]v1beta1.Ingress{ingress, ingress2})

	vmatch, cmatch := c.equals(c2)
	if vmatch != true {
		t.Error("virtual hosts did not match")
	}
	if cmatch != true {
		t.Error("clusters did not match")
	}
}

func TestNotEquals(t *testing.T) {
	ingress := newIngress("foo.bar.com", "bar.cluster.com")
	ingress2 := newIngress("foo.app.com", "bar.cluster.com")
	ingress3 := newIngress("foo.baz.com", "bar.cluster.com")
	ingress4 := newIngress("foo.howdy.com", "bar.cluster.com")
	c := translateIngresses([]v1beta1.Ingress{ingress, ingress3, ingress2})
	c2 := translateIngresses([]v1beta1.Ingress{ingress, ingress2, ingress4})

	vmatch, cmatch := c.equals(c2)
	if vmatch == true {
		t.Error("virtual hosts matched")
	}
	if cmatch == true {
		t.Error("clusters matched")
	}

}

func TestPartialEquals(t *testing.T) {
	ingress := newIngress("foo.app.com", "bar.cluster.com")
	ingress2 := newIngress("foo.app.com", "foo.cluster.com")
	c := translateIngresses([]v1beta1.Ingress{ingress2})
	c2 := translateIngresses([]v1beta1.Ingress{ingress})

	vmatch, cmatch := c2.equals(c)
	if vmatch != true {
		t.Error("virtual hosts did not match")
	}
	if cmatch == true {
		t.Error("clusters matched")
	}

}

func TestGeneratesForSingleIngress(t *testing.T) {
	ingress := newIngress("foo.app.com", "foo.cluster.com")
	c := translateIngresses([]v1beta1.Ingress{ingress})

	if len(c.VirtualHosts) != 1 {
		t.Error("expected 1 virtual host")
	}
	if c.VirtualHosts[0].Host != "foo.app.com" {
		t.Errorf("expected virtual host for foo.app.com, was %s", c.VirtualHosts[0].Host)
	}

	if len(c.Clusters) != 1 {
		t.Error("expected 1 clusters")
	}

	if c.Clusters[0].Name != "foo.app.com" {
		t.Errorf("expected cluster to be named after ingress host, was %s", c.Clusters[0].Name)
	}
	if c.Clusters[0].Hosts[0] != "foo.cluster.com" {
		t.Errorf("expected cluster host for foo.cluster.com, was %s", c.Clusters[0].Hosts[0])
	}
}

func TestGeneratesForMultipleIngressSharingSpecHost(t *testing.T) {
	fooIngress := newIngress("app.com", "foo.com")
	barIngress := newIngress("app.com", "bar.com")
	c := translateIngresses([]v1beta1.Ingress{fooIngress, barIngress})

	if len(c.VirtualHosts) != 1 {
		t.Error("expected 1 virtual host")
	}
	if c.VirtualHosts[0].Host != "app.com" {
		t.Errorf("expected virtual host for app.com, was %s", c.VirtualHosts[0].Host)
	}

	if len(c.Clusters) != 1 {
		t.Errorf("expected 1 clusters, was %d", len(c.Clusters))
	}

	if c.Clusters[0].Name != "app.com" {
		t.Errorf("expected cluster to be named after ingress host, was %s", c.Clusters[0].Name)
	}

	if len(c.Clusters[0].Hosts) != 2 {
		t.Errorf("expected 2 host, was %d", len(c.Clusters[0].Hosts))
	}
	if c.Clusters[0].Hosts[0] != "foo.com" {
		t.Errorf("expected cluster host for foo.com, was %s", c.Clusters[0].Hosts[0])
	}
	if c.Clusters[0].Hosts[1] != "bar.com" {
		t.Errorf("expected cluster host for bar.com, was %s", c.Clusters[0].Hosts[1])
	}
}

func TestFilterMatchingIngresses(t *testing.T) {
	ingress := []v1beta1.Ingress{
		newIngress("host", "balancer"),
	}
	ingressClasses := []string{"bar"}
	matchingIngresses := classFilter(ingress, ingressClasses)
	if len(matchingIngresses) != 1 {
		t.Errorf("expected one ingress to match class bar, got %d ingresses", len(matchingIngresses))
	}
}
func TestFilterNonMatchingIngresses(t *testing.T) {
	ingress := []v1beta1.Ingress{
		newIngress("host", "balancer"),
	}
	ingressClasses := []string{"another-class"}
	matchingIngresses := classFilter(ingress, ingressClasses)
	if len(matchingIngresses) != 0 {
		t.Errorf("expected no ingress to match class another-class, got %d ingresses", len(matchingIngresses))
	}
}

func newIngress(specHost string, loadbalancerHost string) v1beta1.Ingress {
	return v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "bar",
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: specHost,
				},
			},
		},
		Status: v1beta1.IngressStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					v1.LoadBalancerIngress{Hostname: loadbalancerHost},
				},
			},
		},
	}
}
