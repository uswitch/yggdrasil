package envoy

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	cal "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	v3cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	tracing "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	eal "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	gal "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	eauthz "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	hcfg "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/health_check/v3"
	tls_inspector "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	previousHosts "github.com/envoyproxy/go-control-plane/envoy/extensions/retry/host/previous_hosts/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_extension_http "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	any "github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	jsonFormat      *structpb.Struct
	allowedRetryOns map[string]bool

	DefaultAccessLogFormat = map[string]interface{}{
		"bytes_received":            "%BYTES_RECEIVED%",
		"bytes_sent":                "%BYTES_SENT%",
		"downstream_local_address":  "%DOWNSTREAM_LOCAL_ADDRESS%",
		"downstream_remote_address": "%DOWNSTREAM_REMOTE_ADDRESS%",
		"duration":                  "%DURATION%",
		"forwarded_for":             "%REQ(X-FORWARDED-FOR)%",
		"protocol":                  "%PROTOCOL%",
		"request_id":                "%REQ(X-REQUEST-ID)%",
		"request_method":            "%REQ(:METHOD)%",
		"request_path":              "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
		"response_code":             "%RESPONSE_CODE%",
		"response_flags":            "%RESPONSE_FLAGS%",
		"start_time":                "%START_TIME(%s.%3f)%",
		"upstream_cluster":          "%UPSTREAM_CLUSTER%",
		"upstream_host":             "%UPSTREAM_HOST%",
		"upstream_local_address":    "%UPSTREAM_LOCAL_ADDRESS%",
		"upstream_service_time":     "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
		"user_agent":                "%REQ(USER-AGENT)%",
	}
)

func init() {
	allowedRetryOns = map[string]bool{
		"5xx":                        true,
		"gateway-error":              true,
		"reset":                      true,
		"connect-failure":            true,
		"envoy-ratelimited":          true,
		"retriable-4xx":              true,
		"refused-stream":             true,
		"retriable-status-codes":     true,
		"retriable-headers":          true,
		"http3-post-connect-failure": true,
	}
}

func makeVirtualHost(vhost *virtualHost, reselectionAttempts int64, defaultRetryOn string) (*route.VirtualHost, error) {
	retryOn := vhost.RetryOn
	if retryOn == "" {
		retryOn = defaultRetryOn
	}

	action := &route.Route_Route{
		Route: &route.RouteAction{
			Timeout: &duration.Duration{Seconds: int64(vhost.Timeout.Seconds())},
			ClusterSpecifier: &route.RouteAction_Cluster{
				Cluster: vhost.UpstreamCluster,
			},
			RetryPolicy: &route.RetryPolicy{
				RetryOn:       retryOn,
				PerTryTimeout: &duration.Duration{Seconds: int64(vhost.PerTryTimeout.Seconds())},
			},
		},
	}

	hosts := &previousHosts.PreviousHostsPredicate{}

	anyHosts, err := anypb.New(hosts)
	if err != nil {
		return &route.VirtualHost{}, fmt.Errorf("failed to marshal hosts config struct to typed struct: %s", err)
	}

	if reselectionAttempts >= 0 {
		action.Route.RetryPolicy.RetryHostPredicate = []*route.RetryPolicy_RetryHostPredicate{
			{
				Name:       "envoy.retry_host_predicates.previous_hosts",
				ConfigType: &route.RetryPolicy_RetryHostPredicate_TypedConfig{TypedConfig: anyHosts},
			},
		}
		action.Route.RetryPolicy.HostSelectionRetryMaxAttempts = reselectionAttempts
	}
	virtualHost := route.VirtualHost{
		Name:    "local_service",
		Domains: []string{vhost.Host},
		Routes: []*route.Route{
			{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				Action: action,
			},
		},
	}
	return &virtualHost, nil
}

func makeHealthConfig() *hcfg.HealthCheck {
	return &hcfg.HealthCheck{
		PassThroughMode: &wrappers.BoolValue{Value: false},
		Headers: []*route.HeaderMatcher{
			{
				Name: ":path",
				HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
					StringMatch: &matcherv3.StringMatcher{
						MatchPattern: &matcherv3.StringMatcher_Exact{Exact: "/yggdrasil/status"},
					},
				},
			},
		},
	}
}

