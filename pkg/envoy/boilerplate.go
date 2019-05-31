package envoy

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	v2cluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	fal "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	al "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/types"
)

var (
	jsonFormat string
)

func init() {
	format := map[string]string{
		"start_time":                "%START_TIME(%s.%3f)%",
		"bytes_received":            "%BYTES_RECEIVED%",
		"protocol":                  "%PROTOCOL%",
		"response_code":             "%RESPONSE_CODE%",
		"bytes_sent":                "%BYTES_SENT%",
		"duration":                  "%DURATION%",
		"response_flags":            "%RESPONSE_FLAGS%",
		"upstream_host":             "%UPSTREAM_HOST%",
		"upstream_cluster":          "%UPSTREAM_CLUSTER%",
		"upstream_local_address":    "%UPSTREAM_LOCAL_ADDRESS%",
		"downstream_remote_address": "%DOWNSTREAM_REMOTE_ADDRESS%",
		"downstream_local_address":  "%DOWNSTREAM_LOCAL_ADDRESS%",
		"request_method":            "%REQ(:METHOD)%",
		"request_path":              "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
		"upstream_service_time":     "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
		"forwarded_for":             "%REQ(X-FORWARDED-FOR)%",
		"user_agent":                "%REQ(USER-AGENT)%",
		"request_id":                "%REQ(X-REQUEST-ID)%",
	}
	b, err := json.Marshal(format)
	if err != nil {
		log.Fatal(err)
	}
	jsonFormat = string(b) + "\n"
}

func makeVirtualHost(vhost *virtualHost, reselectionAttempts int64) route.VirtualHost {

	action := &route.Route_Route{
		Route: &route.RouteAction{
			Timeout: &vhost.Timeout,
			ClusterSpecifier: &route.RouteAction_Cluster{
				Cluster: vhost.Host,
			},
			RetryPolicy: &route.RouteAction_RetryPolicy{
				RetryOn:       "5xx",
				PerTryTimeout: &vhost.PerTryTimeout,
			},
		},
	}

	if reselectionAttempts >= 0 {
		action.Route.RetryPolicy.RetryHostPredicate = []*route.RouteAction_RetryPolicy_RetryHostPredicate{
			{
				Name: "envoy.retry_host_predicates.previous_hosts",
			},
		}
		action.Route.RetryPolicy.HostSelectionRetryMaxAttempts = reselectionAttempts
	}
	virtualHost := route.VirtualHost{
		Name:    "local_service",
		Domains: []string{vhost.Host},
		Routes: []route.Route{route.Route{
			Match: route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: action,
		}},
	}
	return virtualHost
}

func makeConnectionManager(virtualHosts []route.VirtualHost) *hcm.HttpConnectionManager {
	accessLogConfig, err := util.MessageToStruct(&fal.FileAccessLog{
		Path:   "/var/log/envoy/access.log",
		Format: jsonFormat,
	})
	if err != nil {
		log.Fatalf("failed to convert: %s", err)
	}
	return &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "ingress_http",
		HttpFilters: []*hcm.HttpFilter{&hcm.HttpFilter{
			Name: "envoy.router",
		}},
		UpgradeConfigs: []*hcm.HttpConnectionManager_UpgradeConfig{
			{
				UpgradeType: "websocket",
			},
		},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "local_route",
				VirtualHosts: virtualHosts,
			},
		},
		Tracing: &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.EGRESS,
		},
		AccessLog: []*al.AccessLog{
			{
				Name:   "envoy.file_access_log",
				Config: accessLogConfig,
			},
		},
	}
}

