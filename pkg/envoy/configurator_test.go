package envoy

import (
	"testing"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"

	"k8s.io/api/extensions/v1beta1"
)

func assertNumberOfVirtualHosts(t *testing.T, filterChain listener.FilterChain, expected int) {
	var connManager hcm.HttpConnectionManager

	err := util.StructToMessage(filterChain.Filters[0].Config, &connManager)
	if err != nil {
		t.Fatal(err)
	}

	routeSpecifier := connManager.RouteSpecifier.(*hcm.HttpConnectionManager_RouteConfig)
	virtualHosts := routeSpecifier.RouteConfig.VirtualHosts

	if len(virtualHosts) != expected {
		t.Fatalf("Num virtual hosts: %d expected %d", len(virtualHosts), expected)
	}

}

func assertTlsCertificate(t *testing.T, filterChain listener.FilterChain, expectedCert, expectedKey string) {
	certificate := filterChain.TlsContext.CommonTlsContext.TlsCertificates[0]

	certFile := certificate.CertificateChain.Specifier.(*core.DataSource_InlineString)
	keyFile := certificate.PrivateKey.Specifier.(*core.DataSource_InlineString)

	if certFile.InlineString != expectedCert {
		t.Fatalf("certficiate chain filename: '%s' expected '%s'", certFile.InlineString, expectedCert)
	}

	if keyFile.InlineString != expectedKey {
		t.Fatalf("private key filename: '%s' expected '%s'", keyFile.InlineString, expectedKey)
	}
}

func assertServerNames(t *testing.T, filterChain listener.FilterChain, expectedServerNames []string) {
	serverNames := filterChain.FilterChainMatch.ServerNames

	if len(serverNames) != len(expectedServerNames) {
		t.Fatalf("not the same number of server names: '%d' expected '%d'", len(serverNames), len(expectedServerNames))
	}

	for idx, expectedServerName := range expectedServerNames {
		if serverNames[idx] != expectedServerName {
			t.Errorf("server names do not match: '%v' expected '%v'", serverNames[idx], expectedServerName)
		}
	}
}

func TestGenerate(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("wibble", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", []Certificate{
		{Hosts: []string{"*"}, Cert: "b", Key: "c"},
	}, "d", 443, []string{"bar"})

	snapshot := configurator.Generate(ingresses)

	if len(snapshot.Listeners.Items) != 1 {
		t.Fatalf("Num listeners: %d", len(snapshot.Listeners.Items))
	}
	if len(snapshot.Clusters.Items) != 1 {
		t.Fatalf("Num clusters: %d", len(snapshot.Clusters.Items))
	}
}

func TestGenerateMultipleCerts(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("foo.internal.api.com", "bibble"),
		newIngress("foo.internal.api.co.uk", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", []Certificate{
		{Hosts: []string{"*.internal.api.com"}, Cert: "com", Key: "com"},
		{Hosts: []string{"*.internal.api.co.uk"}, Cert: "couk", Key: "couk"},
	}, "d", 443, []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Listeners.Items["listener_0"].(*v2.Listener)

	if len(listener.FilterChains) != 2 {
		t.Fatalf("Num filter chains: %d expected %d", len(listener.FilterChains), 2)
	}

	assertNumberOfVirtualHosts(t, listener.FilterChains[0], 1)
	assertNumberOfVirtualHosts(t, listener.FilterChains[1], 1)
}

func TestGenerateMultipleHosts(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("foo.internal.api.com", "bibble"),
		newIngress("foo.internal.api.co.uk", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", []Certificate{
		{Hosts: []string{"*.internal.api.com", "*.internal.api.co.uk"}, Cert: "com", Key: "com"},
	}, "d", 443, []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Listeners.Items["listener_0"].(*v2.Listener)

	if len(listener.FilterChains) != 1 {
		t.Fatalf("Num filter chains: %d expected %d", len(listener.FilterChains), 1)
	}

	// there should be two virtual hosts on the filter chain
	assertNumberOfVirtualHosts(t, listener.FilterChains[0], 2)
}

func TestGenerateNoMatchingCert(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("foo.internal.api.com", "bibble"),
		newIngress("foo.internal.api.co.uk", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", []Certificate{
		{Hosts: []string{"*.internal.api.com"}, Cert: "com", Key: "com"},
	}, "d", 443, []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Listeners.Items["listener_0"].(*v2.Listener)

	if len(listener.FilterChains) != 1 {
		t.Fatalf("Num filter chains: %d expected %d", len(listener.FilterChains), 1)
	}
}

func TestGenerateIntoTwoCerts(t *testing.T) {
	ingresses := []v1beta1.Ingress{
		newIngress("foo.internal.api.com", "bibble"),
	}

	configurator := NewKubernetesConfigurator("a", []Certificate{
		{Hosts: []string{"*.internal.api.com"}, Cert: "com", Key: "com"},
		{Hosts: []string{"*"}, Cert: "all", Key: "all"},
	}, "d", 443, []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Listeners.Items["listener_0"].(*v2.Listener)

	if len(listener.FilterChains) != 2 {
		t.Fatalf("Num filter chains: %d expected %d", len(listener.FilterChains), 2)
	}

	assertNumberOfVirtualHosts(t, listener.FilterChains[0], 1)
	assertServerNames(t, listener.FilterChains[0], []string{"*.internal.api.com"})

	assertNumberOfVirtualHosts(t, listener.FilterChains[1], 1)
	assertServerNames(t, listener.FilterChains[1], nil)
}
