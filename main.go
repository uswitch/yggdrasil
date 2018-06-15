package main

import (
	"context"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	log "github.com/sirupsen/logrus"
	"github.com/uswitch/yggdrasil/pkg/envoy"
	"github.com/uswitch/yggdrasil/pkg/k8s"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/staging/src/k8s.io/sample-controller/pkg/signals"
)

type options struct {
	kubeconfig []string
	debugLog   bool
}

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}

func main() {
	opts := &options{}
	kingpin.Flag("kubeconfig", "Path to kubeconfig.").StringsVar(&opts.kubeconfig)
	kingpin.Flag("debug", "Log at debug level").BoolVar(&opts.debugLog)
	kingpin.Parse()

	if opts.debugLog {
		log.SetLevel(log.DebugLevel)
	}

	stopCh := signals.SetupSignalHandler()

	sources := make([]k8scache.ListerWatcher, len(opts.kubeconfig))

	for i, configPath := range opts.kubeconfig {
		config, err := createClientConfig(configPath)
		if err != nil {
			log.Fatalf("error creating client config: %s", err)
		}

		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("error creating kube client: %s", err)
		}
		sources[i] = k8s.NewListWatch(clientSet)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//envoyNode := "hello"
	hash := Hasher{}
	envoyCache := cache.NewSnapshotCache(false, hash, nil)

	lister := k8s.NewIngressAggregator(sources)
	configurator := envoy.NewKubernetesConfigurator(lister, "multi-cluster", "hello")
	snapshotter := envoy.NewSnapshotter(envoyCache, configurator, lister.Events())
	go snapshotter.Run(ctx)
	lister.Run(ctx)

	envoyServer := server.NewServer(envoyCache, &callbacks{})
	go runEnvoyServer(envoyServer, ctx.Done())

	<-stopCh
}

func createClientConfig(path string) (*rest.Config, error) {
	if path == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", path)
}
