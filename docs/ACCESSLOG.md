# Access Log

The Access log format is configurable via the Yggdrasil config file only. It is defined as a json object as follows:

```json
{
    "accessLogger": {
        "format": {
            "start_time":                "%START_TIME(%s.%3f)%",
            "bytes_received":            "%BYTES_RECEIVED%",
            "protocol":                  "%PROTOCOL%",
            "response_code":             "%RESPONSE_CODE%",
            "bytes_sent":                "%BYTES_SENT%",
            "duration":                  "%DURATION%",
            "response_flags":            "%RESPONSE_FLAGS%",
            "upstream_host":             "%UPSTREAM_HOST%",
            "upstream_cluster":          "%UPSTREAM_CLUSTER%",
            "upstream_local_address":    "%UPSTREAM_LOCAL_ADDRESS%",
            "downstream_remote_address": "%DOWNSTREAM_REMOTE_ADDRESS%",
            "downstream_local_address":  "%DOWNSTREAM_LOCAL_ADDRESS%",
            "request_method":            "%REQ(:METHOD)%",
            "request_path":              "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
            "upstream_service_time":     "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
            "forwarded_for":             "%REQ(X-FORWARDED-FOR)%",
            "user_agent":                "%REQ(USER-AGENT)%",
            "request_id":                "%REQ(X-REQUEST-ID)%"
        }
    }
}

```

The config above would be the same as the default access logger config shipped with Yggdasil. Thus if no format is provided this will be the format used.

The access log is written to `/var/log/envoy/access.log` which is not currently configurable.
