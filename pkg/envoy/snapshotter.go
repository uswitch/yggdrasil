package envoy

import (
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"

	"github.com/uswitch/yggdrasil/pkg/k8s"
)

//Configurator is an interface that implements Generate and NodeID
type Configurator interface {
	Generate([]v1beta1.Ingress) cache.Snapshot
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
	ingresses, err := s.aggregator.ListIngresses()
	if err != nil {
		return err
	}

	snapshot := s.configurator.Generate(ingresses)

	log.Debugf("took snapshot: %+v", snapshot)

	s.snapshotCache.SetSnapshot(s.configurator.NodeID(), snapshot)
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
				s.snapshot()
				hadChanges = false
				continue
			}
		case k8s.INGRESS:
			change = true
		}
		hadChanges = hadChanges || change
	}
}
