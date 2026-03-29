package isg

import (
	"fmt"

	"github.com/cloudkops/infrasight/adapters/kubernetes"
	"github.com/cloudkops/infrasight/core/model"
)

func loadWorkloads(opts *scanOptions) ([]model.Workload, error) {
	switch opts.source {
	case scanSourceK8s:
		return loadK8sWorkloads(opts)
	case scanSourceAuto:
		if opts.k8sFlag {
			return loadK8sWorkloads(opts)
		}
		return []model.Workload{}, nil
	default:
		return []model.Workload{}, nil
	}
}

func loadK8sWorkloads(opts *scanOptions) ([]model.Workload, error) {
	ns := opts.namespace
	if opts.allNamespaces {
		ns = ""
	}

	client, err := kubernetes.NewClient()
	if err != nil {
		return nil, err
	}

	var workloads []model.Workload

	// Pods
	pods, err := kubernetes.PodList(client, ns)
	if err != nil {
		return nil, fmt.Errorf("pods: %w", err)
	}
	for _, p := range pods {
		workloads = append(workloads, kubernetes.PodToWorkload(p))
	}

	// Deployments
	deployments, err := kubernetes.DeploymentList(client, ns)
	if err != nil {
		return nil, fmt.Errorf("deployments: %w", err)
	}
	for _, d := range deployments {
		workloads = append(workloads, kubernetes.DeploymentToWorkload(d))
	}

	// Services
	services, err := kubernetes.ServiceList(client, ns)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	for _, svc := range services {
		workloads = append(workloads, kubernetes.ServiceToWorkload(svc))
	}

	return workloads, nil
}
