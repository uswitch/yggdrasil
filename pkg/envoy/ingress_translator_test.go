package envoy

import (
	"testing"
	"time"

	"github.com/uswitch/yggdrasil/pkg/k8s"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// dummy p-256 cert
	p256crt = `-----BEGIN CERTIFICATE-----
MIIB3zCCAYWgAwIBAgIUN7vSLskm00u2GGIylQduwZXGjsowCgYIKoZIzj0EAwIw
RTELMAkGA1UEBhMCRlIxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGElu
dGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjA4MDIxNDUyMTFaFw0zMjA3MzAx
NDUyMTFaMEUxCzAJBgNVBAYTAkZSMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYD
VQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAAQhZBml0G0ClRxU+pD9hhk/3riNuZhMjI9Cn96+ITP928PasfpzwROG
uz5ouJHTQVSBsQkT0yQSIkgyFqenDfOJo1MwUTAdBgNVHQ4EFgQUhsgmD7TGpi4u
0PAjVuCUcuK7LGAwHwYDVR0jBBgwFoAUhsgmD7TGpi4u0PAjVuCUcuK7LGAwDwYD
VR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEApbVkyGjhTIpW12SO/9ZC
/fNrH9EJP6WYLU01PHklqMACIAgJjlEmdgCgWyw9kkFwdcwEHNl1rZiPdogCfOI/
aQu5
-----END CERTIFICATE-----`
	p256key = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg1B7hGnz2sT7YYiEY
fONn7xeyqX0gAI7mfJxxxMAwozChRANCAAQhZBml0G0ClRxU+pD9hhk/3riNuZhM
jI9Cn96+ITP928PasfpzwROGuz5ouJHTQVSBsQkT0yQSIkgyFqenDfOJ
-----END PRIVATE KEY-----`

	// dummy p-384 cert
	p384crt = `-----BEGIN CERTIFICATE-----
MIICGzCCAaKgAwIBAgIUfhCbmq9lQxfNE9g8sTdr/0quNW8wCgYIKoZIzj0EAwIw
RTELMAkGA1UEBhMCRlIxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGElu
dGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjA4MDIxNDQzMDhaFw0zMjA3MzAx
NDQzMDhaMEUxCzAJBgNVBAYTAkZSMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYD
VQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwdjAQBgcqhkjOPQIBBgUrgQQA
IgNiAAQWWReyjJbJHMDnomVYrn/CmceQFWDWQ/dLG3OgiffsjhxOS0IaaDKgUxJH
7/eW5AesWmhg1z9x0JSjab6mTneQMtHukPZEaLmwPlksEA1k2A/wph9mEjyZpgS4
IogLORCjUzBRMB0GA1UdDgQWBBTSCNXG1Q5+kIUZwGTHv1RsxDxhtzAfBgNVHSME
GDAWgBTSCNXG1Q5+kIUZwGTHv1RsxDxhtzAPBgNVHRMBAf8EBTADAQH/MAoGCCqG
SM49BAMCA2cAMGQCMDpl5L5TerZTuWb5K2fhDIjEs7YNMG7DxZPsZkZoj94Pzx3z
5CbmMKVQnn9aiIufdQIwCK9mXcQSu6vVYK8dI4BZIjGG6Osa/f638+r8SzIT/DZM
Y2jxayrpJmeeNJVB3QQd
-----END CERTIFICATE-----`
	p384key = `-----BEGIN PRIVATE KEY-----
MIG2AgEAMBAGByqGSM49AgEGBSuBBAAiBIGeMIGbAgEBBDDg36b+cJYLMeuJr6Y3
wheQ7S71MEMHQDzY7GrwPwkr9/4aJprY4NGQeLp2ZSvqSp6hZANiAAQWWReyjJbJ
HMDnomVYrn/CmceQFWDWQ/dLG3OgiffsjhxOS0IaaDKgUxJH7/eW5AesWmhg1z9x
0JSjab6mTneQMtHukPZEaLmwPlksEA1k2A/wph9mEjyZpgS4IogLORA=
-----END PRIVATE KEY-----`

	// dummy rsa2048 cert
	rsa2048crt = `-----BEGIN CERTIFICATE-----
MIIDETCCAfkCFArEpbFYH4WmMV2id+QeAriE3c+CMA0GCSqGSIb3DQEBCwUAMEUx
CzAJBgNVBAYTAkZSMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRl
cm5ldCBXaWRnaXRzIFB0eSBMdGQwHhcNMjIwODAyMTQ1NDQxWhcNMzIwNzMwMTQ1
NDQxWjBFMQswCQYDVQQGEwJGUjETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UE
CgwYSW50ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAyjA0rkVDC/sqPHD8uhiR7R009F6vkux+4IeeRY+z+nHQPceO
65LZOVGK8wAbeqq3/wLW5S3DKdEJwtyCW7gh2eGe5JllONKLLMAeHbfPEwlnKcJM
Ks/QDGtTwLSUJEIZEpBHJpPEX+ps1PtH1bdrLQHAnYZb6j4R2WUiC1ZaT30QWUF9
Rf/zpGWaf5Gr8Hwct2Z57EOGk0FKFXEexT0/zYq+z4rFBWm9cpWLCgGUyPU16dnx
O++GI86Pu3CKEXl/yfCxg95iK18SqV9HNMuGCzjnP2i1JTX91bgmwyIrkirBDb9u
wFyrlXXX+x/Dhg1vZL2HsmomfrcMhGc61ti5WQIDAQABMA0GCSqGSIb3DQEBCwUA
A4IBAQCKWSHYAefrBQNt+8r/MZ4SVJyHi8d7IEdCQEZ8c7Raz58KILewhq7ryMW6
PuUWweNkWUi4cg1lsAdtn7L+s1lCYaPx+4+x/WhdvhV2EK1B++dpMoIjIgguLSwE
gkGliRHp8s5J6SMS0iIUl5bZHWffzPywPj22FL04tiDxLqH6MxGqUtpazNUobllR
OWEc00pZQE9+LFjzq1X0GLGMnZGv5FLHTplgLw6nTmGFdpnQsIIN9jV+QZqKnltb
68sC8WktuoKamwpBm6jyxU252VJo6KHU1zuqK3Rr3ZT31j6ezCan6FUbcz+zJ/1x
wfzidY7YDRv5Hj/58DghbY46B4Md
-----END CERTIFICATE-----`
	rsa2048key = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAyjA0rkVDC/sqPHD8uhiR7R009F6vkux+4IeeRY+z+nHQPceO
65LZOVGK8wAbeqq3/wLW5S3DKdEJwtyCW7gh2eGe5JllONKLLMAeHbfPEwlnKcJM
Ks/QDGtTwLSUJEIZEpBHJpPEX+ps1PtH1bdrLQHAnYZb6j4R2WUiC1ZaT30QWUF9
Rf/zpGWaf5Gr8Hwct2Z57EOGk0FKFXEexT0/zYq+z4rFBWm9cpWLCgGUyPU16dnx
O++GI86Pu3CKEXl/yfCxg95iK18SqV9HNMuGCzjnP2i1JTX91bgmwyIrkirBDb9u
wFyrlXXX+x/Dhg1vZL2HsmomfrcMhGc61ti5WQIDAQABAoIBAB9r+HPw7aeKBBla
kdw1/0+zT0x+/pg9ysYILY+v8o+EapF/VvXDX6DpzEdRh/O7XlbyUQVS5Fa2VucC
r/ujFRewWao7MKDxD5IH1WZW74aM2oyB1qZ8n1+eumhjZ/Zuk0jwRS54nnctjnZX
CuXq2GwpLo8Ml3JC7TE052KNrAFYLlyOQGS8Vc5bVgHla6d0rWvUVilMBJhIiiMF
k516wOfzv+vQ/g0yd3F5d+2BX8OQ2Mc02Zm4M3oSqkA0tibqnW8N9bpobY3xCmpt
e/HeVaczCYS/qy6pauURsC/ZcWaPaFbN6q0H48m4EN/al51t2ITp9+uH8vdp0toH
DUKeRZECgYEA+BAmSTTQblEBPkzkEMKJeMQG8k/1Nm5dRqEKo6MsvMMfDdaQgArj
s2oEMJhroRGKSJJJl37ELc99vO9lbns/CjIF5quhM0FOIZ4hbZHmE3Gntif3bwKr
ZH5/3KUjjYekhWf1rac6Jldc96Qok1NF2DORCHVdjSjbaYt5u8naBsUCgYEA0KhN
X8b4AlRxsps0MLXx6iPW69VB37uYxhx1zB3bkWADldZxvUuLuo7dx1q0j0dM22Tx
7fIbzoHbVZPkiqryCV7TXzuEsXs7Il9FOwAdb3HPtaixZSSprU+QbnlYMTTbRGoW
BuY/VGpDVu29HVi40ADTswEHtdRRQwK7mJihsYUCgYEAiN9YULJcg1Ic7oQ8nubI
NaNr9c2ajqMMwojfNvU1HR5Ihzkp2AyqIPfRQgMH+AKWm35kLrwagPBo/5PUEsbc
PMLnMKTouEBDXRdEeJG1MmtWz5Jz24PMkBGgeV2BJXl/lMCM9Xk9A6TWvimM3eVn
t2iUkOc5bEbr8eusiqpQq8UCgYAGwYxPz5d0K9vKlq/n69w7YpGw7STG48IcmAtQ
Yp8bY+p5zYm9QVby4XFY5Rz3jq10ZR5YQACafSXm7XT28HYJy6I4cSrovD677C3H
rUdGtn6tORsVVUyRKgRZL2Clpzp6Sp0P+CCQ0SgBBo2bB6ZkRcKVBzGCt68x5kLA
vCBwKQKBgQDKunnMo3dxxsGuVahWJZ53OKaJ0xdrRWzzkeApKoPcOCA23HY4Es/S
Ke1NRtClxRYbm7lp75NUv3DVAlAg6YFaMs/tmzK6kHiEX9qDpbtTQ8dPRnR0baWQ
0XJc9Xisy367mUdL9n9ny1FRf05C/tA1XYOhBLTYsCAPbq1sD1kd5g==
-----END RSA PRIVATE KEY-----`
)

