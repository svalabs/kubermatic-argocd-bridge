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

// Default cluster secret template
//
//go:embed template/cluster-secret.yaml
var defaultClusterSecretTemplate string

func main() {

	kkpKubeConfigPath := flag.String("kkp-kubeconfig", "", "Provide the path to the KKP KubeConfig")
	kkpServiceAccount := flag.Bool("kkp-serviceaccount", true, "If the default service account should be used for kkp connection")
	kkpClusterName := flag.String("kkp-cluster-name", "", "If set, add this string as identifier to your cluster secrets. Useful if you have multiple KKP clusters.")
	argoKubeConfigPath := flag.String("argo-kubeconfig", "", "Provide the path to the KKP KubeConfig")
	argoServiceAccount := flag.Bool("argo-serviceaccount", true, "If the default service account should be used for the argocd connection")
	argoCdNamespace := flag.String("argo-namespace", "argocd", "ArgoCD Namespace")
	refreshInterval := flag.Duration("refresh-interval", 60*time.Second, "Refresh interval")
	clusterSecretTemplateFlag := flag.String("cluster-secret-template", "", "Cluster Secret Template file")
	cleanupRemovedClusters := flag.Bool("cleanup-removed-clusters", false, "Cleanup removed clusters")
	cleanupTimedClusters := flag.Bool("cleanup-timed-clusters", false, "Cleanup clusters from removed/unavailable clusters")
	clusterTimeoutTime := flag.Duration("cluster-timeout-time", 30*time.Second, "Time before a cluster gets deleted, when cleanup-timed-clusters is enabled ")

	flag.Parse()

	clusterSecretTemplate := defaultClusterSecretTemplate

	if *clusterSecretTemplateFlag != "" {
		stat, err := os.Stat(*clusterSecretTemplateFlag)

		if err != nil || stat.IsDir() {
			log.Fatal("Failed to stat clusterSecretTemplateFlag: ", err)
		}

		data, err := os.ReadFile(*clusterSecretTemplateFlag)
		if err != nil {
			log.Fatal("Failed to read clusterSecretTemplateFlag: ", err)
		}

		clusterSecretTemplate = string(data)
	}

	kkpKubeConfig, err := GetKubeConfig(*kkpKubeConfigPath, *kkpServiceAccount)
	if err != nil {
		log.Fatal("Failed to generate KKP KubeConfig: ", err)
	}
	argoKubeConfig, err := GetKubeConfig(*argoKubeConfigPath, *argoServiceAccount)
	if err != nil {
		log.Fatal("Failed to generate Argo KKP KubeConfig: ", err)
	}

	kkpArgoBridge, err := bridge.NewBridge(kkpKubeConfig, *kkpClusterName, argoKubeConfig, *argoCdNamespace, *refreshInterval, clusterSecretTemplate, *cleanupRemovedClusters, *cleanupTimedClusters, *clusterTimeoutTime)

	if err != nil {
		log.Fatal("Failed to initiate bridge", err)
	}

	kkpArgoBridge.Connect()
}

func GetKubeConfig(kubeConfigPath string, useServiceAccount bool) (*restclient.Config, error) {
	if len(kubeConfigPath) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}

	if useServiceAccount {
		log.Println("Using Service Account")
		config, err := restclient.InClusterConfig()
		if err != nil {
			log.Printf("No service Account found trying default kubeconfig: %s\n", err)
			return nil, err
		} else {
			return config, nil
		}
	}

	kubeConfigPath = os.Getenv("KUBECONFIG")
	if len(kubeConfigPath) == 0 {
		if home := homedir.HomeDir(); home != "" {
			kubeConfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)

}
