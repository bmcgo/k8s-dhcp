# K8S-DHCP

Kubernetes native dhcp server.

The server can be configured by creating and editing kubernetes objects `dhcpserver`, `dhcpsubnet` and optionally
`dhcphost`.

## Server instances

To start listening, at least one `dhcpserver` object must be created:
```yaml
apiVersion: dhcp.bmcgo.dev/v1alpha1
kind: DHCPServer
metadata:
  name: dhcpserver-sample-1
spec:
  listenInterface: enp0s3
  listenAddress: 10.0.1.1
```

* `listenInterface` Server will listen on all interfaces if this field is empty.
* `listenAddress` Server will listen at `0.0.0.0` if empty.

## Subnets
Each subnet is represented by `dhcpsubnet` object:

```yaml
apiVersion: dhcp.bmcgo.dev/v1alpha1
kind: DHCPSubnet
metadata:
  name: dhcpsubnet-sample-broadcast
spec:
  subnet: 10.0.1.0/24
  rangeFrom: 10.0.1.100
  rangeTo: 10.0.1.200
  gateway: 10.0.1.254
  bootFileName: http://1.2.3.4/undionly.kpxe
  leaseTime: 3600
  dns:
    - 1.1.1.1
    - 8.8.8.8
  options:
  - id: 66
    type: string
    value: 10.12.0.1
```

* `subnet` subnet address. Required.
* `rangeFrom` Required.
* `rangeTo` Required.
* `gateway` Required.
* `bootFileName` Optional.
* `leaseTime` Required.
* `dns` Optional.
* `options` list of dhcp options to be included in response. Optional.

Each server instance may serve multiple subnets. Server will automatically detect proper subnet for each
request, and will construct dhcp response according to `dhcpsubnet` settings.

Requests on unknown subnets will be ignored.

## Static Hosts

Per host configuration may be applied if needed by creating `dhcphost` objects:

```yaml
apiVersion: dhcp.bmcgo.dev/v1alpha1
kind: DHCPHost
metadata:
  name: host-sample-1
spec:
  subnet: 10.0.1.0/24
  mac: "00:01:02:03:04:05"
  ip: 10.0.1.20
  gateway: 10.0.1.3
  hostname: sample-pxe-node
  dns:
  - 1.1.1.1
  - 8.8.4.4
  options:
  - id: 66
    type: string
    value: 1.1.2.2
  serverHostName: example.net
  bootFileName: http://10.1.2.3/alternate.ipxe
  leaseTime: 3600
```

* `subnet` is a reference to subnet. Required.
* `mac` client hardware address. Required.
* `ip` client fixed ip address. may be outside of range but must be inside of subnet. Will be taken from pool if empty.
* `gateway` Optional.
* `hostname` Optional.
* `dns` Optional.
* `options` Optional.
* `serverHostName` Optional.
* `bootFileName` Optional.
* `leaseTime` Optional.

Server start listening and logging dhcp requests when at least one `dhcpserver` is created, and start responding
when at least one `dhcpsubnet` is created.

```
$ kubectl get dhcpservers
NAME                   INTERFACE   LISTEN
dhcpserver-sample-br1  br1
dhcpserver-veth 1      veth1       10.7.0.1
```

```
$ kubectl get dhcpsubnets
NAME                          SUBNET         FROM          TO              GATEWAY
dhcpsubnet-sample-br1         10.10.0.0/16   10.10.1.100   10.10.255.200   10.10.0.1
dhcpsubnet-sample-veth1       10.11.0.0/16   10.11.1.1     10.11.255.250   10.11.255.254
dhcpsubnet-test-relay         10.7.0.0/16    10.7.1.100    10.7.255.200    10.7.0.1

```

Leases are stored in `dhcpsubnet` status:

```
$ kubectl get dhcpsubnet dhcpsubnet-sample-br1 -o yaml
...
status:
  errorMessage: ""
  leases:
    52:54:10:00:1c:01:
      ip: 10.7.255.100
      updatedAt: "2022-08-27T10:27:26Z"
    52:54:10:00:1c:02:
      ip: 10.7.255.101
      updatedAt: "2022-08-27T10:27:28Z"
    52:54:10:00:1c:03:
      ip: 10.7.255.102
      updatedAt: "2022-08-27T10:27:29Z"
```
## TODO:

* detect start of another server;
* load all subnets, leases and hosts before starting the server;
* configure namespace;
* log server version;
* add ping check option;
* handle subnet update;
* handle hostnames;
* support dhcp NAK;
* support dhcp INFORM;
* conditional options;
* respect requested options;
* add ReuseAddr property to server/listen;
* exit if failed to bind;
* dhcp option 43 (vendor-option-space);

# K8S README

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [Minikube](https://github.com/kubernetes/minikube) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/k8s-dhcp:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/k8s-dhcp:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

