package envoy

func WithEnvoyPort(port uint32) option {
	return func(c *KubernetesConfigurator) {
		c.envoyListenPort = port
	}
}

func WithUpstreamPort(port uint32) option {
	return func(c *KubernetesConfigurator) {
		c.upstreamPort = port
	}
}
