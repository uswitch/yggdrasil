package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"

	cluster "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type callbacks struct {
	fetchReq  int
	fetchResp int
}

func (c *callbacks) OnDeltaStreamClosed(int64) {}
func (c *callbacks) OnDeltaStreamOpen(context.Context, int64, string) error {
	return nil
}
func (c *callbacks) OnStreamDeltaRequest(int64, *discovery.DeltaDiscoveryRequest) error {
	return nil
}
func (c *callbacks) OnStreamDeltaResponse(int64, *discovery.DeltaDiscoveryRequest, *discovery.DeltaDiscoveryResponse) {
}
func (c *callbacks) OnStreamOpen(context.Context, int64, string) error {
	return nil
}
func (c *callbacks) OnStreamClosed(int64) {}
func (c *callbacks) OnStreamRequest(int64, *discovery.DiscoveryRequest) error {
	return nil
}
func (c *callbacks) OnStreamResponse(context.Context, int64, *discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {
}
func (c *callbacks) OnFetchRequest(context.Context, *discovery.DiscoveryRequest) error {
	c.fetchReq++
	return nil
}
func (c *callbacks) OnFetchResponse(*discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {
	c.fetchResp++
}

func runEnvoyServer(envoyServer server.Server, address string, healthAddress string, stopCh <-chan struct{}) {

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("failed to listen")
	}

	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, envoyServer)
	endpoint.RegisterEndpointDiscoveryServiceServer(grpcServer, envoyServer)
	cluster.RegisterClusterDiscoveryServiceServer(grpcServer, envoyServer)
	route.RegisterRouteDiscoveryServiceServer(grpcServer, envoyServer)
	listener.RegisterListenerDiscoveryServiceServer(grpcServer, envoyServer)

	healthMux := http.NewServeMux()

	healthServer := &http.Server{
		Addr:    fmt.Sprint(healthAddress),
		Handler: healthMux,
	}

	healthMux.Handle("/metrics", promhttp.Handler())
	healthMux.HandleFunc("/healthz", health)

	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to start grpc server: %v", err)
		}
	}()

	go func() {
		if err = healthServer.ListenAndServe(); err != nil {
			log.Fatalf("Failed to listen and serve health server: %v", err)
		}
	}()

	<-stopCh
	log.Info("shutting down server")
	grpcServer.GracefulStop()
}

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
