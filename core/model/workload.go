package model

type Workload struct {
	ID         string
	Name       string
	Namespace  string
	Type       string // pod, container, service, task
	Platform   string // kubernetes, docker, vm
	Containers []Container
	Metadata   Metadata
}