func makeExtAuthzConfig(cfg HttpExtAuthz) *eauthz.ExtAuthz {
	return &eauthz.ExtAuthz{
		TransportApiVersion: core.ApiVersion_V3,
		Services: &eauthz.ExtAuthz_GrpcService{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: cfg.Cluster,
					},
				},
				Timeout: durationpb.New(cfg.Timeout),
			},
		},
		WithRequestBody: &eauthz.BufferSettings{
			MaxRequestBytes:     cfg.MaxRequestBytes,
			AllowPartialMessage: cfg.AllowPartialMessage,
			PackAsBytes:         cfg.PackAsBytes,
		},
		FailureModeAllow: cfg.FailureModeAllow,
	}
}

func makeGrpcLoggerConfig(cfg HttpGrpcLogger) *gal.HttpGrpcAccessLogConfig {
	return &gal.HttpGrpcAccessLogConfig{
		CommonConfig: &gal.CommonGrpcAccessLogConfig{
			LogName: cfg.Name,
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: cfg.Cluster,
					},
				},
				Timeout: durationpb.New(cfg.Timeout),
			},
			TransportApiVersion: core.ApiVersion_V3,
		},
		AdditionalRequestHeadersToLog:  cfg.RequestHeaders,
		AdditionalResponseHeadersToLog: cfg.ResponseHeaders,
	}
}

func makeFileAccessLog(cfg AccessLogger, accessLog string) *eal.FileAccessLog {
	format := DefaultAccessLogFormat
	if len(cfg.Format) > 0 {
		format = cfg.Format
	}

	b, err := structpb.NewValue(format)
	if err != nil {
		log.Fatal(err)
	}
	jsonFormat = b.GetStructValue()

	accessLogConfig := &eal.FileAccessLog{
		Path: filepath.Join(accessLog, "access.log"),
		AccessLogFormat: &eal.FileAccessLog_LogFormat{
			LogFormat: &core.SubstitutionFormatString{
				Format: &core.SubstitutionFormatString_JsonFormat{
					JsonFormat: jsonFormat,
				},
			},
		},
	}

	return accessLogConfig
}

func makeZipkinTracingProvider() *tracing.ZipkinConfig {
	zipkinTracingProviderConfig := &tracing.ZipkinConfig{
		CollectorCluster:         "zipkin",
		CollectorEndpoint:        "/api/v2/spans",
		CollectorEndpointVersion: tracing.ZipkinConfig_HTTP_JSON,
	}

	return zipkinTracingProviderConfig
}

func (c *KubernetesConfigurator) makeConnectionManager(virtualHosts []*route.VirtualHost, accessLog string) (*hcm.HttpConnectionManager, error) {
	// Access Logs
	accessLogConfig := makeFileAccessLog(c.accessLogger, accessLog)
	anyAccessLogConfig, err := anypb.New(accessLogConfig)
	if err != nil {
		log.Fatalf("failed to marshal access log config struct to typed struct: %s", err)
	}

	accessLoggers := []*cal.AccessLog{
		{
			Name:       "envoy.access_loggers.file",
			ConfigType: &cal.AccessLog_TypedConfig{TypedConfig: anyAccessLogConfig},
		},
	}

	if c.httpGrpcLogger.Cluster != "" {
		anyGrpcLoggerConfig, err := anypb.New(makeGrpcLoggerConfig(c.httpGrpcLogger))
		if err != nil {
			log.Fatalf("failed to marshal healthcheck config struct to typed struct: %s", err)
		}
		accessLoggers = append(accessLoggers, &cal.AccessLog{
			Name:       "envoy.access_loggers.http_grpc",
			ConfigType: &cal.AccessLog_TypedConfig{TypedConfig: anyGrpcLoggerConfig},
		})
	}

	// HTTP Filters
	filterBuilder := &httpFilterBuilder{}

	anyHealthConfig, err := anypb.New(makeHealthConfig())
	if err != nil {
		log.Fatalf("failed to marshal healthcheck config struct to typed struct: %s", err)
	}

	filterBuilder.Add(&hcm.HttpFilter{
		Name:       "envoy.filters.http.health_check",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: anyHealthConfig},
	})

	if c.httpExtAuthz.Cluster != "" {
		anyExtAuthzConfig, err := anypb.New(makeExtAuthzConfig(c.httpExtAuthz))
		if err != nil {
			log.Fatalf("failed to marshal extAuthz config struct to typed struct: %s", err)
		}

		filterBuilder.Add(&hcm.HttpFilter{
			Name:       "envoy.filters.http.ext_authz",
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: anyExtAuthzConfig},
		})
	}

	filter, err := filterBuilder.Filters()
	if err != nil {
		return &hcm.HttpConnectionManager{}, err
	}

	tracingConfig := &hcm.HttpConnectionManager_Tracing{}

	if c.tracingProvider == "zipkin" {
		zipkinTracingProvider, err := anypb.New(makeZipkinTracingProvider())
		if err != nil {
			log.Fatalf("failed to set zipkin tracing provider config: %s", err)
		}

		tracingConfig.Provider = &tracing.Tracing_Http{
			Name:       "config.trace.v3.Tracing.Http",
			ConfigType: &tracing.Tracing_Http_TypedConfig{TypedConfig: zipkinTracingProvider},
		}
	}

	return &hcm.HttpConnectionManager{
		CodecType:   hcm.HttpConnectionManager_AUTO,
		StatPrefix:  "ingress_http",
		HttpFilters: filter,
		UpgradeConfigs: []*hcm.HttpConnectionManager_UpgradeConfig{
			{
				UpgradeType: "websocket",
			},
		},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &route.RouteConfiguration{
				Name:         "local_route",
				VirtualHosts: virtualHosts,
			},
		},
		Tracing:               tracingConfig,
		AccessLog:             accessLoggers,
		UseRemoteAddress:      &wrapperspb.BoolValue{Value: c.useRemoteAddress},
		StripMatchingHostPort: true,
	}, nil
}

