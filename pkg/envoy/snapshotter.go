package envoy

import (
	"context"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/uswitch/yggdrasil/pkg/k8s"
)

//Configurator is an interface that implements Generate and NodeID
type Configurator interface {
	Generate([]*k8s.Ingress) (cache.Snapshot, error)
	NodeID() string
}

//Snapshotter watches for Ingress changes and updates the
//config snapshot
type Snapshotter struct {
	snapshotCache cache.SnapshotCache
	configurator  Configurator
	aggregator    *k8s.Aggregator
}

//NewSnapshotter returns a new Snapshotter
func NewSnapshotter(snapshotCache cache.SnapshotCache, config Configurator, aggregator *k8s.Aggregator) *Snapshotter {
	return &Snapshotter{snapshotCache: snapshotCache, configurator: config, aggregator: aggregator}
}

func (s *Snapshotter) snapshot() error {
	genericIngresses, err := s.aggregator.GetGenericIngresses()
	if err != nil {
		return err
	}

	snapshot, err := s.configurator.Generate(genericIngresses)

	log.Debugf("took snapshot: %+v", snapshot)

	s.snapshotCache.SetSnapshot(context.Background(), s.configurator.NodeID(), &snapshot)
	return nil
}

//Run will periodically refresh the snapshot
func (s *Snapshotter) Run(a *k8s.Aggregator) {
	log.Infof("started snapshotter")
	hadChanges := false
	for event := range a.Events() {
		change := false
		switch event.SyncType {
		case k8s.COMMAND:
			if hadChanges {
				err := s.snapshot()
				if err != nil {
					logrus.Warnf("caught error in snapshot: %s", err)
					continue
				}
				hadChanges = false
				continue
			}
		case k8s.INGRESS:
			change = true
		}
		hadChanges = hadChanges || change
	}
}
