package envoy

import (
	"fmt"

	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	types "github.com/golang/protobuf/ptypes"
)

type httpFilterBuilder struct {
	filters []*hcm.HttpFilter
}

func (b *httpFilterBuilder) Add(filter *hcm.HttpFilter) *httpFilterBuilder {
	b.filters = append(b.filters, filter)
	return b
}

func (b *httpFilterBuilder) Filters() ([]*hcm.HttpFilter, error) {
	router := &router.Router{}

	anyRouter, err := types.MarshalAny(router)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal router config struct to typed struct: %s", err)
	}
	b.Add(&hcm.HttpFilter{
		Name:       "envoy.filters.http.router",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: anyRouter},
	})
	return b.filters, nil
}
