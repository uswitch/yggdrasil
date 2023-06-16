# Getting Started
In this example, we will setup a basic HTTP envoy load balancer that will receive its config from Yggdrasil via gRPC. To do this, we will configure two docker containers; one container running an envoy node and the other running Yggdrasil. This example assumes that you have a working Kubernetes cluster, so Yggdrasil can communicate with the Kubernetes API.

`Note:` This specific example is running on GCP, but the steps are cloud-agnostic and there is no reason why this wouldn't also work with a local docker daemon and Kube cluster (e.g, minikube).

## Configure Kubernetes
For this example to work, we will need to have a service running in Kube with a valid corresponding ingress resource. In this example, we will use an nginx ingress controller.

`Note:` If deploying an ingress controller using [Helm on GCP](https://github.com/GoogleCloudPlatform/community/blob/master/tutorials/nginx-ingress-gke/index.md#deploy-nginx-ingress-controller-with-rbac-enabled), it will likely be necessary for the `--set controller.publishService.enabled=true` flag to be set, so that the created ingress uses the ingress controller's IP address/hostname. The ingress IP address should match the ingress controller's, as this is the IP address that Yggdrasil will use to generate config for envoy.

Assuming we have a simple HTTP web service called 'hello-world', we can apply the following 'hello-world' ingress resource:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: hello-world
  namespace: default
  annotations:
    kubernetes.io/ingress.class: nginx
    yggdrasil.uswitch.com/healthcheck-path: /healthz
    yggdrasil.uswitch.com/timeout: 30s
spec:
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          serviceName: hello-world
          servicePort: 80
```

Once the resource has been created, we should see the ingress controller's IP address or hostname when fetching the ingress:

```console
$ kubectl get ingress hello-world
NAME          HOSTS              ADDRESS        PORTS   AGE
hello-world   example.com        192.168.0.10   80      1h
```

Double check that this matches the ingress controller's external address:

```console
$ kubectl get svc nginx-ingress-controller
NAME                       TYPE           CLUSTER-IP   EXTERNAL-IP    PORT(S)                      AGE
nginx-ingress-controller   LoadBalancer   10.10.10.10  192.168.0.10   80:30757/TCP,443:31061/TCP   1h
```

We can verify that the ingress is working correctly by cURLing the ingress controller's IP address:

```console
$ curl -H Host:example.com http://192.168.0.10
Hello world!
```

## Configure Yggdrasil
With our ingress working correctly, we can now setup Yggdrasil. Pull the following docker image (this version added support for IP address ingresses as seen in GCP)

```console
$ docker pull quay.io/uswitch/yggdrasil:v0.11.0
```

Next, we will setup a config file for Yggdrasil so we can retrieve ingress details from our Kubernetes cluster's API. Consider the following Yggdrasil config:

```yaml
{
  "nodeName": "envoy-node",
  "ingressClasses": ["nginx"],
  "clusters": [
    {
      "token": "kubeApiToken",
      "apiServer": "https://kube.api.server:<port>",
      "ca": "ca.crt"
    }
  ]
}
```

Where:
* `nodeName` is the name we will give our envoy node(s)
* `ingressClasses` is a list of the ingress classes that Yggdrasil will look for
* `clusters` is a list of Kubernetes clusters, where:
  * `token` is the Kube token of a service account that is able to get ingress resources
  * `apiServer` is the address of the Kube api server
  * `ca` is the Kube API CA certificate

#### Kubernetes service account for Yggdrasil

`Note:` Once the service account has been created, you will need to retrieve its token from the corresponding Kube secret for use by Yggdrasil in the `token` field mentioned above. Please see the [Kubernetes docs](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#service-account-tokens) for greater detail on creating and using service account tokens.

We can create a service account specifically for Yggdrasil that is able to list, watch, and get ingress resources with the below ClusterRole and matching ClusterRoleBinding.

Create the service account:
```console
$ kubectl create serviceaccount yggdrasil-sa
```

Apply the following manifest for the ClusterRole:
```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: '*'
  name: yggdrasil-read-only
rules:
- apiGroups: ["extensions"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch"]
```

And apply the following ClusterRoleBinding:
```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: yggdrasil-sa-binding
subjects:
- kind: ServiceAccount
  name: yggdrasil-sa
  namespace: default
roleRef:
  kind: ClusterRole
  name: yggdrasil-read-only
  apiGroup: rbac.authorization.k8s.io
```

The Yggdrasil docker container can now be started - make sure to mount the config file you have created, as well as the Kube API CA cert:

```console
$ docker run -d -v /path/to/config.yaml:/config.yaml -v /path/to/ca.crt:/ca.crt quay.io/uswitch/yggdrasil:v0.11.0 --config=config.yaml --debug --upstream-port=80
```

By default, Yggdrasil will use an upstream ingress port of 443 (HTTPS), as we are just running an HTTP ingress we will use the `--upstream-port=80` flag as seen above.

`Note:` For more information on Yggdrasil's flags, please see [here](/README.md#Flags).

## Configure envoy
With the Yggdrasil container running, we can now configure an envoy node. Pull an envoy v1.10 docker image with the following command:

```console
$ docker pull envoyproxy/envoy:v1.26-latest
```

Next, we will need to setup a minimal config file to create the admin listener for envoy, as well as pointing to our dynamic configuration provider - Yggdrasil:

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
                address: <yggdrasil-container-ip-address>
                port_value: 8080
```

Where `<yggdrasil-container-ip-address>` is the IP address of the Yggdrasil docker container. Save the file as `envoy.yaml`.

Run the envoy docker container with the following command, making sure to mount the minimal config file that you've created:

```console
$ docker run -e ENVOY_UID=0 -w /var/log/envoy/ -v /path/to/envoy.yaml:/etc/envoy/envoy.yaml -p 10000:10000 -d envoyproxy/envoy:v1.26-latest --service-node envoy-node --service-cluster envoy-node --config-path /etc/envoy/envoy.yaml
```

The working directory for the container is set to `/var/log/envoy/` in order to create it at runtime, as Yggdrasil will configure envoy to write access logs to this directory.

We also forward port 10000 of the container to port 10000 of the docker host with the above command, so we can easily cURL the host and verify that envoy is load balancing correctly. You can forward the admin listener port `9901` as well in order to access envoy's admin web UI from the docker host, but this is not essential for the example to work.

Envoy will take a short while to start and retrieve its config, once this is complete we can cURL `localhost:10000` and check that we can reach our web service:

```console
$ curl -H Host:example.com http://localhost:10000
Hello world!
```

If you are unable to reach the web service, check envoy's logs and make sure that it has finished starting up. If envoy has started successfully, you should see something similar to the below in the logs:
```console
$ docker logs -f envoy_container_id
...
[2019-09-02 09:56:06.207][1][info][main] [source/server/server.cc:462] all clusters initialized. initializing init manager
[2019-09-02 09:56:06.212][1][info][upstream] [source/server/lds_api.cc:74] lds: add/update listener 'listener_0'
[2019-09-02 09:56:06.212][1][info][config] [source/server/listener_manager_impl.cc:1006] all dependencies initialized. starting workers
```