func TestVirtualHostEquality(t *testing.T) {
	a := &virtualHost{Host: "foo"}
	b := &virtualHost{Host: "foo"}

	if a.Equals(nil) {
		t.Error("virtual host is equal nix, expected not to be equal")
	}

	if !a.Equals(b) {
		t.Error()
	}

	c := &virtualHost{Host: ""}
	if a.Equals(c) {
		t.Error()
	}

	d := &virtualHost{Host: "foo", Timeout: (5 * time.Second)}
	if a.Equals(d) {
		t.Error("virtual hosts with different timeout values should not be equal")
	}

	e := &virtualHost{Host: "foo", PerTryTimeout: (5 * time.Second)}
	if a.Equals(e) {
		t.Error("virtual hosts with different per try timeout values should not be equal")
	}
}

func TestClusterEquality(t *testing.T) {
	a := &cluster{Name: "foo", Hosts: []LBHost{{"host1", 1}, {"host2", 1}}}
	b := &cluster{Name: "foo", Hosts: []LBHost{{"host1", 1}, {"host2", 1}}}

	if !a.Equals(b) {
		t.Error()
	}

	if a.Equals(nil) {
		t.Error("cluster is equals nil, expect not to be equal")
	}

	c := &cluster{Name: "bar", Hosts: []LBHost{{"host1", 1}, {"host2", 1}}}
	if a.Equals(c) {
		t.Error("clusters have different names, expected not to be equal")
	}

	d := &cluster{Name: "foo", Hosts: []LBHost{{"host1", 1}}} // missing host2
	if a.Equals(d) {
		t.Error("clusters have different hosts, should be different")
	}

	e := &cluster{Name: "foo", Hosts: []LBHost{{"bad1", 1}, {"bad2", 1}}}
	if a.Equals(e) {
		t.Error("cluster hosts are different, shouldn't be equal")
	}

	f := &cluster{Name: "foo"}
	if a.Equals(f) {
		t.Error("no hosts set")
	}

	g := &cluster{Name: "foo", Hosts: []LBHost{{"host1", 1}, {"host2", 1}}, Timeout: (5 * time.Second)}
	if a.Equals(g) {
		t.Error("clusters with different timeout values should not be equal")
	}

	h := &cluster{Name: "foo", VirtualHost: "bar"}
	if a.Equals(h) {
		t.Error("cluster virtualHosts are different, shouldn't be equal")
	}

	i := &cluster{Name: "foo", HealthCheckPath: "bar"}
	if a.Equals(i) {
		t.Error("cluster virtualHosts are different, shouldn't be equal")
	}
}

