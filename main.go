package main

import (
	"context"
	"encoding/json"
	"io/ioutil"

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
	kubeconfig   []string
	debugLog     bool
	ingressClass string
	nodeName     string
	config       string
}

type clusterConfig struct {
	APIServer string `json:"apiServer"`
	Ca        string `json:"ca"`
	Token     string `json:"token"`
}

type config struct {
	IngressClass string          `json:"ingressClass"`
	NodeName     string          `json:"nodeName"`
	Clusters     []clusterConfig `json:"clusters"`
}

var (
	ingressClass string
	nodeName     string
	sources      []k8scache.ListerWatcher
)

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
	kingpin.Flag("ingress-class", "Ingress class to watch").StringVar(&opts.ingressClass)
	kingpin.Flag("node-name", "Envoy node name").StringVar(&opts.nodeName)
	kingpin.Flag("debug", "Log at debug level").BoolVar(&opts.debugLog)
	kingpin.Flag("config", "Config file path").StringVar(&opts.config)
	kingpin.Parse()

	if opts.debugLog {
		log.SetLevel(log.DebugLevel)
	}

	if opts.config != "" {
		err := parseConfig(opts.config)
		if err != nil {
			log.Fatalf("error parsing config file: %s", err)
		}
	}

	if len(opts.kubeconfig) != 0 {
		err := configFromKubeConfig(opts.kubeconfig)
		if err != nil {
			log.Fatalf("error parsing kube config %s", err)
		}
	}
	if opts.ingressClass != "" {
		ingressClass = opts.ingressClass
	}
	if opts.nodeName != "" {
		nodeName = opts.nodeName
	}

	stopCh := signals.SetupSignalHandler()

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

	hash := Hasher{}
	envoyCache := cache.NewSnapshotCache(false, hash, nil)

	lister := k8s.NewIngressAggregator(sources)
	configurator := envoy.NewKubernetesConfigurator(lister, ingressClass, nodeName)
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

func parseConfig(path string) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var conf config
	err = json.Unmarshal(bytes, &conf)
	if err != nil {
		return err
	}

	ingressClass = conf.IngressClass
	nodeName = conf.NodeName

	for _, cluster := range conf.Clusters {
		config := &rest.Config{
			BearerToken: cluster.Token,
			Host:        cluster.APIServer,
			TLSClientConfig: rest.TLSClientConfig{
				CAFile: cluster.Ca,
			},
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		sources = append(sources, k8s.NewListWatch(clientSet))
	}
	return nil
}

func configFromKubeConfig(paths []string) error {
	for _, configPath := range paths {
		config, err := createClientConfig(configPath)
		if err != nil {
			return err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		sources = append(sources, k8s.NewListWatch(clientSet))
	}
	return nil
}
