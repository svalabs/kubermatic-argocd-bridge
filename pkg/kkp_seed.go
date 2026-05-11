package pkg

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KKPSeed struct {
	Name                    string
	KubeConfig              restclient.Config
	dynamicClient           dynamic.DynamicClient
	staticClient            kubernetes.Interface
	clusterSchema           schema.GroupVersionResource
	machineDeploymentSchema schema.GroupVersionResource
	fetchMachineDeployments bool
	ManagementProxy         map[string]interface{}
}

type UserCluster struct {
	Seed               *KKPSeed
	ID                 string
	Name               string
	kubeconfig         []byte `json:"-"`
	RawData            map[string]interface{}
	MachineDeployments []map[string]interface{}
}

func NewSeed(name string, kubeconfig []byte, fetchMachineDeployments bool, managementProxySettings map[string]interface{}) (*KKPSeed, error) {
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
		Name:                    name,
		KubeConfig:              *loadedKubeConfig,
		dynamicClient:           *dynamicClient,
		staticClient:            staticClient,
		fetchMachineDeployments: fetchMachineDeployments,
		clusterSchema: schema.GroupVersionResource{
			Group:    "kubermatic.k8c.io",
			Version:  "v1",
			Resource: "clusters",
		},
		machineDeploymentSchema: schema.GroupVersionResource{
			Group:    "cluster.k8s.io",
			Version:  "v1alpha1",
			Resource: "machinedeployments",
		},
		ManagementProxy: managementProxySettings,
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

		var machineDeployments []map[string]interface{}
		if seed.fetchMachineDeployments {
			machineDeployments, err = seed.fetchMachineDeploymentsForUserCluster(kubeConfigSecret.Data["kubeconfig"])
			if err != nil {
				log.Printf("Failed to fetch MachineDeployments for UserCluster %s: %s\n", name, err)
				continue
			}
		}

		clusters = append(clusters, UserCluster{
			Seed:               seed,
			ID:                 id,
			Name:               name,
			kubeconfig:         kubeConfigSecret.Data["kubeconfig"],
			RawData:            cluster.Object,
			MachineDeployments: machineDeployments,
		})
	}

	return clusters, nil
}

func (seed *KKPSeed) fetchMachineDeploymentsForUserCluster(kubeconfig []byte) ([]map[string]interface{}, error) {
	loadedKubeConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	if seed.ManagementProxy != nil {
		proxyHost, _ := seed.ManagementProxy["proxyHost"].(string)
		proxyProtocol, _ := seed.ManagementProxy["proxyProtocol"].(string)
		if proxyHost != "" && proxyProtocol != "" {

			proxyURL := &url.URL{
				Scheme: proxyProtocol,
				Host:   proxyHost,
			}

			if seed.ManagementProxy["proxyPort"] != nil {
				proxyURL = &url.URL{
					Scheme: proxyProtocol,
					Host:   fmt.Sprintf("%s:%d", proxyHost, seed.ManagementProxy["proxyPort"]),
				}
			}
			loadedKubeConfig.Proxy = http.ProxyURL(proxyURL)
		}
	}

	dynamicClient, err := dynamic.NewForConfig(loadedKubeConfig)
	if err != nil {
		return nil, err
	}

	list, err := dynamicClient.Resource(seed.machineDeploymentSchema).Namespace("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var machineDeployments []map[string]interface{}

	for _, machineDeployment := range list.Items {
		machineDeployments = append(machineDeployments, machineDeployment.Object)
	}

	return machineDeployments, nil
}
