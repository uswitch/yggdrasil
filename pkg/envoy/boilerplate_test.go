package envoy

import (
	"fmt"
	"testing"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/golang/protobuf/ptypes/duration"
)

func TestMakeHealthChecksEmptyPath(t *testing.T) {
	healthChecks := makeHealthChecks("example.com", "", UpstreamHealthCheck{})

	if len(healthChecks) != 0 {
		t.Error("Expected healthchecks to be empty")
	}
}

func TestMakeHealthChecksValidPath(t *testing.T) {
	host, path := "foo", "/bobba"
	cfg := UpstreamHealthCheck{
		Timeout:            mustParseDuration("5s"),
		Interval:           mustParseDuration("10s"),
		UnhealthyThreshold: 3,
		HealthyThreshold:   3,
	}
	healthChecks := makeHealthChecks(host, path, cfg)
	timeout := healthChecks[0].Timeout
	interval := healthChecks[0].Interval

	cfgTimeout := &duration.Duration{Seconds: int64(cfg.Timeout.Seconds())}
	cfgInterval := &duration.Duration{Seconds: int64(cfg.Interval.Seconds())}

	if len(healthChecks) != 1 {
		t.Error("Expected healthcheck to exist")
	}

	if cfgTimeout.Seconds != timeout.Seconds {
		t.Errorf("Expected timeout to be %s, but got %s", cfgTimeout, timeout)
	}

	if cfgInterval.Seconds != interval.Seconds {
		t.Errorf("Expected interval to be %s, but got %s", cfgInterval, interval)
	}

	httpCheck := healthChecks[0].HealthChecker.(*core.HealthCheck_HttpHealthCheck_)

	if httpCheck.HttpHealthCheck.Host != host {
		t.Errorf("Expect health check host to be %s, but got %s", host, httpCheck.HttpHealthCheck.Host)
	}

	if httpCheck.HttpHealthCheck.Path != path {
		t.Errorf("Expect health check path to be %s, but got %s", path, httpCheck.HttpHealthCheck.Path)
	}

}

func mustParseDuration(dur string) time.Duration {
	d, err := time.ParseDuration(dur)
	if err != nil {
		panic(fmt.Sprintf("Failed test setup: %s", err))
	}
	return d
}
