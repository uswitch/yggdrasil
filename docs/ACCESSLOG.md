# Access Log

The Access log format is configurable via the Yggdrasil config file only. It is defined as a json object as follows:

```json
{
    "accessLogger": {
        "format": {
            "bytes_received":            "%BYTES_RECEIVED%",
            "bytes_sent":                "%BYTES_SENT%",
            "downstream_local_address":  "%DOWNSTREAM_LOCAL_ADDRESS%",
            "downstream_remote_address": "%DOWNSTREAM_REMOTE_ADDRESS%",
            "duration":                  "%DURATION%",
            "forwarded_for":             "%REQ(X-FORWARDED-FOR)%",
            "protocol":                  "%PROTOCOL%",
            "request_id":                "%REQ(X-REQUEST-ID)%",
            "request_method":            "%REQ(:METHOD)%",
            "request_path":              "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
            "response_code":             "%RESPONSE_CODE%",
            "response_flags":            "%RESPONSE_FLAGS%",
            "start_time":                "%START_TIME(%s.%3f)%",
            "upstream_cluster":          "%UPSTREAM_CLUSTER%",
            "upstream_host":             "%UPSTREAM_HOST%",
            "upstream_local_address":    "%UPSTREAM_LOCAL_ADDRESS%",
            "upstream_service_time":     "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
            "user_agent":                "%REQ(USER-AGENT)%"
        }
    }
}

```

The config above would be the same as the default access logger config shipped with Yggdasil. Thus if no format is provided this will be the format used.

[See Envoy docs for more on access log formats](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#config-access-log-default-format)

The access log is written to `/var/log/envoy/access.log` which is not currently configurable.
