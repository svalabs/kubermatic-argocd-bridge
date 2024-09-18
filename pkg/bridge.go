package pkg

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"log"
)

type KKPArgoBridge struct {
	argoCDNamespace       string
	argoClient            *kubernetes.Clientset
	kkpDynamicClient      *dynamic.DynamicClient
	kkpStaticClient       *kubernetes.Clientset
	refreshTime           time.Duration
	clusterSecretTemplate string
}

func NewBridge(kkpKubeConfig *restclient.Config, argoKubeConfig *restclient.Config, argoCdNamespace string, duration time.Duration, clusterSecretTemplate string) (*KKPArgoBridge, error) {
	if kkpKubeConfig == nil {
		return nil, errors.New("kkpKubeConfig is nil")
	}

	if argoKubeConfig == nil {
		argoKubeConfig = kkpKubeConfig
		log.Println("No ArgoCD Kubeconfig provided, falling back to one cluster for both")
	}

	log.Println("Building kube clients")

	argoClient, err := kubernetes.NewForConfig(argoKubeConfig)
	if err != nil {
		return nil, err
	}
	kkpClient, err := dynamic.NewForConfig(kkpKubeConfig)
	if err != nil {
		return nil, err
	}
	kkpStaticClient, err := kubernetes.NewForConfig(argoKubeConfig)
	if err != nil {
		return nil, err
	}
	return &KKPArgoBridge{
		argoCDNamespace:       argoCdNamespace,
		argoClient:            argoClient,
		kkpDynamicClient:      kkpClient,
		kkpStaticClient:       kkpStaticClient,
		refreshTime:           duration,
		clusterSecretTemplate: clusterSecretTemplate,
	}, nil
}

func (bridge *KKPArgoBridge) Connect() {
	log.Println("Creating Bridge")

	kkpConnector := NewKKPConnector(bridge.kkpDynamicClient, bridge.kkpStaticClient)
	argoConnector := NewArgoConnector(bridge.argoClient, bridge.argoCDNamespace, bridge.clusterSecretTemplate)

	err := kkpConnector.VerifyCRD()
	if err != nil {
		log.Fatalln("Failed to verify that KKP is installed", err)
	}

	err = argoConnector.VerifyNamespace()
	if err != nil {
		log.Fatal("The provided argocd namespace does not exist: ", argoConnector.namespace, err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdown
		log.Println("Shutting down Bridge")
		os.Exit(1)
	}()

	for {
		start := time.Now()

		err := bridge.Sync(kkpConnector, argoConnector)
		if err != nil {
			log.Println("Failed to sync bridge", err)
		}
		log.Println("Sync took", time.Since(start))
		if time.Since(start) < bridge.refreshTime {
			time.Sleep(bridge.refreshTime - time.Since(start))
		}
	}

}

func (bridge *KKPArgoBridge) Sync(kkpConnector *KKPConnector, argoConnector *ArgoConnector) error {
	log.Println("Syncing Clusters")

	projects, err := kkpConnector.GetProjects()
	if err != nil {
		return err
	}

	seeds, err := kkpConnector.GetSeeds()

	if err != nil {
		return err
	}

	allUserClusters := []UserCluster{}

	connectedSeeds := []KKPSeed{}

	for _, seed := range seeds {

		userClusters, err := seed.GetUserClusters()
		if err != nil {
			log.Println("Failed to get user clusters for seed", seed.Name, err)
			continue
		}

		connectedSeeds = append(connectedSeeds, seed)
		allUserClusters = append(allUserClusters, userClusters...)

	}

	log.Println("Got", len(allUserClusters), "UserClusters")

	err = argoConnector.StoreClusters(allUserClusters, connectedSeeds, projects)

	return err
}
