# Yggdrasil

Yggdrasil is an Envoy control plane that configures listeners and clusters based off Kubernetes ingresses from multiple Kube Clusters. This allows you to have an envoy cluster acting as a mutli-cluster loadbalancer for Kubernetes.

## Usage
Yggdrasil will watch all Ingresses in each Kubernetes Cluster that you give it via the Kubeconfig flag. Any ingresses that match the ingress class that you have specified will have a listener and cluster created that listens on the same Host as the Host defined in the Ingress object. If you have multiple clusters Yggdrasil will create a cluster address for each Kubernetes cluster your Ingress is in, the address is the address of the ingress loadbalancer. 

## Flags
```
--help                         Show context-sensitive help (also try --help-long and --help-man).
--kubeconfig=KUBECONFIG ...    Path to kubeconfig.
--ingress-class=INGRESS-CLASS  Ingress class to watch
--node-name=NODE-NAME          Envoy node name
--debug                        Log at debug level
```
