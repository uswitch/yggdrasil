package envoy

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	eal "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
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

type accessLoggerTestCase struct {
	name   string
	format map[string]interface{}
	custom bool
}

func TestAccessLoggerConfig(t *testing.T) {
	testCases := []accessLoggerTestCase{
		{name: "default log format", format: DefaultAccessLogFormat, custom: false},
		{name: "custom log format", format: map[string]interface{}{"a-key": "a-format-specifier"}, custom: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := AccessLogger{}
			if tc.custom {
				cfg.Format = tc.format
			}

			fileAccessLog := makeFileAccessLog(cfg, "/var/log/envoy/")
			if fileAccessLog.Path != "/var/log/envoy/access.log" {
				t.Errorf("Expected access log to use default path but was, %s", fileAccessLog.Path)
			}

			alf, ok := fileAccessLog.AccessLogFormat.(*eal.FileAccessLog_LogFormat)
			if !ok {
				t.Fatalf("File Access Log Format had incorrect type, should be FileAccessLog_LogFormat")
			}

			lf, ok := alf.LogFormat.Format.(*core.SubstitutionFormatString_JsonFormat)
			if !ok {
				t.Fatalf("LogFormat had incorrect type, should be SubstitutionFormatString_JsonFormat")
			}

			format := lf.JsonFormat.AsMap()
			if !reflect.DeepEqual(format, tc.format) {
				t.Errorf("Log format map should match configuration")
			}
		})
	}
}

func mustParseDuration(dur string) time.Duration {
	d, err := time.ParseDuration(dur)
	if err != nil {
		panic(fmt.Sprintf("Failed test setup: %s", err))
	}
	return d
}
