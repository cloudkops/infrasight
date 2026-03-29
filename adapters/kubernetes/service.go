package kubernetes

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/cloudkops/infrasight/core/model"
)

// ServiceList lists all Services in the given namespace (empty = all).
func ServiceList(client *kubernetes.Clientset, namespace string) ([]v1.Service, error) {
	list, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ServiceToWorkload maps a Kubernetes Service into the InfraSight model.
// Services are represented as a workload with a single virtual container
// carrying the network exposure information.
func ServiceToWorkload(svc v1.Service) model.Workload {
	exposures := buildServiceExposures(svc)

	return model.Workload{
		ID:        string(svc.UID),
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      "service",
		Platform:  "kubernetes",
		Metadata: model.Metadata{
			Labels:      svc.Labels,
			Annotations: svc.Annotations,
		},
		Containers: []model.Container{
			{
				Name:      svc.Name,
				Exposures: exposures,
			},
		},
	}
}

func buildServiceExposures(svc v1.Service) []model.Exposure {
	var exposures []model.Exposure
	switch svc.Spec.Type {
	case v1.ServiceTypeNodePort:
		for _, p := range svc.Spec.Ports {
			exposures = append(exposures, model.Exposure{Type: "NodePort", Port: p.NodePort})
		}
	case v1.ServiceTypeLoadBalancer:
		exposures = append(exposures, model.Exposure{Type: "publicIP"})
		for _, p := range svc.Spec.Ports {
			exposures = append(exposures, model.Exposure{Type: "port", Port: p.Port})
		}
	}
	return exposures
}
