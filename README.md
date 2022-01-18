# Yggdrasil
Yggdrasil is an Envoy control plane that configures listeners and clusters based off Kubernetes ingresses from multiple Kube Clusters. This allows you to have an envoy cluster acting as a mutli-cluster loadbalancer for Kubernetes. This was something we needed as we wanted our apps to be highly available in the event of a cluster outage but did not want the solution to live inside of Kubernetes itself.

`Note:` Currently we support version 1.19.x of Envoy.</br>
`Note:` Yggdrasil now uses [Go modules](https://github.com/golang/go/wiki/Modules) to handle dependencies.

## Usage
Yggdrasil will watch all Ingresses in each Kubernetes Cluster that you give it via the Kubeconfig flag. Any ingresses that match any of the ingress classes that you have specified will have a listener and cluster created that listens on the same Host as the Host defined in the Ingress object. If you have multiple clusters Yggdrasil will create a cluster address for each Kubernetes cluster your Ingress is in, the address is the address of the ingress loadbalancer.

[Joseph Irving](https://github.com/Joseph-Irving) has published a [blog post](https://medium.com/uswitch-labs/multi-cluster-kubernetes-load-balancing-in-aws-with-yggdrasil-c1583ea7d78f) which describes our need for and use of Yggdrasil at Uswitch.

## Setup
Please see the [Getting Started](/docs/GETTINGSTARTED.md) guide for a walkthrough of setting up a simple HTTP service with Yggdrasil and envoy.

The basic setup is to have a cluster of envoy nodes which connect to Yggdrasil via GRPC and get given dynamic listeners and clusters from it. Yggdrasil is set up to talk to each Kubernetes api where it will watch the ingresses for any that are using the ingress class it's watching for.

![Yggdrasil Diagram](/img/yggdrasil.png)

Your envoy nodes only need a very minimal config where they are simply set up to get dynamic clusters and listeners from Yggdrasil.
Example envoy config:
```yaml
admin:
  access_log_path: /tmp/admin_access.log
  address:
    socket_address: { address: 0.0.0.0, port_value: 9901 }

dynamic_resources:
  lds_config:
    resource_api_version: V3
    api_config_source:
      transport_api_version: V3
      api_type: GRPC
      grpc_services:
      - envoy_grpc:
          cluster_name: xds_cluster
  cds_config:
    resource_api_version: V3
    api_config_source:
      transport_api_version: V3
      api_type: GRPC
      grpc_services:
      - envoy_grpc:
          cluster_name: xds_cluster

static_resources:
  clusters:
  - name: xds_cluster
    connect_timeout: 0.25s
    type: STATIC
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    load_assignment:
      cluster_name: xds_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: yggdrasil
                port_value: 8080
```

Your ingress set up then looks like this:

![Envoy Diagram](/img/envoysetup.png)

Where the envoy nodes are loadbalancing between each cluster for a given ingress.

### Health Check
Yggdrasil always configures a path on your Envoy nodes at `/yggdrasil/status`, this can be used to health check your envoy nodes, it will only return 200 if your nodes have started and been configured by Yggdrasil.

## Annotations
Yggdrasil allows for some customisation of the route and cluster config per Ingress through the annotations below.

| Name                                                         | type     |
|--------------------------------------------------------------|----------|
| [yggdrasil.uswitch.com/healthcheck-path](#health-check-path) | string   |
| [yggdrasil.uswitch.com/timeout](#timeout)                    | duration |

### Health Check Path
Specifies a path to configure a [HTTP health check](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/config/core/v3/health_check.proto#config-core-v3-healthcheck-httphealthcheck) to. Envoy will not route to clusters that fail health checks.

* [config.core.v3.HealthCheck.HttpHealthCheck.Path](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/config/core/v3/health_check.proto#envoy-v3-api-field-config-core-v3-healthcheck-httphealthcheck-path)

### Timeout
Allows for adjusting the timeout in envoy. Currently this will set the following timeouts to this value:

* [config.route.v3.RouteAction.Timeout](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/config/route/v3/route_components.proto#envoy-v3-api-field-config-route-v3-routeaction-timeout)
* [config.route.v3.RetryPolicy.PerTryTimeout](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/config/route/v3/route_components.proto#envoy-v3-api-field-config-route-v3-retrypolicy-per-try-timeout)
* [config.cluster.v3.Cluster.ConnectTimeout](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/config/cluster/v3/cluster.proto#envoy-v3-api-field-config-cluster-v3-cluster-connect-timeout)

### Example
Below is an example of an ingress with some of the annotations specified

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: example-com
  namespace: default
  annotations:
    yggdrasil.uswitch.com/healthcheck-path: /healthz
    yggdrasil.uswitch.com/timeout: 30s
spec:
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          serviceName: example
          servicePort: 80
```

## Configuration
Yggdrasil can be configured using a config file e.g:
```json
{
  "nodeName": "foo",
  "ingressClasses": ["multi-cluster", "multi-cluster-staging"],
  "certificates": [
    {
      "hosts": ["*.api.com"],
      "cert": "path/to/cert",
      "key": "path/to/key"
    }
  ],
  "clusters": [
    {
      "token": "xxxxxxxxxxxxxxxx",
      "apiServer": "https://cluster1.api.com",
      "ca": "pathto/cluster1/ca"
    },
    {
      "tokenPath": "/path/to/a/token",
      "apiServer": "https://cluster2.api.com",
      "ca": "pathto/cluster2/ca"
    }
  ]
}
```

The list of certificates will be loaded by Yggdrasil and served to the Envoy nodes by inlining the key pairs. These will then be used to group the ingress into different filter chains, split using hosts.

`nodeName` is the same `node-name` that you start your envoy nodes with.
The `ingressClasses` is a list of ingress classes that yggdrasil will watch for.
Each cluster represents a different Kubernetes cluster with the token being a service account token for that cluster. `ca` is the Path to the ca certificate for that cluster.

## Metrics
Yggdrasil has a number of Go, gRPC, Prometheus, and Yggdrasil-specific metrics built in which can be reached by cURLing the `/metrics` path at the health API address/port (default: 8081). See [Flags](#Flags) for more information on configuring the health API address/port.

The Yggdrasil-specific metrics which are available from the API are:

| Name                        | Description                                    | Type     |
|-----------------------------|------------------------------------------------|----------|
| yggdrasil_cluster_updates   | Number of times the clusters have been updated | counter  |
| yggdrasil_clusters          | Total number of clusters generated             | gauge    |
| yggdrasil_ingresses         | Total number of matching ingress objects       | gauge    |
| yggdrasil_listener_updates  | Number of times the listener has been updated  | counter  |
| yggdrasil_virtual_hosts     | Total number of virtual hosts generated        | gauge    |

## Flags
```
--address string                           yggdrasil envoy control plane listen address (default "0.0.0.0:8080")
--ca string                                trustedCA
--cert string                              certfile
--config string                            config file
--debug                                    Log at debug level
--envoy-listener-ipv4-address string       IPv4 address by the envoy proxy to accept incoming connections (default 0.0.0.0)
--envoy-port uint32                        port by the envoy proxy to accept incoming connections (default 10000)
--health-address string                    yggdrasil health API listen address (default "0.0.0.0:8081")
-h, --help                                 help for yggdrasil
--host-selection-retry-attempts int        Number of host selection retry attempts. Set to value >=0 to enable (default -1)
--ingress-classes strings                  Ingress classes to watch
--key string                               keyfile
--kube-config stringArray                  Path to kube config
--max-ejection-percentage int32            maximal percentage of hosts ejected via outlier detection. Set to >=0 to activate outlier detection in envoy. (default -1)
--node-name string                         envoy node name
--upstream-healthcheck-healthy uint32      number of successful healthchecks before the backend is considered healthy (default 3)
--upstream-healthcheck-interval duration   duration of the upstream health check interval (default 10s)
--upstream-healthcheck-timeout duration    timeout of the upstream healthchecks (default 5s)
--upstream-healthcheck-unhealthy uint32    number of failed healthchecks before the backend is considered unhealthy (default 3)
--upstream-port uint32                     port used to connect to the upstream ingresses (default 443)
--use-remote-address                       populates the X-Forwarded-For header with the client address. Set to true when used as edge proxy - [documentation](https://www.envoyproxy.io/docs/envoy/v1.19.0/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-field-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-use-remote-address) (default false)
```
