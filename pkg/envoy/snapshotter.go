package envoy

import (
	"context"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/uswitch/yggdrasil/pkg/k8s"
)

// Configurator is an interface that implements Generate and NodeID
type Configurator interface {
	Generate([]*k8s.Ingress, []*v1.Secret) (cache.Snapshot, error)
	NodeID() string
}

// Snapshotter watches for Ingress changes and updates the
// config snapshot
type Snapshotter struct {
	snapshotCache cache.SnapshotCache
	configurator  Configurator
	aggregator    *k8s.Aggregator
}

// NewSnapshotter returns a new Snapshotter
func NewSnapshotter(snapshotCache cache.SnapshotCache, config Configurator, aggregator *k8s.Aggregator) *Snapshotter {
	return &Snapshotter{snapshotCache: snapshotCache, configurator: config, aggregator: aggregator}
}

func (s *Snapshotter) snapshot() error {
	genericIngresses, err := s.aggregator.GetGenericIngresses()
	if err != nil {
		return err
	}
	secrets, err := s.aggregator.GetSecrets()
	if err != nil {
		return err
	}

	snapshot, err := s.configurator.Generate(genericIngresses, secrets)

	log.Debugf("took snapshot: %+v", snapshot)

	s.snapshotCache.SetSnapshot(context.Background(), s.configurator.NodeID(), &snapshot)

	return nil
}

func (s *Snapshotter) CurrentSnapshot() (cache.ResourceSnapshot, error) {
	return s.snapshotCache.GetSnapshot(s.configurator.NodeID())
}

// Run will periodically refresh the snapshot
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
					logrus.Errorf("caught error in snapshot: %s", err)
					continue
				}
				hadChanges = false
				continue
			}
		case k8s.INGRESS:
			change = true
		case k8s.SECRET:
			change = true
		}
		hadChanges = hadChanges || change
	}
}