func (c *KubernetesConfigurator) makeFilterChain(certificate Certificate, virtualHosts []*route.VirtualHost, accessLog string) (listener.FilterChain, error) {
	httpConnectionManager, err := c.makeConnectionManager(virtualHosts, accessLog)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to get httpConnectionManager: %s", err)
	}
	anyHttpConfig, err := anypb.New(httpConnectionManager)
	if err != nil {
		return listener.FilterChain{}, fmt.Errorf("failed to marshal HTTP config struct to typed struct: %s", err)
	}

	tls := &auth.DownstreamTlsContext{}
	tls.CommonTlsContext = &auth.CommonTlsContext{
		AlpnProtocols: c.alpnProtocols,
		TlsCertificates: []*auth.TlsCertificate{
			{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: certificate.Cert},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{InlineString: certificate.Key},
				},
			},
		},
		TlsParams: &auth.TlsParameters{
			TlsMinimumProtocolVersion: auth.TlsParameters_TLSv1_2,
		},
	}

	anyTls, err := anypb.New(tls)
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
			{
				Name:       "envoy.filters.network.http_connection_manager",
				ConfigType: &listener.Filter_TypedConfig{TypedConfig: anyHttpConfig},
			},
		},
		TransportSocket: &core.TransportSocket{
			Name:       "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{TypedConfig: anyTls},
		},
	}, nil
}

func makeListener(filterChains []*listener.FilterChain, envoyListenerIpv4Address []string, envoyListenPort uint32) (*listener.Listener, error) {
	tlsInspectorConfig, err := anypb.New(&tls_inspector.TlsInspector{})
	if err != nil {
		return &listener.Listener{}, fmt.Errorf("failed to marshal tls_inspector config struct to typed struct: %s", err)
	}

	additional_addresses := make([]*listener.AdditionalAddress, len(envoyListenerIpv4Address)-1)
	for i, address := range envoyListenerIpv4Address {
		/// Skip the first address as it will be the principal address of the listener
		if i == 0 {
			continue
		}
		additional_address := listener.AdditionalAddress{
			Address: &core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Address: address,
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: envoyListenPort,
						},
					},
				},
			},
		}
		additional_addresses[i-1] = &additional_address
	}

	listener := listener.Listener{
		Name: "listener_0",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: envoyListenerIpv4Address[0],
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: envoyListenPort,
					},
				},
			},
		},
		AdditionalAddresses: additional_addresses,
		ListenerFilters: []*listener.ListenerFilter{
			{
				Name:       "envoy.filters.listener.tls_inspector",
				ConfigType: &listener.ListenerFilter_TypedConfig{TypedConfig: tlsInspectorConfig},
			},
		},
		FilterChains: filterChains,
		// Setting the TrafficDirection here for tracing
		TrafficDirection: core.TrafficDirection_OUTBOUND,
	}

	return &listener, nil
}

