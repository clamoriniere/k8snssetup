# K8s Ns Setup

Dummy tool to create a multi tenants Kubernetes cluster.

## Description

This tool was build in order to prepare a Kubernetes cluster (GKE) for a Lab. It eases the creation of severals namespaces for a multi-tenants usage and also the creation of a dedicate ServiceAccount for each namespaces with proper isolation between each user/namespace with Role and Rolebindings. It also generate a kubeconfig dedicated for each user.

## Usage

First you need to have already a kubernetes cluster up and running. You can look a the session `How to setup a Kubernetes Cluster`.

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
$ k8snssetup new-ns user1 --user user1 --kubeconfig=/tmp/admin.kubeconfi.yaml
namespace "user1" created
```

then you can use the generated `user1` `kubeconfig`

```console
kubectl get pods --kubeconfig=$(pwd)/user1-user1.kubeconfig.yaml
No resources found.
```

and also proxy the kubernetes dashboard

```console
kubectl  proxy --kubeconfig=$(pwd)/user1-user1.kubeconfig.yaml
Starting to serve on 127.0.0.1:8001
```

Thank to that you will be able to access the kubernetes dashboard with limited namespace access.
If I still follow my example. I can access the resource from my dedicated `user1` namespace thanks to this link:
`http://localhost:8001/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy/#!/overview?namespace=user1`

## How to setup a Kubernetes Cluster

If your goal is also to create a multi-tenant cluster for a lab. It is maybe more conveniant to do it on a public cloud like `Google Cloud Platfom`.
But It works also with a Kubernetes Cluster generate with `minikube`.


### GKE cluster (Google Cloud Plafrom)

First you need to create GKE cluster (doc)[https://cloud.google.com/kubernetes-engine/docs/], then configure your kubectl command line (generate|update the kubeconfig file).

```console
$ gcloud container clusters get-credentials <clusterName> --zone <zoneName> --project <projectName>
Fetching cluster endpoint and auth data.
kubeconfig entry generated for <clusterName>
```

Then run at least one, a kubectl command in order to generate the access tocken (ex: ```kubectl get nodes```).

You need to allow your user to create new Role and RoleBinding. To do so you need to add a cluster-admin Role to your user.

```console
$ kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user <user>
clusterrolebinding.rbac.authorization.k8s.io "cluster-admin-binding" created
```

After that you can run the command:

```console
$ kubectl config view --flatten --minify > admin.kubeconfig.yaml
.
```

You need to do a small change in the generated kubeconfig file. (I didn't have time to automate this part for now.):

- Move ```users.user.auth-provider.config.access-token``` to ```users.user.token```
- Remove the full section ```users.user.auth-provider```

Now you are ready to use this ```kubeconfig``` with the `k8snssetup` tool.

### Minikube

// TODO: document this session.