package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	discover "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type callbacks struct {
	fetchReq  int
	fetchResp int
}

func (c *callbacks) OnStreamOpen(context.Context, int64, string) error {
	return nil
}
func (c *callbacks) OnStreamClosed(int64) {}
func (c *callbacks) OnStreamRequest(int64, *v2.DiscoveryRequest) error {
	return nil
}
func (c *callbacks) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
func (c *callbacks) OnFetchRequest(context.Context, *v2.DiscoveryRequest) error {
	c.fetchReq++
	return nil
}
func (c *callbacks) OnFetchResponse(*v2.DiscoveryRequest, *v2.DiscoveryResponse) {
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

	discover.RegisterAggregatedDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterEndpointDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterClusterDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterRouteDiscoveryServiceServer(grpcServer, envoyServer)
	v2.RegisterListenerDiscoveryServiceServer(grpcServer, envoyServer)

	healthMux := http.NewServeMux()

	healthServer := &http.Server{
		Addr:    fmt.Sprintf(healthAddress),
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
