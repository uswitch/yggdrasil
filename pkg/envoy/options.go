package envoy

type option func(c *KubernetesConfigurator)

// WithEWithEnvoyListenerIpv4AddressnvoyPort configures envoy IPv4 listen address into a KubernetesConfigurator
func WithEnvoyListenerIpv4Address(address string) option {
	return func(c *KubernetesConfigurator) {
		c.envoyListenerIpv4Address = address
	}
}

// WithEnvoyPort configures the given envoy port into a KubernetesConfigurator
func WithEnvoyPort(port uint32) option {
	return func(c *KubernetesConfigurator) {
		c.envoyListenPort = port
	}
}

// WithUpstreamPort configures the given upstream port into a KubernetesConfigurator
func WithUpstreamPort(port uint32) option {
	return func(c *KubernetesConfigurator) {
		c.upstreamPort = port
	}
}

// WithOutlierPercentage configures the given percentage as maximal outlier percentage into a KubernetesConfigurator
func WithOutlierPercentage(percentage int32) option {
	return func(c *KubernetesConfigurator) {
		c.outlierPercentage = percentage
	}
}

// WithHostSelectionRetryAttempts configures number of host selection reattempts into a KubernetesConfigurator
func WithHostSelectionRetryAttempts(attempts int64) option {
	return func(c *KubernetesConfigurator) {
		c.hostSelectionRetryAttempts = attempts
	}
}

// WithUpstreamHealthCheck configures the upstream health check into the KubernetesConfigurator
func WithUpstreamHealthCheck(config UpstreamHealthCheck) option {
	return func(c *KubernetesConfigurator) {
		c.upstreamHealthCheck = config
	}
}

// WithUseRemoteAddress configures the useRemoteAddress option into the KubernetesConfigurator
func WithUseRemoteAddress(useRemoteAddress bool) option {
	return func(c *KubernetesConfigurator) {
		c.useRemoteAddress = useRemoteAddress
	}
}

// WithHttpExtAuthzCluster configures the options for the gRPC cluster
func WithHttpExtAuthzCluster(httpExtAuthz HttpExtAuthz) option {
	return func(c *KubernetesConfigurator) {
		c.httpExtAuthz = httpExtAuthz
	}
}

// WithHttpGrpcLogger configures the options for the gPRC access logger
func WithHttpGrpcLogger(httpGrpcLogger HttpGrpcLogger) option {
	return func(c *KubernetesConfigurator) {
		c.httpGrpcLogger = httpGrpcLogger
	}
}

// WithSyncSecrets configures the syncSecrets option
func WithSyncSecrets(syncSecrets bool) option {
	return func(c *KubernetesConfigurator) {
		c.syncSecrets = syncSecrets
	}
}

// WithDefaultRetryOn configures the default retry policy
func WithDefaultRetryOn(defaultRetryOn string) option {
	return func(c *KubernetesConfigurator) {
		c.defaultRetryOn = defaultRetryOn
	}
}
