package main

import (
	"net"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	discover "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

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
