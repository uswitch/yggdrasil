# Yggdrasil

Yggdrasil is an Envoy control plane that configures listeners and clusters based off Kubernetes ingresses from multiple Kube Clusters. This allows you to have an envoy cluster acting as a mutli-cluster loadbalancer for Kubernetes. This was something we needed as we wanted our apps to be highly available in the event of a cluster outage but did not want the solution to live inside of Kubernetes itself.

## Usage
Yggdrasil will watch all Ingresses in each Kubernetes Cluster that you give it via the Kubeconfig flag. Any ingresses that match the ingress class that you have specified will have a listener and cluster created that listens on the same Host as the Host defined in the Ingress object. If you have multiple clusters Yggdrasil will create a cluster address for each Kubernetes cluster your Ingress is in, the address is the address of the ingress loadbalancer.

## Setup
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
    api_config_source:
      api_type: GRPC
      cluster_names: [xds_cluster]
  cds_config:
    api_config_source:
      api_type: GRPC
      cluster_names: [xds_cluster]

static_resources:
  clusters:
  - name: xds_cluster
    connect_timeout: 0.25s
    type: STATIC
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    hosts: [{ socket_address: { address: yggdrasil, port_value: 8080 }}]
```

Your ingress set up then looks like this:

![Envoy Diagram](/img/envoysetup.png)

Where the envoy nodes are loadbalancing between each cluster for a given ingress.
## Configuration
Yggdrasil can be configured using a config file e.g:
```json
{
  "nodeName": "foo",
  "ingressClass": "multi-cluster",
  "cert": "path/to/cert",
  "key": "path/to/key",
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
`nodeName` is the same `node-name` that you start your envoy nodes with.
The cert and key are used to talk over https with the ingress loadbalancers.
The `ingressClass` is the ingress class that yggdrasil will watch for.
Each cluster represents a different Kubernetes cluster with the token being a service account token for that cluster. `ca` is the Path to the ca certificate for that cluster.

## Flags
```
--cert string               certfile
--config string             config file
--debug                     Log at debug level
--help                      help for yggdrasil
--ingress-class string      Ingress class to watch
--key string                keyfile
--kube-config stringArray   Path to kube config
--node-name string          envoy node name
```