func makeAddresses(addresses []LBHost, upstreamPort uint32) []*core.Address {

	envoyAddresses := []*core.Address{}
	for _, address := range addresses {
		envoyAddress := &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Address: address.Host,
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

func makeCluster(c cluster, ca string, healthCfg UpstreamHealthCheck, outlierPercentage int32, addresses []*core.Address) *v3cluster.Cluster {

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
		anyTls, err = anypb.New(tls)
		if err != nil {
			log.Printf("Error marhsalling cluster TLS config: %s", err)
		}
	}

	healthChecks := makeHealthChecks(c.HealthCheckHost, c.HealthCheckPath, healthCfg)

	endpoints := make([]*endpoint.LbEndpoint, len(addresses))

	for idx, address := range addresses {
		endpoints[idx] = &endpoint.LbEndpoint{
			HostIdentifier:      &endpoint.LbEndpoint_Endpoint{Endpoint: &endpoint.Endpoint{Address: address}},
			LoadBalancingWeight: &wrappers.UInt32Value{Value: c.Hosts[idx].Weight},
		}
	}

	var httpOptions *envoy_extension_http.HttpProtocolOptions
	if c.HttpVersion == "1.1" {

		httpOptions = &envoy_extension_http.HttpProtocolOptions{
			CommonHttpProtocolOptions: &core.HttpProtocolOptions{
				IdleTimeout:              &duration.Duration{Seconds: 60},
				MaxConnectionDuration:    &durationpb.Duration{Seconds: 60},
				MaxRequestsPerConnection: &wrapperspb.UInt32Value{Value: 10000},
			},
			UpstreamProtocolOptions: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
						HttpProtocolOptions: &core.Http1ProtocolOptions{},
					},
				},
			},
		}
	} else { // TODO be more specific, handle default version
		httpOptions = &envoy_extension_http.HttpProtocolOptions{
			CommonHttpProtocolOptions: &core.HttpProtocolOptions{
				IdleTimeout:              &duration.Duration{Seconds: 60},
				MaxConnectionDuration:    &durationpb.Duration{Seconds: 60},
				MaxRequestsPerConnection: &wrapperspb.UInt32Value{Value: 10000},
			},
			UpstreamProtocolOptions: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_extension_http.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
						Http2ProtocolOptions: &core.Http2ProtocolOptions{
							AllowConnect:         true,
							MaxConcurrentStreams: &wrapperspb.UInt32Value{Value: 128},
						},
					},
				},
			},
		}
	}

	httpOptionsPb, err := anypb.New(httpOptions)
	if err != nil {
		log.Fatalf("Error marshaling httpOptions: %s", err)
	}

	cluster := &v3cluster.Cluster{
		ClusterDiscoveryType: &v3cluster.Cluster_Type{Type: v3cluster.Cluster_STRICT_DNS},
		Name:                 c.Name,
		ConnectTimeout:       durationpb.New(c.Timeout),
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: c.Name,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{LbEndpoints: endpoints},
			},
		},
		HealthChecks: healthChecks,
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpOptionsPb,
		},
		CircuitBreakers: &v3cluster.CircuitBreakers{
			Thresholds: []*v3cluster.CircuitBreakers_Thresholds{
				&v3cluster.CircuitBreakers_Thresholds{
					Priority:           core.RoutingPriority_DEFAULT,
					MaxConnections:     wrapperspb.UInt32(32768),
					MaxRequests:        wrapperspb.UInt32(32768),
					MaxPendingRequests: wrapperspb.UInt32(32768),
				},
			},
		},
	}
	if outlierPercentage >= 0 {
		cluster.OutlierDetection = &v3cluster.OutlierDetection{
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

func ValidateEnvoyRetryOn(retryOn string) bool {
	retryOnList := strings.Split(retryOn, ",")

	for _, ro := range retryOnList {
		if !allowedRetryOns[ro] {
			return false
		}
	}
	return true
}