func makeFilterChain(certificate Certificate, virtualHosts []route.VirtualHost) (listener.FilterChain, error) {
	httpConnectionManager := makeConnectionManager(virtualHosts)
	httpConfig, err := util.MessageToStruct(httpConnectionManager)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to convert: %s", err)
	}

	tls := &auth.DownstreamTlsContext{}
	tls.CommonTlsContext = &auth.CommonTlsContext{
		TlsCertificates: []*auth.TlsCertificate{
			&auth.TlsCertificate{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: certificate.Cert},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: certificate.Key},
				},
			},
		},
	}

	filterChainMatch := &listener.FilterChainMatch{}

	hosts := []string{}

	for _, host := range certificate.Hosts {
		if host != "*" {
			hosts = append(hosts, host)
		}
	}

	if len(hosts) > 0 {
		filterChainMatch.ServerNames = hosts
	}

	return listener.FilterChain{
		TlsContext:       tls,
		FilterChainMatch: filterChainMatch,
		Filters: []listener.Filter{
			listener.Filter{
				Name:   "envoy.http_connection_manager",
				Config: httpConfig,
			},
		},
	}, nil
}

func makeListener(filterChains []listener.FilterChain, envoyListenPort uint32) *v2.Listener {

	listener := v2.Listener{
		Name: "listener_0",
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: envoyListenPort,
					},
				},
			},
		},
		ListenerFilters: []listener.ListenerFilter{
			{Name: "envoy.listener.tls_inspector"},
		},
		FilterChains: filterChains,
	}

	return &listener
}

func makeAddresses(addresses []string, upstreamPort uint32) []*core.Address {

	envoyAddresses := []*core.Address{}
	for _, address := range addresses {
		envoyAddress := &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: address,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: upstreamPort,
					},
				},
			},
		}
		envoyAddresses = append(envoyAddresses, envoyAddress)
	}

	return envoyAddresses
}

func makeHealthChecks(healthPath string) []*core.HealthCheck {
	healthChecks := []*core.HealthCheck{}
	healthCheckTimeout, _ := time.ParseDuration("5s")
	healthCheckInterval, _ := time.ParseDuration("10s")

	if healthPath != "" {
		check := &core.HealthCheck{
			Timeout:            &healthCheckTimeout,
			Interval:           &healthCheckInterval,
			UnhealthyThreshold: &types.UInt32Value{Value: 3},
			HealthyThreshold:   &types.UInt32Value{Value: 3},
			HealthChecker: &core.HealthCheck_HttpHealthCheck_{
				HttpHealthCheck: &core.HealthCheck_HttpHealthCheck{
					Path: healthPath,
				},
			},
		}
		healthChecks = append(healthChecks, check)
	}

	return healthChecks
}

func makeCluster(host, ca, healthPath string, timeout time.Duration, outlierPercentage int32, addresses []*core.Address) *v2.Cluster {

	tls := &auth.UpstreamTlsContext{}
	if ca != "" {
		tls.CommonTlsContext = &auth.CommonTlsContext{
			ValidationContextType: &auth.CommonTlsContext_ValidationContext{
				ValidationContext: &auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_Filename{Filename: ca},
					},
				},
			},
		}
	} else {
		tls = nil
	}
	healthChecks := makeHealthChecks(healthPath)

	endpoints := make([]endpoint.LbEndpoint, len(addresses))

	for idx, address := range addresses {
		endpoints[idx] = endpoint.LbEndpoint{
			Endpoint: &endpoint.Endpoint{Address: address},
		}
	}

	cluster := &v2.Cluster{
		Type:           v2.Cluster_STRICT_DNS,
		Name:           host,
		ConnectTimeout: timeout,
		LoadAssignment: &v2.ClusterLoadAssignment{
			ClusterName: host,
			Endpoints: []endpoint.LocalityLbEndpoints{
				{LbEndpoints: endpoints},
			},
		},
		TlsContext:   tls,
		HealthChecks: healthChecks,
	}
	if outlierPercentage >= 0 {
		cluster.OutlierDetection = &v2cluster.OutlierDetection{
			MaxEjectionPercent: &types.UInt32Value{Value: uint32(outlierPercentage)},
		}
	}
	return cluster
}