func TestEqualityClusters(t *testing.T) {
	c1 := []*cluster{{Name: "foo"}, {Name: "bar"}}
	c2 := []*cluster{{Name: "foo"}, {Name: "bar"}}

	if !ClustersEquals(c1, c2) {
		t.Error("expected equal clusters")
	}

	c3 := []*cluster{{Name: "foo"}, {Name: "baz"}}
	if ClustersEquals(c1, c3) {
		t.Error("clusters have different names, shouldn't be equal")
	}
}

func TestEqualityVirtualHosts(t *testing.T) {
	a := []*virtualHost{{Host: "foo.com"}, {Host: "bar.com"}}
	b := []*virtualHost{{Host: "foo.com"}, {Host: "baz.com"}}

	if VirtualHostsEquals(a, b) {
		t.Error("have different hosts, shouldn't be equal")
	}
}

func TestEquals(t *testing.T) {
	ingress := newGenericIngress("foo.app.com", "foo.cluster.com")
	ingress2 := newGenericIngress("bar.app.com", "foo.bar.com")
	c := translateIngresses([]*k8s.Ingress{ingress, ingress2}, false, []*v1.Secret{})
	c2 := translateIngresses([]*k8s.Ingress{ingress, ingress2}, false, []*v1.Secret{})

	vmatch, cmatch := c.equals(c2)
	if vmatch != true {
		t.Error("virtual hosts did not match")
	}
	if cmatch != true {
		t.Error("clusters did not match")
	}
}

