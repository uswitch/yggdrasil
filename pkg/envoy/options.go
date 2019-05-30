package envoy

type option func(c *KubernetesConfigurator)

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
