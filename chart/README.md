# Kubermatic ArgoCD Bridge

A Helm chart to deploy your kkp argocd bridge on a kubernetes cluster

The default settings are designed to accomodate as setup, where the bridge, kkp and argo run on the same cluster, using
a service account which will be created and disabled cleanup.
This behaviour can be adjusted by changing the [values](https://github.com/svalabs/kubermatic-argocd-bridge/blob/main/chart/values.yaml)

This Chart is currently not pushed inside a registry due to active development, to use it, follow the following steps 

- clone the repo
- create a customized values file if needed
- > helm upgrade --install -n <namespace> --create-namespace <release-name> ./chart -f <customized values>