func TestNotEquals(t *testing.T) {
	ingress := newGenericIngress("foo.bar.com", "bar.cluster.com")
	ingress2 := newGenericIngress("foo.app.com", "bar.cluster.com")
	ingress3 := newGenericIngress("foo.baz.com", "bar.cluster.com")
	ingress4 := newGenericIngress("foo.howdy.com", "bar.cluster.com")
	c := translateIngresses([]*k8s.Ingress{ingress, ingress3, ingress2}, false, []*v1.Secret{})
	c2 := translateIngresses([]*k8s.Ingress{ingress, ingress2, ingress4}, false, []*v1.Secret{})

	vmatch, cmatch := c.equals(c2)
	if vmatch == true {
		t.Error("virtual hosts matched")
	}
	if cmatch == true {
		t.Error("clusters matched")
	}

}

func TestPartialEquals(t *testing.T) {
	ingress := newGenericIngress("foo.app.com", "bar.cluster.com")
	ingress2 := newGenericIngress("foo.app.com", "foo.cluster.com")
	c := translateIngresses([]*k8s.Ingress{ingress2}, false, []*v1.Secret{})
	c2 := translateIngresses([]*k8s.Ingress{ingress}, false, []*v1.Secret{})

	vmatch, cmatch := c2.equals(c)
	if vmatch != true {
		t.Error("virtual hosts did not match")
	}
	if cmatch == true {
		t.Error("clusters matched")
	}

}

func TestGeneratesForSingleIngress(t *testing.T) {
	ingress := newGenericIngress("foo.app.com", "foo.cluster.com")
	c := translateIngresses([]*k8s.Ingress{ingress}, false, []*v1.Secret{})

	if len(c.VirtualHosts) != 1 {
		t.Error("expected 1 virtual host")
	}
	if c.VirtualHosts[0].Host != "foo.app.com" {
		t.Errorf("expected virtual host for foo.app.com, was %s", c.VirtualHosts[0].Host)
	}

	if len(c.Clusters) != 1 {
		t.Error("expected 1 clusters")
	}

	if c.Clusters[0].Name != "foo_app_com" {
		t.Errorf("expected cluster to be named after ingress host, was %s", c.Clusters[0].Name)
	}
	if c.Clusters[0].Hosts[0].Host != "foo.cluster.com" {
		t.Errorf("expected cluster host for foo.cluster.com, was %s", c.Clusters[0].Hosts[0].Host)
	}

	if c.Clusters[0].Hosts[0].Weight != 1 {
		t.Errorf("expected cluster host's weight for 1, was %s", c.Clusters[0].Hosts[0].Weight)
	}

	if c.VirtualHosts[0].UpstreamCluster != c.Clusters[0].Name {
		t.Errorf("expected upstream cluster of vHost the same as the generated cluster, was %s and %s", c.VirtualHosts[0].UpstreamCluster, c.Clusters[0].Name)
	}

	if c.Clusters[0].VirtualHost != "foo.app.com" {
		t.Errorf("expected upstream cluster vHost the same as the ingress vHost")
	}
}

