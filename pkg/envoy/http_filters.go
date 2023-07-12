package envoy

import (
	"fmt"

	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

type httpFilterBuilder struct {
	filters []*hcm.HttpFilter
}

func (b *httpFilterBuilder) Add(filter *hcm.HttpFilter) *httpFilterBuilder {
	b.filters = append(b.filters, filter)
	return b
}

func (b *httpFilterBuilder) Filters() ([]*hcm.HttpFilter, error) {
	httpFilterConfig, err := anypb.New(&router.Router{})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal router config struct to typed struct: %s", err)
	}

	b.Add(&hcm.HttpFilter{
		Name:       "envoy.filters.http.router",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: httpFilterConfig},
	})
	return b.filters, nil
}
