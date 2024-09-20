package pkg

import (
	"bytes"
	"context"
	"encoding/base64"
	stdErrors "errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"text/template"
)

const BASE_LABEL string = "kubermatic-argocd-bridge"
const ARGO_CLUSTER_LABEL string = "argocd.argoproj.io/secret-type=cluster"

type ArgoConnector struct {
	client         *kubernetes.Clientset
	namespace      string
	secretTemplate *template.Template
}

func NewArgoConnector(client *kubernetes.Clientset, namespace string, clusterSecretTemplate string) *ArgoConnector {
	templ, err := template.New("secret").Funcs(template.FuncMap{
		"base64": func(b []byte) string {
			return base64.StdEncoding.EncodeToString(b)
		},
	}).Parse(clusterSecretTemplate)
	if err != nil {
		log.Fatal("Failed to parse Secret template", err)
	}
	return &ArgoConnector{client, namespace, templ}
}

func (connector *ArgoConnector) VerifyNamespace() error {
	_, err := connector.client.CoreV1().Namespaces().Get(context.TODO(), connector.namespace, metav1.GetOptions{})
	return err
}

func (connector *ArgoConnector) CurrentClusters() ([]v1.Secret, error) {
	list, err := connector.client.CoreV1().Secrets(connector.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: ARGO_CLUSTER_LABEL + "," + BASE_LABEL + "/managed=true",
	})

	if err != nil {
		return nil, err
	}
	return list.Items, err
}

/**
 * Store the provided clusters inside ArgoCD
 */
func (connector *ArgoConnector) StoreClusters(userClusters []UserCluster, projects []KKPProject) error {
	reconciled := 0

	for _, userCluster := range userClusters {
		var project KKPProject
		projectID := userCluster.RawData["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["project-id"]

		for _, availableProject := range projects {
			if availableProject.ID == projectID {
				project = availableProject
				break
			}
		}

		err := connector.StoreClusterI(userCluster, project)
		if err != nil {
			return err
		}

		reconciled++
	}

	log.Println("Reconciled Argo Secrets for", reconciled, "UserClusters")

	return nil
}

/**
 * Builds the desired Secret and stores in inside the cluster
 */
func (connector *ArgoConnector) StoreClusterI(userCluster UserCluster, project KKPProject) error {

	filledTemplateRaw, err := connector.ParseTemplate(userCluster, project)

	if err != nil {
		return err
	}

	filledTemplate := filledTemplateRaw.(map[string]interface{})

	secretName := filledTemplate["name"].(string)

	labels, err := FlattenToStringStringMap(filledTemplate["labels"])

	if err != nil {
		return err
	}

	annotations, err := FlattenToStringStringMap(filledTemplate["annotations"])

	if err != nil {
		return err
	}

	data, err := FlattenToStringStringMap(filledTemplate["data"])

	if err != nil {
		return err
	}

	ctx := context.TODO()
	secret, err := connector.client.CoreV1().Secrets(connector.namespace).Get(ctx, secretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   connector.namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Data: TransformStringStringMapValuesToByteArray(data),
		}

		_, err := connector.client.CoreV1().Secrets(connector.namespace).Create(ctx, newSecret, metav1.CreateOptions{})

		return err
	} else {
		secret.Data = TransformStringStringMapValuesToByteArray(data)

		for key, value := range labels {
			secret.Labels[key] = value
		}
		for key, value := range annotations {
			secret.Annotations[key] = value
		}

		_, err := connector.client.CoreV1().Secrets(connector.namespace).Update(ctx, secret, metav1.UpdateOptions{})

		return err
	}

	return nil
}

/**
 * Accessable data during templating
 */
type TemplateData struct {
	UserCluster UserCluster
	BaseLabel   string
	KubeConfig  restclient.Config
	Project     KKPProject
	Labels      map[string]string
}

/**
 * Takes the provided Secret Template and renders it with different supported data
 */
func (contector *ArgoConnector) ParseTemplate(userCluster UserCluster, project KKPProject) (interface{}, error) {
	kubeconfig, err := clientcmd.RESTConfigFromKubeConfig(userCluster.kubeconfig)

	if err != nil {
		return nil, err
	}
	labels := map[string]string{}

	if project.RawData["metadata"].(map[string]interface{})["labels"] != nil {
		projectLabels, err := FlattenToStringStringMap(project.RawData["metadata"].(map[string]interface{})["labels"])
		if err != nil {
			return nil, err
		}

		for k, v := range projectLabels {
			labels[k] = v
		}
	}

	if userCluster.RawData["metadata"].(map[string]interface{})["labels"] != nil {
		clusterLabels, err := FlattenToStringStringMap(userCluster.RawData["metadata"].(map[string]interface{})["labels"])
		if err != nil {
			return nil, err
		}

		for k, v := range clusterLabels {
			labels[k] = v
		}
	}

	data := &TemplateData{
		UserCluster: userCluster,
		BaseLabel:   BASE_LABEL,
		KubeConfig:  *kubeconfig,
		Project:     project,
		Labels:      labels,
	}

	buf := &bytes.Buffer{}
	err = contector.secretTemplate.ExecuteTemplate(buf, "secret", data)
	if err != nil {
		return nil, err
	}

	var config interface{}

	err = yaml.Unmarshal(buf.Bytes(), &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

/**
 *
 */
func (connector *ArgoConnector) RemoveCluster(cluster v1.Secret) error {
	return connector.client.CoreV1().Secrets(connector.namespace).Delete(context.TODO(), cluster.ObjectMeta.Name, metav1.DeleteOptions{})
}

func (connector *ArgoConnector) UpdateCluster(cluster v1.Secret) error {
	_, err := connector.client.CoreV1().Secrets(connector.namespace).Update(context.TODO(), &cluster, metav1.UpdateOptions{})
	return err
}

/**
 * Flattens a map[string]interface{} to a map[string]string by converting all non string values via json
 */
func FlattenToStringStringMap(config interface{}) (map[string]string, error) {
	flattened := map[string]string{}

	unboxed, ok := config.(map[string]interface{})
	if !ok {
		json, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		return nil, stdErrors.New("Invalid interface provided" + string(json))
	}

	if unboxed != nil {
		for key, value := range unboxed {
			switch valueType := value.(type) {
			case string:
				flattened[key] = valueType
			default:
				stringValue, err := json.Marshal(value)
				if err != nil {
					return nil, err
				}
				flattened[key] = string(stringValue)
			}

		}
	}

	return flattened, nil
}

/**
 * Converts a string-string map to string-byte, to be able to use it as secretdata
 */
func TransformStringStringMapValuesToByteArray(values map[string]string) map[string][]byte {
	output := map[string][]byte{}

	for k, v := range values {
		output[k] = []byte(v)
	}

	return output
}
