# K8s Ns Setup

Dummy tool to create a multi tenants Kubernetes cluster.

## Description

This tool was built in order to prepare a Kubernetes cluster (GKE) for a Lab. It eases the creation of severals namespaces for a multi-tenants usage and also the creation of a dedicate ServiceAccount for each namespaces with proper isolation between each user/namespace with Role and Rolebindings. It also generates a kubeconfig dedicated for each user.

## Usage

First you need to have already a kubernetes cluster up and running. You can look a the session `How to setup a Kubernetes Cluster`.

### Create one new namespace with its associated user(s)

```console
$ k8snssetup new-ns --help
Creates a new namespace with new user

Usage:
  k8snssetup new-ns <namespace-name> [required-flags] [flags]

Flags:
  -h, --help                help for new-ns
      --kubeconfig string   kubeconfig file path
      --output string       kubeconfig file path
      --user stringArray    user name
```

example

```console
$ k8snssetup new-ns project1 --user user1 --kubeconfig=/tmp/admin.kubeconfi.yaml
namespace "project1" created
```

### Create several namespace and associated user in one command line

```console
./k8snssetup multi --help
Creates several namespaces and associated user

Usage:
  k8snssetup multi <number of namespace> [required-flags] [flags]

Flags:
  -h, --help                 help for multi
      --kubeconfig string    kubeconfig file path
      --ns-prefix string     namespace prefix (default "project")
      --output string        kubeconfig file path
      --user-prefix string   user prefix (default "user")

```

### How to use the command output files

For each user/namespace a `kubeConfig` file have been generated.

for example to use the generated `kube config` file generated for the `user1` in namespace `project1`.

```console
kubectl get pods --kubeconfig=$(pwd)/project1-user1.kubeconfig.yaml
No resources found.
```

and also proxy the kubernetes dashboard

```console
kubectl proxy --kubeconfig=$(pwd)/project1-user1.kubeconfig.yaml
Starting to serve on 127.0.0.1:8001
```

Thank to that you will be able to access the kubernetes dashboard with a limitation to the namespace qssociated to the uer.
You can access the resources inside `user1`'s dedicated namespace thanks to this link:

`http://localhost:8001/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy/#!/overview?namespace=project1`

## How to setup a Kubernetes Cluster

If your goal is also to create a multi-tenant cluster for a lab. It is maybe more conveniant to do it on a public cloud like `Google Cloud Platfom`.
But It works also with a Kubernetes Cluster generate with `minikube`.


### GKE cluster (Google Cloud Plafrom)

First you need to create GKE cluster [doc](https://cloud.google.com/kubernetes-engine/docs/), then configure your kubectl command line (generate|update the kubeconfig file).

```console
$ gcloud container clusters get-credentials <clusterName> --zone <zoneName> --project <projectName>
Fetching cluster endpoint and auth data.
kubeconfig entry generated for <clusterName>
```

You need to allow your user to create new Role and RoleBinding. To do so, add a cluster-admin Role to your user.

```console
$ kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user <user>
clusterrolebinding.rbac.authorization.k8s.io "cluster-admin-binding" created
```

### Minikube

You can run a Kubernetes cluster (of one node) thanks to [minikube](https://github.com/kubernetes/minikube). Install minikube by following the instruction on the github page.

then run the following command:

```console
$ minikube start --extra-config=apiserver.Authorization-Mode=RBAC
Starting local Kubernetes v1.9.0 cluster...
Starting VM...
...
```