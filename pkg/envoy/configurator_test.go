package envoy

import (
	"testing"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	util "github.com/envoyproxy/go-control-plane/pkg/conversion"
	"github.com/golang/protobuf/ptypes"

	"k8s.io/api/extensions/v1beta1"
)

func assertNumberOfVirtualHosts(t *testing.T, filterChain *listener.FilterChain, expected int) {
	var connManager hcm.HttpConnectionManager
	var dynamicAny ptypes.DynamicAny

	err := ptypes.UnmarshalAny(filterChain.Filters[0].GetTypedConfig(), &dynamicAny)
	if err != nil {
		t.Fatal(err)
	}

	structMessage, err := util.MessageToStruct(dynamicAny.Message)
	if err != nil {
		t.Fatal(err)
	}

	err = util.StructToMessage(structMessage, &connManager)
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

func assertServerNames(t *testing.T, filterChain *listener.FilterChain, expectedServerNames []string) {
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
	}, "d", []string{"bar"})

	snapshot := configurator.Generate(ingresses)

	if len(snapshot.Resources[cache.Listener].Items) != 1 {
		t.Fatalf("Num listeners: %d", len(snapshot.Resources[cache.Listener].Items))
	}
	if len(snapshot.Resources[cache.Cluster].Items) != 1 {
		t.Fatalf("Num clusters: %d", len(snapshot.Resources[cache.Cluster].Items))
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
	}, "d", []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Resources[cache.Listener].Items["listener_0"].(*v2.Listener)

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
	}, "d", []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Resources[cache.Listener].Items["listener_0"].(*v2.Listener)

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
	}, "d", []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Resources[cache.Listener].Items["listener_0"].(*v2.Listener)

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
	}, "d", []string{"bar"})

	snapshot := configurator.Generate(ingresses)
	listener := snapshot.Resources[cache.Listener].Items["listener_0"].(*v2.Listener)

	if len(listener.FilterChains) != 2 {
		t.Fatalf("Num filter chains: %d expected %d", len(listener.FilterChains), 2)
	}

	assertNumberOfVirtualHosts(t, listener.FilterChains[0], 1)
	assertServerNames(t, listener.FilterChains[0], []string{"*.internal.api.com"})

	assertNumberOfVirtualHosts(t, listener.FilterChains[1], 1)
	assertServerNames(t, listener.FilterChains[1], nil)
}

func TestGenerateListeners(t *testing.T) {
	testcases := []struct {
		name        string
		certs       []Certificate
		virtualHost []*virtualHost
		serverNames []string
	}{
		{
			name:  "http",
			certs: nil,
			virtualHost: []*virtualHost{
				{Host: "foo", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
				{Host: "bar", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
			},
			serverNames: []string{"foo", "bar"},
		},
		{
			name: "https",
			certs: []Certificate{
				{
					Hosts: []string{"foo", "bar"},
					Cert:  "cert",
					Key:   "key",
				},
			},
			virtualHost: []*virtualHost{
				{Host: "foo", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
				{Host: "bar", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
			},
			serverNames: []string{"foo", "bar"},
		},
		{
			name: "more-certs-than-hosts",
			certs: []Certificate{
				{
					Hosts: []string{"foo", "bar"},
					Cert:  "cert",
					Key:   "key",
				}, {
					Hosts: []string{"baz"},
					Cert:  "cert",
					Key:   "key",
				},
			},
			virtualHost: []*virtualHost{
				{Host: "foo", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
				{Host: "bar", Timeout: 1 * time.Second, PerTryTimeout: 500 * time.Millisecond},
			},
			serverNames: []string{"foo", "bar"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			configurator := NewKubernetesConfigurator("a", tc.certs, "", nil)
			ret := configurator.generateListeners(&envoyConfiguration{VirtualHosts: tc.virtualHost})
			listener := ret[0].(*v2.Listener)
			if len(listener.FilterChains) != 1 {
				t.Fatalf("filterchain number missmatch")
			}
			assertNumberOfVirtualHosts(t, listener.FilterChains[0], 2)
			if len(tc.certs) > 0 {
				if listener.FilterChains[0].FilterChainMatch == nil {
					t.Fatalf("Expected filter chain")
				}
			}
		})
	}
}
