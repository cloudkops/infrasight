package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/cloudkops/infrasight/core/model"
)

// DeploymentList lists all Deployments in the given namespace (empty = all).
func DeploymentList(client *kubernetes.Clientset, namespace string) ([]appsv1.Deployment, error) {
	list, err := client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// DeploymentToWorkload maps a Kubernetes Deployment into the InfraSight model.
func DeploymentToWorkload(d appsv1.Deployment) model.Workload {
	return model.Workload{
		ID:        string(d.UID),
		Name:      d.Name,
		Namespace: d.Namespace,
		Type:      "deployment",
		Platform:  "kubernetes",
		Metadata: model.Metadata{
			Labels:      d.Labels,
			Annotations: d.Annotations,
		},
		Containers: mapContainers(d.Spec.Template.Spec.Containers, d.Spec.Template.Spec.HostNetwork),
	}
}
