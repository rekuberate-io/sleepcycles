![rekuberate-sleepcycle-banner.png](docs/images/rekuberate-sleepcycle-banner.png)

Define sleep & wake up cycles for your Kubernetes resources. Automatically schedule to shutdown **Deployments**, **CronJobs**, 
**StatefulSets** and **HorizontalPodAutoscalers** that occupy resources in your cluster and wake them up **only** when you need them; 
in that way you can: 

- _schedule_ resource-hungry workloads (migrations, synchronizations, replications) in hours that do not impact your daily business
- _depressurize_ your cluster
- _decrease_ your costs
- _reduce_ your power consumption
- _lower_ you carbon footprint

Watch it here in action:

[![alt text](https://img.youtube.com/vi/hxaunYJAjms/0.jpg)](https://www.youtube.com/watch?v=hxaunYJAjms)

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) or [K3D](https://k3d.io) to get a local cluster for testing, 
or run against a remote cluster. 

> [!CAUTION]
> Minimum required Kubernetes version is **1.25** 

### Samples

Under `config/samples` you will find a set manifests that you can use to test this sleepcycles on your cluster:

#### SleepCycles

* _core_v1alpha1_sleepcycle_app_x.yaml_, manifests to deploy 2 `SleepCycle` resources in namespaces `app-1` and `app-2`

```yaml
apiVersion: core.rekuberate.io/v1alpha1
kind: SleepCycle
metadata:
  name: sleepcycle-app-1
  namespace: app-1
spec:
  shutdown: "1/2 * * * *"
  shutdownTimeZone: "Europe/Athens"
  wakeup: "*/2 * * * *"
  wakeupTimeZone: "Europe/Dublin"
  enabled: true
```

> [!NOTE]
> The cron expressions of the samples are tailored so you perform a quick demo. The `shutdown` expression schedules
> the deployment to scale down on _odd_ minutes and the `wakeup` schedule to scale up on _even_ minutes.

Every `SleepCycle` has the following **mandatory** properties:

- `shutdown`: cron expression for your shutdown schedule
- `enabled`: whether this sleepcycle policy is enabled

and the following **non-mandatory** properties:

- `shutdownTimeZone`: the timezone for your shutdown schedule, defaults to `UTC`
- `wakeup`: cron expression for your wake-up schedule
- `wakeupTimeZone`: the timezone for your wake-up schedule, defaults to `UTC`
- `successfulJobsHistoryLimit`: how many _completed_ CronJob Runner Pods to retain for debugging reasons, defaults to `1`
- `failedJobsHistoryLimit`: how many _failed_ CronJob Runner Pods to retain for debugging reasons, defaults to `1`
- `runnerImage`: the image to use when spawn CronJob Runner pods, defaults to `akyriako78/rekuberate-io-sleepcycles-runners`

> [!IMPORTANT]
> DO **NOT** ADD **seconds** or **timezone** information to you cron expressions.

#### Demo workloads

* _whoami-app-1_x-deployment.yaml_, manifests to deploy 2 `Deployment` that provisions _traefik/whoami_ in namespace `app-1` 
* _whoami-app-2_x-deployment.yaml_, manifests to deploy a `Deployment`that provisions _traefik/whoami_ in namespace `app-2`
* _apache-hpa.yaml_, manifest to deploy an `HorizontalPodAutoscaler` for a PHP application in namespace `app-2`
* _nginx-statefulset.yaml_, manifest to deploy a `Statefulset`in namespace `app-2`
* _busybox-cronjob.yaml_, manifest to deploy a `Statefulset`in namespace `app-1`

`SleepCycle` is a namespace-scoped custom resource; the controller will monitor all the resources in that namespace that
are marked with a `Label` that has as key `rekuberate.io/sleepcycle:` and as value the `name` of the manifest you created:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-2
  namespace: app-2
  labels:
    app: app-2
    rekuberate.io/sleepcycle: sleepcycle-app-2
spec:
  replicas: 9
  selector:
    matchLabels:
      app: app-2
  template:
    metadata:
      name: app-2
      labels:
        app: app-2
    spec:
      containers:
        - name: app-2
          image: traefik/whoami
          imagePullPolicy: IfNotPresent
```

> [!IMPORTANT]
> Any workload in namespace `kube-system` marked with `rekuberate.io/sleepcycle` will be ignored by the controller **by design**.

## How it works

The diagram below describes how `rekuberate.io/sleepcycles` are dealing with scheduling a `Deployment`: 

1. The `sleepcycle-controller` **watches** periodically, every 1min, all the `SleepCycle` custom resources for changes (in **all** namespaces).
2. The controller, for **every** `SleepCycle` resource within the namespace `app-1`, collects all the resources that have been marked with the label `rekuberate.io/sleepcycle: sleepcycle-app1`.
3. It provisions, for **every** workload - in this case deployment `deployment-app1` a `CronJob` for the shutdown schedule and optionally a second `CronJob` if a wake-up schedule is provided.
4. It provisions a `ServiceAccount`, a `Role` and a `RoleBinding` **per namespace**, in order to make possible for runner-pods to update resources' specs.
5. The `Runner` pods will be created automatically by the cron jobs and are responsible for scaling the resources up or down. 


![SCR-20240527-q9y.png](docs/images/SCR-20240527-qei.png)

> [!NOTE]
> In the diagram it was depicted how `rekuberate.io/sleepcycles` scales `Deployment`. The same steps count for a
> `StatefulSet` and a `HorizontalPodAutoscaler`. There are two exception though:
> - a `HorizontalPodAutoscaler` will scale down to `1` replica and not to `0` as for a `Deployment` or a `Statefulset`.
> - a `CronJob` has no replicas to scale up or down, it is going to be enabled or suspended respectively.

## Deploy

### From sources

1. Build and push your image to the location specified by `IMG` in `Makefile`:

```shell
# Image URL to use all building/pushing image targets
IMG_TAG ?= $(shell git rev-parse --short HEAD)
IMG_NAME ?= rekuberate-io-sleepcycles
DOCKER_HUB_NAME ?= $(shell docker info | sed '/Username:/!d;s/.* //')
IMG ?= $(DOCKER_HUB_NAME)/$(IMG_NAME):$(IMG_TAG)
RUNNERS_IMG_NAME ?= rekuberate-io-sleepcycles-runners
KO_DOCKER_REPO ?= $(DOCKER_HUB_NAME)/$(RUNNERS_IMG_NAME)
```

```sh
make docker-build docker-push
```

2. Deploy the controller to the cluster using the image defined in `IMG`:

```sh
make deploy
```

and then deploy the samples:

```sh
kubectl create namespace app-1
kubectl create namespace app-2
kubectl apply -f config/samples
```

#### Uninstall

```sh
make undeploy
```

### Using Helm (from sources)

If you are on a development environment, you can quickly test & deploy the controller to the cluster
using a **Helm chart** directly from `config/helm`:

```sh
helm install rekuberate-io-sleepcycles config/helm/ -n <namespace> --create-namespace
```

and then deploy the samples:

```sh
kubectl create namespace app-1
kubectl create namespace app-2
kubectl apply -f config/samples
```
#### Uninstall

```shell
helm uninstall rekuberate-io-sleepcycles -n <namespace>
```

### Using Helm (from repo)

On the other hand if you are deploying on a production environment, it is **highly recommended** to deploy the 
controller to the cluster using a **Helm chart** from its repo:


```sh
helm repo add sleepcycles https://rekuberate-io.github.io/sleepcycles/
helm repo update

helm upgrade --install sleepcycles sleepcycles/sleepcycles -n rekuberate-system --create-namespace
```

and then deploy the samples:

```sh
kubectl create namespace app-1
kubectl create namespace app-2
kubectl apply -f config/samples
```
#### Uninstall

```shell
helm uninstall rekuberate-io-sleepcycles -n <namespace>
```

## Develop

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Controller

#### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make generate
make manifests
```

then install the CRDs in the cluster with:

```sh
make install
```

> [!TIP]
> You can debug the controller in the IDE of your choice by hooking to the `main.go` **or** you can start
> the controller _without_ debugging with:

```sh
make run
```

> [!TIP]
> Run `make --help` for more information on all potential `make` targets 
> More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

#### Build

You always need to build a new docker container and push it to your repository:

```sh
make docker-build docker-push
```

> [!IMPORTANT]
> In this case you will need to adjust your Helm chart values to use your repository and container image.

### Runner

#### Build

```sh
make ko-build-runner
```

> [!IMPORTANT]
> In this case you will need to adjust the `runnerImage` of your `SleepCycle` manifest to use your own Runner image.

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```
