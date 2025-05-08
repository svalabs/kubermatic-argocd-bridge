# Kubermatic ArgoCD Bridge

This Project builds a bridge between
the [Kubermatic Kubernetes Platform](https://www.kubermatic.com/products/kubermatic-kubernetes-platform/)
and [ArgoCD](https://argo-cd.readthedocs.io/en/stable/), by auto importing UserClusters into KKP.

Project is under active Development

## Getting Started

### Requirements

You need to have a
existing [Kubermatic Kubernetes Platform](https://www.kubermatic.com/products/kubermatic-kubernetes-platform/)
and [ArgoCD](https://argo-cd.readthedocs.io/en/stable/) Installation.
The base configuration of the Helm Chart is designed for booth beeing installed on the same cluster together with the
bridge. But it is possible to spread all 3 components to different clusters/locations.

### How to use it

#### Helm Chart

We have a Helm Chart to deploy the bridge, which can be found [here](https://github.com/svalabs/kubermatic-argocd-bridge/blob/main/chart/README.md). The Helmchart will handle the commandline parameters and will create the required Serviceaccount and related Rbac if enabled

#### Docker

If you want to decouple this bridge from your kubernetes infrastructure or just want a quick dev/test environment, this also possible.
If you dont wont to use the public image, you can [build it yourself](#docker-image). Afterwards you can run it in your container environment for example docker:

> docker run -v $HOME/.kube/config:/etc/kubeconfig -e KUBECONFIG=/etc/kubeconfig ghcr.io/svalabs/kubermatic-argocd-bridge:[version]


### Anywhere else

It is possible to run this bridge anywhere outside kubernetes/containers by just running the compiled binary and
providing it with kubeapi access for KKP and ArgoCD

You can obtain the binary by [building it yourself](#raw-binary) or download if from the [releases](https://github.com/svalabs/kubermatic-argocd-bridge/releases).
After that, you can run the binary with the required [parametes](#parameters)

### Parameters

> **Important**: The following parameters are ment for docker or raw binary environments, when using the Helm
> Installtion, refer to our [Helmchart](https://github.com/svalabs/kubermatic-argocd-bridge/blob/main/chart/README.md) for
> further customization

| Parameter                 | value                                                           | default value | description                                                                                                                                                                                                                   |
|---------------------------|-----------------------------------------------------------------|---------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| -kkp-kubeconfig           | System Path                                                     | ""            | Path to the kubeconfig, which should be used for the connection to KKP                                                                                                                                                        | 
| -kkp-serviceaccount       | Boolean                                                         | true          | If the default service account in your pod should be used for the connection to KPK                                                                                                                                           | 
| -argo-kubeconfig          | System Path                                                     | ""            | Path to the kubeconfig, which should be used for the connection to ArgoCD                                                                                                                                                     | 
| -argo-serviceaccount      | Boolean                                                         | true          | If the default service account in your pod should be used for the connection to ArgoCD                                                                                                                                        | 
| -argo-namespace           | String                                                          | argocd        | The ArgoCD namespace, where the secrets get managed                                                                                                                                                                           | 
| -refresh-interval         | [Duration](https://pkg.go.dev/maze.io/x/duration#ParseDuration) | 60s           | How often the clusters should be synced                                                                                                                                                                                       | 
| -cluster-secret-template  | System Path                                                     | ""            | Path to the custom secret Template, to add addition information to your cluster secret, use the [default](https://github.com/svalabs/kubermatic-argocd-bridge/blob/main/cmd/template/cluster-secret.yaml) as a starting point |
| -cleanup-removed-clusters | Boolean                                                         | false         | If enabled, UserClusters which no longer exist at their seed, get also removed from ArgoCD                                                                                                                                    |
| -cleanup-timed-clusters   | Boolean                                                         | false         | If enabled, UserClusters whose seed got removed or is not reachable, are remove after a specific timeout                                                                                                                      |                                                                                                                     |
| -cluster-timeout-time     | [Duration](https://pkg.go.dev/maze.io/x/duration#ParseDuration) | 30s           | After which duration clusters will be removed, if `-cleanup-timed-clusters` is enabled                                                                                                                                        |                                                                                                                     |

## Build it yourself

### Docker Image

```
docker build -t [image name] .
docker push -t [image name]
```

### Raw Binary
```
go mod download
cd cmd
go build -o kubermatic-argocd-bridge
```


