package envoy

import (
	"context"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/networking/v1"

	"github.com/uswitch/yggdrasil/pkg/k8s"
)

//Configurator is an interface that implements Generate and NodeID
type Configurator interface {
	Generate([]v1.Ingress) cache.Snapshot
	NodeID() string
}

//Snapshotter watches for Ingress changes and updates the
//config snapshot
type Snapshotter struct {
	snapshotCache cache.SnapshotCache
	configurator  Configurator
	lister        *k8s.IngressAggregator
}

//NewSnapshotter returns a new Snapshotter
func NewSnapshotter(snapshotCache cache.SnapshotCache, config Configurator, lister *k8s.IngressAggregator) *Snapshotter {
	return &Snapshotter{snapshotCache: snapshotCache, configurator: config, lister: lister}
}

func (s *Snapshotter) snapshot() error {
	ingresses, err := s.lister.List()
	if err != nil {
		return err
	}
	snapshot := s.configurator.Generate(ingresses)

	log.Debugf("took snapshot: %+v", snapshot)

	s.snapshotCache.SetSnapshot(context.TODO(), s.configurator.NodeID(), &snapshot)
	return nil
}

//Run will periodically refresh the snapshot
func (s *Snapshotter) Run(ctx context.Context) {
	go func() {
		for {
			select {
			case <-s.lister.Events():
				s.snapshot()
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Infof("started snapshotter")
}
