Start by creating and ssh'ing to the Vagrant VM
```
vagrant up
vagrant ssh
```

Build yggdrasil and it's docker image, in the repo root mounted at /yggdrasil:
```
sudo su
cd /yggdrasil
docker-compose -f develop-docker-compose.yml up -d
docker exec -it yggdrasil-app bash
go get
go mod tidy
make
exit
docker build . -t local-yggdrasil:latest
```

Go to this examples directory (mounted at /vagrant) and configure your k3d clusters

```
cd /vagrant
k3d cluster create cluster1 --k3s-arg "--disable=traefik@server:0" --k3s-arg "--disable=servicelb@server:0" --k3s-arg "--cluster-cidr=10.118.0.0/17@server:*" --k3s-arg "--service-cidr=10.118.128.0/17@server:*"
k3d cluster create cluster2 --k3s-arg "--disable=traefik@server:0" --k3s-arg "--disable=servicelb@server:0" --k3s-arg "--cluster-cidr=10.119.0.0/17@server:*" --k3s-arg "--service-cidr=10.119.128.0/17@server:*"

for cluster_name in $(docker network list --format "{{ .Name}}" | grep k3d); do

kubectl config use-context $cluster_name

kubectl apply -f kube-manifests/metallb.yml

# configure metallb ingress address range
cidr_block=$(docker network inspect $cluster_name | jq '.[0].IPAM.Config[0].Subnet' | tr -d '"')
cidr_base_addr=${cidr_block%???}
ingress_first_addr=$(echo $cidr_base_addr | awk -F'.' '{print $1,$2,255,0}' OFS='.')
ingress_last_addr=$(echo $cidr_base_addr | awk -F'.' '{print $1,$2,255,255}' OFS='.')
ingress_range=$ingress_first_addr-$ingress_last_addr
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - $ingress_range
EOF

kubectl config view --minify --raw --output "jsonpath={.clusters.name==\"$cluster_name\"}{..cluster.certificate-authority-data}" | base64 -d > yggdrasil/$cluster_name-ca.crt
kubectl apply -f kube-manifests/yggdrasil.yml
kubectl get secrets -o jsonpath="{.items[?(@.metadata.annotations['kubernetes\.io/service-account\.name']=='yggdrasil-sa')].data.token}"|base64 --decode > yggdrasil/$cluster_name-token

kubectl apply -f kube-manifests/nginx-ingress-controller.yml
done
```

Once the nginx ingress is up and running (might take a couple of minutes) you can deploy the example app+ingress
```
for cluster_name in $(docker network list --format "{{ .Name}}" | grep k3d); do
kubectl config use-context $cluster_name
kubectl apply -f kube-manifests/example-ingress.yml
kubectl apply -f kube-manifests/example-$cluster_name.yml
done
```

Run yggdrasil and envoy from the docker-compose.yml
```
docker-compose up -d
```

Once yggdrasil and enovy are up and running, we can test the different paths and domains
```
curl -H host:example.com http://localhost:10000
curl -H host:example.net http://localhost:10000/example
curl -H host:cluster1.example.org http://localhost:10000/cluster1
curl -H host:cluster2.example.org http://localhost:10000/cluster2
```

If everything is working correctly, you should see the requests come from pods in different clusters

You can access envoy admin interface from http://localhost:9901 to dump configuration