func TestGeneratesForMultipleIngressSharingSpecHost(t *testing.T) {
	fooIngress := newGenericIngress("app.com", "foo.com")
	barIngress := newGenericIngress("app.com", "bar.com")
	c := translateIngresses([]*k8s.Ingress{fooIngress, barIngress}, false, []*v1.Secret{})

	if len(c.VirtualHosts) != 1 {
		t.Error("expected 1 virtual host")
	}
	if c.VirtualHosts[0].Host != "app.com" {
		t.Errorf("expected virtual host for app.com, was %s", c.VirtualHosts[0].Host)
	}

	if len(c.Clusters) != 1 {
		t.Errorf("expected 1 clusters, was %d", len(c.Clusters))
	}

	if c.Clusters[0].Name != "app_com" {
		t.Errorf("expected cluster to be named after ingress host, was %s", c.Clusters[0].Name)
	}

	if len(c.Clusters[0].Hosts) != 2 {
		t.Errorf("expected 2 host, was %d", len(c.Clusters[0].Hosts))
	}
	if c.Clusters[0].Hosts[0].Host != "foo.com" {
		t.Errorf("expected cluster host for foo.com, was %s", c.Clusters[0].Hosts[0])
	}
	if c.Clusters[0].Hosts[1].Host != "bar.com" {
		t.Errorf("expected cluster host for bar.com, was %s", c.Clusters[0].Hosts[1])
	}

	if c.VirtualHosts[0].UpstreamCluster != c.Clusters[0].Name {
		t.Errorf("expected upstream cluster of vHost the same as the generated cluster, was %s and %s", c.VirtualHosts[0].UpstreamCluster, c.Clusters[0].Name)
	}
}

func TestFilterMatchingIngresses(t *testing.T) {
	ingress := []*k8s.Ingress{
		newGenericIngress("host", "balancer"),
	}
	ingressClasses := []string{"bar"}
	matchingIngresses := classFilter(ingress, ingressClasses)
	if len(matchingIngresses) != 1 {
		t.Errorf("expected one ingress to match class bar, got %d ingresses", len(matchingIngresses))
	}
}
func TestFilterNonMatchingIngresses(t *testing.T) {
	ingress := []*k8s.Ingress{
		newGenericIngress("host", "balancer"),
	}
	ingressClasses := []string{"another-class"}
	matchingIngresses := classFilter(ingress, ingressClasses)
	if len(matchingIngresses) != 0 {
		t.Errorf("expected no ingress to match class another-class, got %d ingresses", len(matchingIngresses))
	}
}

func TestIngressWithIP(t *testing.T) {
	ingress := newIngressIP("app.com", "127.0.0.1")
	c := translateIngresses([]*k8s.Ingress{ingress}, false, []*v1.Secret{})
	if c.Clusters[0].Hosts[0].Host != "127.0.0.1" {
		t.Errorf("expected cluster host to be IP address, was %s", c.Clusters[0].Hosts[0])
	}
}

func TestIngressFilterWithValidConfigWithHostname(t *testing.T) {
	ingresses := []*k8s.Ingress{
		newGenericIngress("app.com", "foo.com"),
	}
	matchingIngresses := validIngressFilter(ingresses)
	if len(matchingIngresses) != 1 {
		t.Errorf("expected one ingress to be valid, got %d ingresses", len(matchingIngresses))
	}
}

func TestIngressFilterWithValidConfigWithIP(t *testing.T) {
	ingresses := []*k8s.Ingress{
		newGenericIngress("app.com", "127.0.0.1"),
	}
	matchingIngresses := validIngressFilter(ingresses)
	if len(matchingIngresses) != 1 {
		t.Errorf("expected one ingress to be valid, got %d ingresses", len(matchingIngresses))
	}
}

func TestIngressFilterWithNoHost(t *testing.T) {
	ingresses := []*k8s.Ingress{
		newGenericIngress("", "foo.com"),
	}
	matchingIngresses := validIngressFilter(ingresses)
	if len(matchingIngresses) != 0 {
		t.Errorf("expected no ingress to be valid without a hostname or ip, got %d ingresses", len(matchingIngresses))
	}
}

func TestIngressFilterWithNoLoadBalancerHostName(t *testing.T) {
	ingresses := []*k8s.Ingress{
		newGenericIngress("app.com", ""),
	}
	matchingIngresses := validIngressFilter(ingresses)
	if len(matchingIngresses) != 0 {
		t.Errorf("expected no ingress to be valid without a hostname, got %d ingresses", len(matchingIngresses))
	}
}

func TestHostMatch(t *testing.T) {
	matching := [][]string{
		{"*.a.b", "*.a.b"},
		{"a.a.b", "a.a.b"},
		{"a.a.b", "*.a.b"},
	}
	nonMatching := [][]string{
		{"*.a.b", "a.b"},
		{"*.a.b", "a.a.b"},
		{"*.a.b", "a.a.a.b"},
		{"a.a.b", "*.a.a.b"},
		{"a.b", ""},
		{"", "a.b"},
	}
	for _, m := range matching {
		if !hostMatch(m[0], m[1]) {
			t.Errorf("expected %s to match with %s", m[0], m[1])
		}

	}
	for _, m := range nonMatching {
		if hostMatch(m[0], m[1]) {
			t.Errorf("expected %s not to match with %s", m[0], m[1])
		}

	}
}

