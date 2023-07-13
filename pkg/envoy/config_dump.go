package envoy

import (
	types "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

type EnvoySnapshot struct {
	Listeners map[string]types.Resource
	Clusters  map[string]types.Resource
}

func (s *Snapshotter) ConfigDump() (EnvoySnapshot, error) {
	snapshot, err := s.CurrentSnapshot()
	if err != nil {
		return EnvoySnapshot{}, err
	}

	listeners := snapshot.GetResources(resource.ListenerType)
	clusters := snapshot.GetResources(resource.ClusterType)

	return EnvoySnapshot{
		Listeners: listeners,
		Clusters:  clusters,
	}, nil
}
