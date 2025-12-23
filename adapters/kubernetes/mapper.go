package kubernetes

import (
	"github.com/cloudkops/infrasight/core/model"
	v1 "k8s.io/api/core/v1"
)

func PodToWorkload(pod v1.Pod) model.Workload {
	w := model.Workload{
		Name: pod.Name,
	}

	for _, c := range pod.Spec.Containers {
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

		w.Containers = append(w.Containers, model.Container{
			Name:       c.Name,
			Image:      c.Image,
			User:       user,
			Privileged: privileged,
		})
	}

	return w
}
