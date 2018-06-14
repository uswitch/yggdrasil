package main

import (
	"net"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	discover "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/kubernetes/staging/src/k8s.io/sample-controller/pkg/signals"
)

type options struct {
	kubeconfig string
}

// Hasher returns node ID as an ID
type Hasher struct {
}

type callbacks struct {
	fetchReq  int
	fetchResp int
}

func (c *callbacks) OnStreamOpen(int64, string)                                          {}
func (c *callbacks) OnStreamClosed(int64)                                                {}
func (c *callbacks) OnStreamRequest(int64, *v2.DiscoveryRequest)                         {}
func (c *callbacks) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
func (c *callbacks) OnFetchRequest(*v2.DiscoveryRequest) {
	c.fetchReq++
}
func (c *callbacks) OnFetchResponse(*v2.DiscoveryRequest, *v2.DiscoveryResponse) {
	c.fetchResp++
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
	kingpin.Flag("kubeconfig", "Path to kubeconfig.").StringVar(&opts.kubeconfig)
	kingpin.Parse()

	stopCh := signals.SetupSignalHandler()

	config, err := createClientConfig(opts)
	if err != nil {
		log.Fatalf("error creating client config: %s", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("error creating kube client: %s", err)
	}

	envoyNode := core.Node{Id: "hello"}
	hash := Hasher{}
	envoyCache := cache.NewSnapshotCache(false, hash, nil)
	cluster := &v2.Cluster{
		Name:           "foo",
		ConnectTimeout: time.Second * 30,
	}

	items := []cache.Resource{cluster}
	clusters := cache.NewResources("1", items)

	snapshot := cache.Snapshot{
		Clusters: clusters,
	}

	err = envoyCache.SetSnapshot(envoyNode.Id, snapshot)

	if err != nil {
		log.Fatal("ouch")
	}

	envoyServer := server.NewServer(envoyCache, &callbacks{})
	go runEnvoyServer(envoyServer, stopCh)
	kubeInformerFactory := informers.NewSharedInformerFactory(clientSet, time.Second*30)

	controller := NewController(clientSet, kubeInformerFactory.Extensions().V1beta1().Ingresses())

	go kubeInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}
}

func runEnvoyServer(envoyServer server.Server, stopCh <-chan struct{}) {

	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", "192.168.112.101:8080")
	if err != nil {
		log.Fatal("failed to listen")
	}

	discover.RegisterAggregatedDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterEndpointDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterClusterDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterRouteDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterListenerDiscoveryServiceServer(grpcServer, envoyServer)

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-stopCh
	log.Info("shutting down server")
	grpcServer.GracefulStop()
}

func createClientConfig(opts *options) (*rest.Config, error) {
	if opts.kubeconfig == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", opts.kubeconfig)
}
