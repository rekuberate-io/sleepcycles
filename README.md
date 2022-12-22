<p align="center">
  <img width="256" height="250" src="https://github.com/rekuberate-io/sleepcycles/blob/main/docs/images/rekuberate-sleepcycle-logo.png?raw=true">
</p>

Define sleep & wake up cycles for your Kubernetes resources. Automatically schedule to shutdown Deployments, CronJobs, StatefulSets and HorizontalPodAutoscalers that occupy resources in your cluster and wake them up only when you need them, reducing that way the overall power consumption.
## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) or [K3D](https://k3d.io) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

Under `config/samples` you will find an example manifest that you can use to test this controller:

```yaml
apiVersion: core.rekuberate.io/v1alpha1
kind: SleepCycle
metadata:
  name: sleepcycle-sample
spec:
  shutdown: "*/3 * * * *"
  wakeup: "*/4 * * * *"
  enabled: true
```

You need to provide to every `SleepCycle` the `shutdown` (mandatory) and `wakeup` (non-mandatory) policies via Cron expressions.
`SleepCycle` is a Namespaced Custom Resource, and the controller will monitor all the resources in the Namespace you installed the
`SleepCycle` manifest and they are marked with a `Label` that has as key `rekuberate.io/sleepcycle:` and as value the `name` of the manifest you created:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: nginx-demo
    rekuberate.io/sleepcycle: sleepcycle-sample
  name: nginx-demo
  namespace: default
spec:
  ...
  ...
```

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:
	
```sh
make docker-build docker-push IMG=<some-registry>/sleepcycles:tag
```
	
3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/sleepcycles:tag
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
Please refer to our [Contributing Guidelines](CONTRIBUTING.md)

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

![debugging the controller](docs/images/SCR-20221222-hij.png)

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make generate
make manifests
```

then install the CRDs in the cluster with:

```sh
make install
```

**NOTE:** You can debug the controller in the IDE of your choice by hooking to the main.go or you can start
the controller without debugging with:

```sh
make run
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

