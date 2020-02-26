package envoy

import (
	"encoding/json"
	"fmt"
	"log"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	v2cluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	fal "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	al "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcfg "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/health_check/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	util "github.com/envoyproxy/go-control-plane/pkg/conversion"
	types "github.com/golang/protobuf/ptypes"
	any "github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
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

func makeVirtualHost(vhost *virtualHost, reselectionAttempts int64) *route.VirtualHost {

	action := &route.Route_Route{
		Route: &route.RouteAction{
			Timeout: &duration.Duration{Seconds: int64(vhost.Timeout.Seconds())},
			ClusterSpecifier: &route.RouteAction_Cluster{
				Cluster: vhost.UpstreamCluster,
			},
			RetryPolicy: &route.RetryPolicy{
				RetryOn:       "5xx",
				PerTryTimeout: &duration.Duration{Seconds: int64(vhost.PerTryTimeout.Seconds())},
			},
		},
	}

	if reselectionAttempts >= 0 {
		action.Route.RetryPolicy.RetryHostPredicate = []*route.RetryPolicy_RetryHostPredicate{
			{
				Name: "envoy.retry_host_predicates.previous_hosts",
			},
		}
		action.Route.RetryPolicy.HostSelectionRetryMaxAttempts = reselectionAttempts
	}
	virtualHost := route.VirtualHost{
		Name:    "local_service",
		Domains: []string{vhost.Host},
		Routes: []*route.Route{&route.Route{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: action,
		}},
	}
	return &virtualHost
}

func makeHealthConfig() *hcfg.HealthCheck {
	return &hcfg.HealthCheck{
		PassThroughMode: &wrappers.BoolValue{Value: false},
		Headers: []*route.HeaderMatcher{
			&route.HeaderMatcher{
				Name: ":path",
				HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
					ExactMatch: "/yggdrasil/status",
				},
			},
		},
	}
}

func makeConnectionManager(virtualHosts []*route.VirtualHost) *hcm.HttpConnectionManager {
	accessLogConfig, err := util.MessageToStruct(&fal.FileAccessLog{
		Path:            "/var/log/envoy/access.log",
		AccessLogFormat: &fal.FileAccessLog_Format{Format: jsonFormat},
	})
	if err != nil {
		log.Fatalf("failed to convert access log proto message to struct: %s", err)
	}
	anyAccessLogConfig, err := types.MarshalAny(accessLogConfig)
	if err != nil {
		log.Fatalf("failed to marshal access log config struct to typed struct: %s", err)
	}
	healthConfig, err := util.MessageToStruct(makeHealthConfig())
	if err != nil {
		log.Fatalf("failed to convert healthcheck proto message to struct: %s", err)
	}
	anyHealthConfig, err := types.MarshalAny(healthConfig)
	if err != nil {
		log.Fatalf("failed to marshal healthcheck config struct to typed struct: %s", err)
	}
	return &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "ingress_http",
		HttpFilters: []*hcm.HttpFilter{
			&hcm.HttpFilter{
				Name:       "envoy.health_check",
				ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: anyHealthConfig},
			}, &hcm.HttpFilter{
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
		Tracing: &hcm.HttpConnectionManager_Tracing{},
		AccessLog: []*al.AccessLog{
			{
				Name:       "envoy.file_access_log",
				ConfigType: &al.AccessLog_TypedConfig{TypedConfig: anyAccessLogConfig},
			},
		},
	}
}

func makeFilterChain(certificate Certificate, virtualHosts []*route.VirtualHost) (listener.FilterChain, error) {
	httpConnectionManager := makeConnectionManager(virtualHosts)
	httpConfig, err := util.MessageToStruct(httpConnectionManager)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to convert virtualHost to envoy control plane struct: %s", err)
	}
	anyHttpConfig, err := types.MarshalAny(httpConfig)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to marshal HTTP config struct to typed struct: %s", err)
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

	anyTls, err := types.MarshalAny(tls)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to marshal TLS config struct to typed struct: %s", err)
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
		FilterChainMatch: filterChainMatch,
		Filters: []*listener.Filter{
			&listener.Filter{
				Name:       "envoy.http_connection_manager",
				ConfigType: &listener.Filter_TypedConfig{TypedConfig: anyHttpConfig},
			},
		},
		TransportSocket: &core.TransportSocket{
			Name:       "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{TypedConfig: anyTls},
		},
	}, nil
}

func makeListener(filterChains []*listener.FilterChain, envoyListenPort uint32) *v2.Listener {

	listener := v2.Listener{
		Name: "listener_0",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: envoyListenPort,
					},
				},
			},
		},
		ListenerFilters: []*listener.ListenerFilter{
			{Name: "envoy.listener.tls_inspector"},
		},
		FilterChains: filterChains,
		// Setting the TrafficDirection here for tracing
		TrafficDirection: core.TrafficDirection_OUTBOUND,
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

func makeHealthChecks(upstreamVHost string, healthPath string, config UpstreamHealthCheck) []*core.HealthCheck {
	healthChecks := []*core.HealthCheck{}

	if healthPath != "" {
		check := &core.HealthCheck{
			Timeout:            &duration.Duration{Seconds: int64(config.Timeout.Seconds())},
			Interval:           &duration.Duration{Seconds: int64(config.Interval.Seconds())},
			UnhealthyThreshold: &wrappers.UInt32Value{Value: config.UnhealthyThreshold},
			HealthyThreshold:   &wrappers.UInt32Value{Value: config.HealthyThreshold},
			HealthChecker: &core.HealthCheck_HttpHealthCheck_{
				HttpHealthCheck: &core.HealthCheck_HttpHealthCheck{
					Host: upstreamVHost,
					Path: healthPath,
				},
			},
		}
		healthChecks = append(healthChecks, check)
	}

	return healthChecks
}

func makeCluster(c cluster, ca string, healthCfg UpstreamHealthCheck, outlierPercentage int32, addresses []*core.Address) *v2.Cluster {

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

	var err error
	var anyTls *any.Any

	if tls != nil {
		anyTls, err = types.MarshalAny(tls)
		if err != nil {
			log.Printf("Error marhsalling cluster TLS config: %s", err)
		}
	}

	healthChecks := makeHealthChecks(c.VirtualHost, c.HealthCheckPath, healthCfg)

	endpoints := make([]*endpoint.LbEndpoint, len(addresses))

	for idx, address := range addresses {
		endpoints[idx] = &endpoint.LbEndpoint{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{Endpoint: &endpoint.Endpoint{Address: address}},
		}
	}

	cluster := &v2.Cluster{
		ClusterDiscoveryType: &v2.Cluster_Type{Type: v2.Cluster_STRICT_DNS},
		Name:                 c.Name,
		ConnectTimeout:       &duration.Duration{Seconds: int64(c.Timeout.Seconds())},
		LoadAssignment: &v2.ClusterLoadAssignment{
			ClusterName: c.Name,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{LbEndpoints: endpoints},
			},
		},
		HealthChecks: healthChecks,
	}
	if outlierPercentage >= 0 {
		cluster.OutlierDetection = &v2cluster.OutlierDetection{
			MaxEjectionPercent: &wrappers.UInt32Value{Value: uint32(outlierPercentage)},
		}
	}
	if anyTls != nil {
		cluster.TransportSocket = &core.TransportSocket{
			Name:       "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{TypedConfig: anyTls},
		}
	}

	return cluster
}
