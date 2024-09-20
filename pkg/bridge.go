package pkg

import (
	"errors"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"log"
)

type KKPArgoBridge struct {
	argoCDNamespace        string
	argoClient             *kubernetes.Clientset
	kkpDynamicClient       *dynamic.DynamicClient
	kkpStaticClient        *kubernetes.Clientset
	refreshTime            time.Duration
	clusterSecretTemplate  string
	cleanupRemovedClusters bool
	cleanupTimedClusters   bool
	clusterTimeout         time.Duration
}

func NewBridge(kkpKubeConfig *restclient.Config, argoKubeConfig *restclient.Config, argoCdNamespace string, duration time.Duration, clusterSecretTemplate string, cleanupRemovedClusters bool, cleanupTimedClusters bool, clusterTimeout time.Duration) (*KKPArgoBridge, error) {
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
		argoCDNamespace:        argoCdNamespace,
		argoClient:             argoClient,
		kkpDynamicClient:       kkpClient,
		kkpStaticClient:        kkpStaticClient,
		refreshTime:            duration,
		clusterSecretTemplate:  clusterSecretTemplate,
		cleanupRemovedClusters: cleanupRemovedClusters,
		cleanupTimedClusters:   cleanupTimedClusters,
		clusterTimeout:         clusterTimeout,
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

	err = argoConnector.StoreClusters(allUserClusters, projects)
	if err != nil {
		return err
	}

	err = bridge.CleanupClusters(argoConnector, allUserClusters, connectedSeeds)

	return err
}

/**
 * If -cleanup-removed-clusters is set to true, removes cluster which are no longer held by their seed and the seed is still available
 * If -cleanup-timed-clusters is set to true, removes cluster whos seed does no longer exists or is unreachable, after -cluster-timeout-time (default 30 seconds)
 */
func (bridge *KKPArgoBridge) CleanupClusters(argoConnector *ArgoConnector, userClusters []UserCluster, seeds []KKPSeed) error {

	if bridge.cleanupRemovedClusters == false && bridge.cleanupTimedClusters == false {
		return nil
	}
	clusters, err := argoConnector.CurrentClusters()
	if err != nil {
		return err
	}

clusters:
	for _, existingCluster := range clusters {
		clusterID := existingCluster.ObjectMeta.Labels[BASE_LABEL+"/cluster-id"]
		if len(clusterID) == 0 {
			log.Println("Invalid existing Cluster Secret(missing "+BASE_LABEL+"/cluster-id label)", existingCluster.ObjectMeta.Name)
			continue
		}
		seedName := existingCluster.ObjectMeta.Labels[BASE_LABEL+"/seed"]

		if len(seedName) == 0 {
			log.Println("Invalid existing Cluster Secret(missing "+BASE_LABEL+"/seed label)", existingCluster.ObjectMeta.Name)
			continue
		}

		for _, userCluster := range userClusters {
			if userCluster.ID == clusterID {
				continue clusters
			}
		}

		for _, seed := range seeds {
			if seed.Name == seedName {
				if bridge.cleanupRemovedClusters {
					log.Println("Deleting removed cluster", existingCluster.ObjectMeta.Name)
					err = argoConnector.RemoveCluster(existingCluster)
					if err != nil {
						log.Println("Failed to remove cluster", existingCluster.ObjectMeta.Name, err)
					}
				}
				continue clusters
			}
		}

		if bridge.cleanupTimedClusters {
			timeoutStart := existingCluster.ObjectMeta.Labels[BASE_LABEL+"/timeout-start"]
			if len(timeoutStart) == 0 {
				existingCluster.ObjectMeta.Labels[BASE_LABEL+"/timeout-start"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
				err = argoConnector.UpdateCluster(existingCluster)
				if err != nil {
					log.Println("Failed to add timeout start to", existingCluster.ObjectMeta.Name, err)
					continue clusters
				}
			} else {
				startMillis, err := strconv.ParseInt(timeoutStart, 10, 64)
				if err != nil {
					log.Println("Failed to parse timeout start ("+BASE_LABEL+"/timeout-start)", timeoutStart)
					continue clusters
				}
				if time.Since(time.UnixMilli(startMillis)) > bridge.clusterTimeout {
					log.Println("Cleaning up expired cluster", existingCluster.ObjectMeta.Name)
					err = argoConnector.RemoveCluster(existingCluster)
					if err != nil {
						log.Println("Failed to remove cluster", existingCluster.ObjectMeta.Name, err)
					}
					continue clusters
				}
			}
		}

	}

	return nil
}
