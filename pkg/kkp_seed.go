package pkg

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

type KKPSeed struct {
	Name          string
	KubeConfig    restclient.Config
	dynamicClient dynamic.DynamicClient
	staticClient  kubernetes.Interface
	clusterSchema schema.GroupVersionResource
}

type UserCluster struct {
	Seed       *KKPSeed
	ID         string
	Name       string
	kubeconfig []byte `json:"-"`
	RawData    map[string]interface{}
}

func NewSeed(name string, kubeconfig []byte) (*KKPSeed, error) {
	loadedKubeConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(loadedKubeConfig)
	if err != nil {
		return nil, err
	}

	staticClient, err := kubernetes.NewForConfig(loadedKubeConfig)
	if err != nil {
		return nil, err
	}

	return &KKPSeed{
		Name:          name,
		KubeConfig:    *loadedKubeConfig,
		dynamicClient: *dynamicClient,
		staticClient:  staticClient,
		clusterSchema: schema.GroupVersionResource{
			Group:    "kubermatic.k8c.io",
			Version:  "v1",
			Resource: "clusters",
		},
	}, nil
}

func (seed *KKPSeed) GetUserClusters() ([]UserCluster, error) {
	clustersCrds, err := seed.dynamicClient.Resource(seed.clusterSchema).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusters := []UserCluster{}

	for _, cluster := range clustersCrds.Items {
		id := cluster.Object["metadata"].(map[string]interface{})["name"].(string)
		name := cluster.Object["spec"].(map[string]interface{})["humanReadableName"].(string)
		nameSpace := "cluster-" + id

		kubeConfigSecret, err := seed.staticClient.CoreV1().Secrets(nameSpace).Get(context.TODO(), "admin-kubeconfig", metav1.GetOptions{})

		if err != nil {
			log.Printf("Failed to get UserCluster Kubeconfig %s.%s\n", nameSpace, kubeConfigSecret)
			continue
		}

		clusters = append(clusters, UserCluster{
			Seed:       seed,
			ID:         id,
			Name:       name,
			kubeconfig: kubeConfigSecret.Data["kubeconfig"],
			RawData:    cluster.Object,
		})
	}

	return clusters, nil
}
