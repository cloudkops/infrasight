package kubernetes

import (
	"github.com/cloudkops/infrasight/core/model"
	v1 "k8s.io/api/core/v1"
)

func PodToWorkload(pod v1.Pod) model.Workload {
	w := model.Workload{
		ID:        string(pod.UID),
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Type:      "pod",
		Platform:  "kubernetes",
		Metadata: model.Metadata{
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
	}
	w.Containers = mapContainers(pod.Spec.Containers, pod.Spec.HostNetwork)
	return w
}

// mapContainers converts a Kubernetes container list into the model.
func mapContainers(containers []v1.Container, hostNetwork bool) []model.Container {
	result := make([]model.Container, 0, len(containers))
	for _, c := range containers {
		result = append(result, mapContainer(c, hostNetwork))
	}
	return result
}

func mapContainer(c v1.Container, hostNetwork bool) model.Container {
	user := int64(0)
	privileged := false

	if c.SecurityContext != nil {
		if c.SecurityContext.RunAsUser != nil {
			user = *c.SecurityContext.RunAsUser
		}
		if c.SecurityContext.Privileged != nil {
			privileged = *c.SecurityContext.Privileged
		}
	}

	resources := model.Resource{}
	if q, ok := c.Resources.Requests[v1.ResourceCPU]; ok {
		resources.CPURequest = q.String()
	}
	if q, ok := c.Resources.Requests[v1.ResourceMemory]; ok {
		resources.MemoryRequest = q.String()
	}
	if q, ok := c.Resources.Limits[v1.ResourceCPU]; ok {
		resources.CPULimit = q.String()
	}
	if q, ok := c.Resources.Limits[v1.ResourceMemory]; ok {
		resources.MemoryLimit = q.String()
	}

	exposures := make([]model.Exposure, 0, len(c.Ports)+1)
	if hostNetwork {
		exposures = append(exposures, model.Exposure{Type: "hostNetwork"})
	}
	for _, p := range c.Ports {
		exposures = append(exposures, model.Exposure{Type: "port", Port: p.ContainerPort})
	}

	return model.Container{
		Name:       c.Name,
		Image:      c.Image,
		User:       user,
		Privileged: privileged,
		Resources:  resources,
		Exposures:  exposures,
	}
}
