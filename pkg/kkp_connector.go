package pkg

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"log"
)

type KKPConnector struct {
	dynamicClient dynamic.DynamicClient
	staticClient  kubernetes.Clientset
	seedSchema    schema.GroupVersionResource
	projectSchema schema.GroupVersionResource
}
type KKPProject struct {
	Name    string
	ID      string
	RawData map[string]interface{}
}

func NewKKPConnector(dynamicClient *dynamic.DynamicClient, staticClient *kubernetes.Clientset) *KKPConnector {

	return &KKPConnector{
		dynamicClient: *dynamicClient,
		staticClient:  *staticClient,
		seedSchema: schema.GroupVersionResource{
			Group:    "kubermatic.k8c.io",
			Version:  "v1",
			Resource: "seeds",
		},
		projectSchema: schema.GroupVersionResource{
			Group:    "kubermatic.k8c.io",
			Version:  "v1",
			Resource: "projects",
		},
	}
}

func (connector *KKPConnector) VerifyCRD() error {
	_, err := connector.dynamicClient.Resource(connector.seedSchema).List(context.TODO(), metav1.ListOptions{})
	return err
}

func (connector *KKPConnector) GetSeeds() ([]KKPSeed, error) {
	seedCrds, err := connector.dynamicClient.Resource(connector.seedSchema).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	seeds := []KKPSeed{}

	for _, seedConfig := range seedCrds.Items {
		spec := seedConfig.Object["spec"].(map[string]interface{})
		name := seedConfig.Object["metadata"].(map[string]interface{})["name"].(string)
		kubeconfigSpec := spec["kubeconfig"].(map[string]interface{})
		kubeconfigName := kubeconfigSpec["name"].(string)
		kubeconfigNamespace := kubeconfigSpec["namespace"].(string)

		kubeconfigSecret, err := connector.staticClient.CoreV1().Secrets(kubeconfigNamespace).Get(context.TODO(), kubeconfigName, metav1.GetOptions{})
		if err != nil {
			log.Println("Failed to get kubeconfig for seed ", name, err)
			continue
		}

		seed, err := NewSeed(name, kubeconfigSecret.Data["kubeconfig"])
		if err != nil {
			log.Println("Failed to create seed ", name, err)
			continue
		}
		seeds = append(seeds, *seed)
	}

	return seeds, nil
}

func (connector *KKPConnector) GetProjects() ([]KKPProject, error) {
	projectCrds, err := connector.dynamicClient.Resource(connector.projectSchema).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	projects := []KKPProject{}

	for _, projectCrd := range projectCrds.Items {
		id := projectCrd.Object["metadata"].(map[string]interface{})["name"].(string)
		name := projectCrd.Object["spec"].(map[string]interface{})["name"].(string)

		projects = append(projects, KKPProject{
			Name:    name,
			ID:      id,
			RawData: projectCrd.Object,
		})
	}

	return projects, nil

}