func TestGetHostTlsSecret(t *testing.T) {
	secrets := []*v1.Secret{
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns3", Name: "foo"}},
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns3", Name: "bar"}},
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "foo"}},
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "bar"}},
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "bar"}},
		&v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "foo"}},
	}
	ing := &k8s.Ingress{
		Namespace:  "ns1",
		Name:       "ing",
		RulesHosts: []string{"foo", "boo", "bar"},
		TLS: map[string]*k8s.IngressTLS{
			"nah":  {Host: "nah", SecretName: "nah"},
			"bar":  {Host: "bar", SecretName: "bar"},
			"nope": {Host: "nope", SecretName: "nope"},
			"foo":  {Host: "foo", SecretName: "foo"},
		},
	}

	if sec, err := getHostTlsSecret(ing, "foo", secrets); err != nil {
		t.Errorf("expected secret, caught error: %s", err.Error())
	} else if sec.Namespace != ing.Namespace || sec.Name != "foo" {
		t.Errorf("expected secret ns1/foo but got %s/%s", sec.Namespace, sec.Name)
	}

	if sec, err := getHostTlsSecret(ing, "bar", secrets); err != nil {
		t.Errorf("expected secret, caught error: %s", err.Error())
	} else if sec.Namespace != ing.Namespace || sec.Name != "bar" {
		t.Errorf("expected secret ns1/bar but got %s/%s", sec.Namespace, sec.Name)
	}

	if sec, _ := getHostTlsSecret(ing, "nope", secrets); sec != nil {
		t.Errorf("expected error for missing secret, got secret %s/%s", sec.Namespace, sec.Name)
	}
}

func TestValidateEmptyTlsSecret(t *testing.T) {
	sec := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte(""),
		"tls.key": []byte(""),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if v {
		t.Errorf("expected empty secret to be invalid")
	}
}

func TestValidateIncompleteTlsSecret(t *testing.T) {
	var sec *v1.Secret
	sec = &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte("blep"),
		"tls.key": []byte(""),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if v {
		t.Errorf("expected empty secret to be invalid")
	}

	sec = &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte(""),
		"tls.key": []byte("blep"),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if v {
		t.Errorf("expected empty secret to be invalid")
	}
}

func TestValidateWrongPEMTlsSecret(t *testing.T) {
	sec := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte("blap"),
		"tls.key": []byte("blep"),
	}}
	if v, err := validateTlsSecret(sec); err == nil || v {
		t.Errorf("expected PEM error, got none")
	}
}

func TestValidateP384TlsSecret(t *testing.T) {
	sec := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte(p384crt),
		"tls.key": []byte(p384key),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if v {
		t.Errorf("expected ECDSA >256 cert to be invalid")
	}
}

func TestValidateP256TlsSecret(t *testing.T) {
	sec := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte(p256crt),
		"tls.key": []byte(p256key),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if !v {
		t.Errorf("expected ECDSA P-256 secret to be valid")
	}
}

func TestValidateRsa2048TlsSecret(t *testing.T) {
	sec := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec"}, Data: map[string][]byte{
		"tls.crt": []byte(rsa2048crt),
		"tls.key": []byte(rsa2048key),
	}}
	if v, err := validateTlsSecret(sec); err != nil {
		t.Errorf("expected no error, caught: %s", err.Error())
	} else if !v {
		t.Errorf("expected RSA 2048 secret to be valid")
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
				{
					Host: specHost,
				},
			},
		},
		Status: v1beta1.IngressStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{Hostname: loadbalancerHost},
				},
			},
		},
	}
}

func newGenericIngress(specHost string, loadbalancerHost string) *k8s.Ingress {
	return &k8s.Ingress{
		Annotations: map[string]string{
			"kubernetes.io/ingress.class": "bar",
		},
		RulesHosts: []string{specHost},
		Upstreams:  []string{loadbalancerHost},
	}
}

func newIngressIP(specHost string, loadbalancerHost string) *k8s.Ingress {
	return &k8s.Ingress{
		Annotations: map[string]string{
			"kubernetes.io/ingress.class": "bar",
		},
		RulesHosts: []string{specHost},
		Upstreams:  []string{loadbalancerHost},
	}
}
