package main

import (
	_ "embed"
	"flag"
	bridge "github.com/svalabs/kubermatic-argocd-bridge/pkg"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"os"
	"path/filepath"
	"time"
)

//go:embed template/cluster-secret.yaml
var defaultClusterSecretTemplate string

func main() {

	kkpKubeConfigPath := flag.String("kkp-kubeconfig", "", "Provide the path to the KKP KubeConfig")
	argoKubeConfigPath := flag.String("argo-kubeconfig", "", "Provide the path to the KKP KubeConfig")
	argoCdNamespace := flag.String("argo-namespace", "argocd", "ArgoCD Namespace")
	refreshInterval := flag.Duration("refresh-interval", 60*time.Second, "Refresh interval")
	clusterSecretTemplateFlag := flag.String("cluster-secret-template", "", "Cluster Secret Template file")
	flag.Parse()

	clusterSecretTemplate := defaultClusterSecretTemplate

	if *clusterSecretTemplateFlag != "" {
		stat, err := os.Stat(*clusterSecretTemplateFlag)
		if err == nil && !stat.IsDir() {
			data, err := os.ReadFile(*clusterSecretTemplateFlag)

			if err == nil {
				clusterSecretTemplate = string(data)
			}
		}
	}

	kkpKubeConfig, err := GetKubeConfig(*kkpKubeConfigPath)
	if err != nil {
		log.Fatal("Failed to generate KKP KubeConfig: ", err)
	}
	argoKubeConfig, err := GetKubeConfig(*argoKubeConfigPath)
	if err != nil {
		log.Fatal("Failed to generate Argo KKP KubeConfig: ", err)
	}

	kkpArgoBridge, err := bridge.NewBridge(kkpKubeConfig, argoKubeConfig, *argoCdNamespace, *refreshInterval, clusterSecretTemplate)

	if err != nil {
		log.Fatal("Failed to initiate bridge", err)
	}

	kkpArgoBridge.Connect()
}

func GetKubeConfig(kubeConfigPath string) (*restclient.Config, error) {
	if len(kubeConfigPath) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	log.Println("Trying Service Account")
	config, err := restclient.InClusterConfig()
	if err != nil {
		log.Println("No service Account found trying default kubeconfig: ", err)
	} else {
		return config, nil
	}

	kubeConfigPath = os.Getenv("KUBECONFIG")
	if len(kubeConfigPath) == 0 {
		if home := homedir.HomeDir(); home != "" {
			kubeConfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	log.Println(kubeConfigPath)
	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)

}
