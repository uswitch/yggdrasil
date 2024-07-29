package envoy

import "github.com/prometheus/client_golang/prometheus"

var (
	matchingIngresses = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "yggdrasil",
			Name:      "ingresses",
			Help:      "Total number of matching ingress objects",
		},
	)

	numClusters = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "yggdrasil",
			Name:      "clusters",
			Help:      "Total number of clusters generated",
		},
	)

	numVhosts = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "yggdrasil",
			Name:      "virtual_hosts",
			Help:      "Total number of virtual hosts generated",
		},
	)

	clusterUpdates = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "yggdrasil",
			Name:      "cluster_updates",
			Help:      "Number of times the clusters have been updated",
		},
	)

	listenerUpdates = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "yggdrasil",
			Name:      "listener_updates",
			Help:      "Number of times the listener has been updated",
		},
	)

	KubernetesClusterInMaintenance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "yggdrasil",
			Name:      "kubernetes_cluster_in_maintenance",
			Help:      "Is kubernetes cluster in maintenance mode ?",
		},
		[]string{"apiServer"},
	)

	EnvoyUpstreamInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "yggdrasil",
			Name:      "upstream_info",
			Help:      "Retrieve information about cluster",
		},
		[]string{"envoy_cluster_name", "upstream", "namespace", "ingressclass", "k8s_cluster", "ingress"},
	)
)

func init() {
	prometheus.MustRegister(matchingIngresses, numClusters, numVhosts, clusterUpdates, listenerUpdates, KubernetesClusterInMaintenance, EnvoyUpstreamInfo)
}
