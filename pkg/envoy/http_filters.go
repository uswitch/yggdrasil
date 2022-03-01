package envoy

import (
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
)

type httpFilterBuilder struct {
	filters []*hcm.HttpFilter
}

func (b *httpFilterBuilder) Add(filter *hcm.HttpFilter) *httpFilterBuilder {
	b.filters = append(b.filters, filter)
	return b
}

func (b *httpFilterBuilder) Filters() []*hcm.HttpFilter {
	b.Add(&hcm.HttpFilter{Name: "envoy.filters.http.router"})
	return b.filters
}
