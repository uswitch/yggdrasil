package envoy

import (
	"fmt"
	"testing"
	"time"
)

func TestMakeHealthChecksEmptyPath(t *testing.T) {
	healthChecks := makeHealthChecks("", UpstreamHealthCheck{})

	if len(healthChecks) != 0 {
		t.Error("Expected healthchecks to be empty")
	}
}

func TestMakeHealthChecksValidPath(t *testing.T) {
	cfg := UpstreamHealthCheck{
		Timeout:            mustParseDuration("5s"),
		Interval:           mustParseDuration("10s"),
		UnhealthyThreshold: 3,
		HealthyThreshold:   3,
	}
	healthChecks := makeHealthChecks("/bobba", cfg)
	timeout := healthChecks[0].Timeout
	interval := healthChecks[0].Interval

	if len(healthChecks) != 1 {
		t.Error("Expected healthcheck to exist")
	}

	if cfg.Timeout != *timeout {
		t.Errorf("Expected timeout to be %s, but got %s", cfg.Timeout, timeout)
	}

	if cfg.Interval != *interval {
		t.Errorf("Expected interval to be %s, but got %s", cfg.Interval, interval)
	}
}

func mustParseDuration(dur string) time.Duration {
	d, err := time.ParseDuration(dur)
	if err != nil {
		panic(fmt.Sprintf("Failed test setup: %s", err))
	}
	return d
}
