package envoy

import (
	"context"

	"github.com/envoyproxy/go-control-plane/pkg/cache"
	log "github.com/sirupsen/logrus"
)

//Configurator is an interface that implements Generate and NodeID
type Configurator interface {
	Generate() (cache.Snapshot, error)
	NodeID() string
}

//Snapshotter watches for Ingress changes and updates the
//config snapshot
type Snapshotter struct {
	snapshotCache cache.SnapshotCache
	configurator  Configurator
	events        chan interface{}
}

//NewSnapshotter returns a new Snapshotter
func NewSnapshotter(snapshotCache cache.SnapshotCache, config Configurator, events chan interface{}) *Snapshotter {
	return &Snapshotter{snapshotCache: snapshotCache, configurator: config, events: events}
}

func (s *Snapshotter) snapshot() error {
	snapshot, err := s.configurator.Generate()
	if err != nil {
		return err
	}

	log.Debugf("took snapshot: %+v", snapshot)

	s.snapshotCache.SetSnapshot(s.configurator.NodeID(), snapshot)
	return nil
}

//Run will periodically refresh the snapshot
func (s *Snapshotter) Run(ctx context.Context) {
	go func() {
		for {
			select {
			case <-s.events:
				s.snapshot()
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Infof("started snapshotter")
}
