package envoy

import (
	"testing"
	"time"
)

func TestMakeHealthChecksEmptyPath(t *testing.T) {
	healthChecks := makeHealthChecks("")

	if len(healthChecks) != 0 {
		t.Error("Expected healthchecks to be empty")
	}
}

func TestMakeHealthChecksValidPath(t *testing.T) {
	healthChecks := makeHealthChecks("/bobba")
	timeout := healthChecks[0].Timeout
	interval := healthChecks[0].Interval

	if len(healthChecks) != 1 {
		t.Error("Expected healthcheck to exist")
	}

	expectedTimeout, _ := time.ParseDuration("5s")
	if expectedTimeout != *timeout {
		t.Errorf("Expected timeout to be %s, but got %s", expectedTimeout, timeout)
	}

	expectedInterval, _ := time.ParseDuration("10s")
	if expectedInterval != *interval {
		t.Errorf("Expected interval to be %s, but got %s", expectedInterval, interval)
	}
}
