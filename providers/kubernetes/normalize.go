package kubernetes

import (
	"fmt"

	"github.com/cloudkops/infrasight/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NormalizeAll maps every raw native object into resource.Resource. Pod owner
// references pointing at a ReplicaSet are resolved one hop further to that
// ReplicaSet's own owner (usually a Deployment) — ReplicaSets are never returned as
// resources themselves, so without this resolution internal/scan/pipeline's
// Owner-based dedup would have nothing to match against.
func NormalizeAll(raw *rawObjects) []resource.Resource {
	rsOwner := make(map[string]*resource.Owner, len(raw.replicaSets))
	for _, rs := range raw.replicaSets {
		rsOwner[rs.Namespace+"/"+rs.Name] = mapOwner(rs.OwnerReferences)
	}

	resolveOwner := func(namespace string, refs []metav1.OwnerReference) *resource.Owner {
		owner := mapOwner(refs)
		if owner != nil && owner.Kind == "ReplicaSet" {
			if resolved, ok := rsOwner[namespace+"/"+owner.Name]; ok && resolved != nil {
				return resolved
			}
		}
		return owner
	}

	var resources []resource.Resource
	for _, pod := range raw.pods {
		resources = append(resources, mapPod(pod, resolveOwner))
	}
	for _, d := range raw.deployments {
		resources = append(resources, mapDeployment(d, resolveOwner))
	}
	for _, s := range raw.statefulSets {
		resources = append(resources, mapStatefulSet(s, resolveOwner))
	}
	for _, d := range raw.daemonSets {
		resources = append(resources, mapDaemonSet(d, resolveOwner))
	}
	for _, j := range raw.jobs {
		resources = append(resources, mapJob(j, resolveOwner))
	}
	for _, c := range raw.cronJobs {
		resources = append(resources, mapCronJob(c, resolveOwner))
	}
	for _, svc := range raw.services {
		resources = append(resources, mapService(svc))
	}
	return resources
}

type ownerResolver func(namespace string, refs []metav1.OwnerReference) *resource.Owner

func mapPod(pod corev1.Pod, resolveOwner ownerResolver) resource.Resource {
	runtime := mapPodSpec(pod.Spec)
	return resource.Resource{
		ID:         resourceID("pod", pod.UID, pod.Namespace, pod.Name),
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Kind:       "pod",
		Provider:   Name,
		Metadata:   mapMetadata(pod.Labels, pod.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(pod.Spec),
		Runtime:    runtime,
		Owner:      resolveOwner(pod.Namespace, pod.OwnerReferences),
		Attributes: podLevelAttributes(pod.Spec),
	}
}

func mapDeployment(d appsv1.Deployment, resolveOwner ownerResolver) resource.Resource {
	runtime := mapPodSpec(d.Spec.Template.Spec)
	return resource.Resource{
		ID:         resourceID("deployment", d.UID, d.Namespace, d.Name),
		Name:       d.Name,
		Namespace:  d.Namespace,
		Kind:       "deployment",
		Provider:   Name,
		Metadata:   mapMetadata(d.Labels, d.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(d.Spec.Template.Spec),
		Runtime:    runtime,
		Owner:      resolveOwner(d.Namespace, d.OwnerReferences),
		Attributes: podLevelAttributes(d.Spec.Template.Spec),
	}
}

func mapStatefulSet(s appsv1.StatefulSet, resolveOwner ownerResolver) resource.Resource {
	runtime := mapPodSpec(s.Spec.Template.Spec)
	return resource.Resource{
		ID:         resourceID("statefulset", s.UID, s.Namespace, s.Name),
		Name:       s.Name,
		Namespace:  s.Namespace,
		Kind:       "statefulset",
		Provider:   Name,
		Metadata:   mapMetadata(s.Labels, s.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(s.Spec.Template.Spec),
		Runtime:    runtime,
		Owner:      resolveOwner(s.Namespace, s.OwnerReferences),
		Attributes: podLevelAttributes(s.Spec.Template.Spec),
	}
}

func mapDaemonSet(d appsv1.DaemonSet, resolveOwner ownerResolver) resource.Resource {
	runtime := mapPodSpec(d.Spec.Template.Spec)
	return resource.Resource{
		ID:         resourceID("daemonset", d.UID, d.Namespace, d.Name),
		Name:       d.Name,
		Namespace:  d.Namespace,
		Kind:       "daemonset",
		Provider:   Name,
		Metadata:   mapMetadata(d.Labels, d.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(d.Spec.Template.Spec),
		Runtime:    runtime,
		Owner:      resolveOwner(d.Namespace, d.OwnerReferences),
		Attributes: podLevelAttributes(d.Spec.Template.Spec),
	}
}

func mapJob(j batchv1.Job, resolveOwner ownerResolver) resource.Resource {
	runtime := mapPodSpec(j.Spec.Template.Spec)
	return resource.Resource{
		ID:         resourceID("job", j.UID, j.Namespace, j.Name),
		Name:       j.Name,
		Namespace:  j.Namespace,
		Kind:       "job",
		Provider:   Name,
		Metadata:   mapMetadata(j.Labels, j.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(j.Spec.Template.Spec),
		Runtime:    runtime,
		Owner:      resolveOwner(j.Namespace, j.OwnerReferences),
		Attributes: podLevelAttributes(j.Spec.Template.Spec),
	}
}

func mapCronJob(c batchv1.CronJob, resolveOwner ownerResolver) resource.Resource {
	podSpec := c.Spec.JobTemplate.Spec.Template.Spec
	runtime := mapPodSpec(podSpec)
	return resource.Resource{
		ID:         resourceID("cronjob", c.UID, c.Namespace, c.Name),
		Name:       c.Name,
		Namespace:  c.Namespace,
		Kind:       "cronjob",
		Provider:   Name,
		Metadata:   mapMetadata(c.Labels, c.Annotations),
		Security:   aggregateSecurity(runtime.Containers),
		Networking: mapPodNetworking(podSpec),
		Runtime:    runtime,
		Owner:      resolveOwner(c.Namespace, c.OwnerReferences),
		Attributes: podLevelAttributes(podSpec),
	}
}

func mapService(svc corev1.Service) resource.Resource {
	return resource.Resource{
		ID:         resourceID("service", svc.UID, svc.Namespace, svc.Name),
		Name:       svc.Name,
		Namespace:  svc.Namespace,
		Kind:       "service",
		Provider:   Name,
		Metadata:   mapMetadata(svc.Labels, svc.Annotations),
		Networking: mapServiceNetworking(svc),
	}
}

// mapPodSpec maps regular, init, and ephemeral containers into resource.Runtime.
// Init/ephemeral containers are tagged via Attributes["container_kind"] so rules
// can still distinguish them without a dedicated field.
func mapPodSpec(spec corev1.PodSpec) resource.Runtime {
	var containers []resource.Container
	for _, c := range spec.InitContainers {
		containers = append(containers, mapContainer(c, "init"))
	}
	for _, c := range spec.Containers {
		containers = append(containers, mapContainer(c, ""))
	}
	for _, ec := range spec.EphemeralContainers {
		containers = append(containers, mapContainer(corev1.Container(ec.EphemeralContainerCommon), "ephemeral"))
	}
	return resource.Runtime{Containers: containers}
}

func mapContainer(c corev1.Container, kindAttr string) resource.Container {
	sec := resource.SecurityContext{}
	var user int64

	if sc := c.SecurityContext; sc != nil {
		if sc.RunAsUser != nil {
			user = *sc.RunAsUser
		}
		switch {
		case sc.RunAsUser != nil && *sc.RunAsUser == 0:
			// Explicit UID 0 is root regardless of RunAsNonRoot — the two fields
			// are independent and RunAsUser is the more direct signal.
			sec.RunAsRoot = true
		case sc.RunAsNonRoot != nil:
			sec.RunAsRoot = !*sc.RunAsNonRoot
		}
		sec.AllowPrivilegeEscalation = sc.AllowPrivilegeEscalation
		sec.ReadOnlyRootFilesystem = sc.ReadOnlyRootFilesystem
		if sc.Privileged != nil {
			sec.Privileged = *sc.Privileged
		}
		if sc.Capabilities != nil {
			for _, a := range sc.Capabilities.Add {
				sec.CapabilitiesAdd = append(sec.CapabilitiesAdd, string(a))
			}
			for _, d := range sc.Capabilities.Drop {
				sec.CapabilitiesDrop = append(sec.CapabilitiesDrop, string(d))
			}
		}
	}

	var attrs map[string]any
	if kindAttr != "" {
		attrs = map[string]any{"container_kind": kindAttr}
	}

	return resource.Container{
		Name:       c.Name,
		Image:      c.Image,
		User:       user,
		Privileged: sec.Privileged,
		Limits:     mapLimits(c.Resources),
		Security:   sec,
		Attributes: attrs,
	}
}

func mapLimits(r corev1.ResourceRequirements) resource.Limits {
	return resource.Limits{
		CPURequest:    quantityString(r.Requests, corev1.ResourceCPU),
		CPULimit:      quantityString(r.Limits, corev1.ResourceCPU),
		MemoryRequest: quantityString(r.Requests, corev1.ResourceMemory),
		MemoryLimit:   quantityString(r.Limits, corev1.ResourceMemory),
	}
}

func quantityString(list corev1.ResourceList, name corev1.ResourceName) string {
	if list == nil {
		return ""
	}
	q, ok := list[name]
	if !ok {
		return ""
	}
	return q.String()
}

// aggregateSecurity gives resource-level rules (resource.security.*) a
// conservative cross-container view: privileged/root if any container is.
func aggregateSecurity(containers []resource.Container) resource.SecurityContext {
	var sec resource.SecurityContext
	for _, c := range containers {
		if c.Security.Privileged {
			sec.Privileged = true
		}
		if c.Security.RunAsRoot {
			sec.RunAsRoot = true
		}
	}
	return sec
}

func mapPodNetworking(spec corev1.PodSpec) resource.Networking {
	if !spec.HostNetwork {
		return resource.Networking{}
	}
	return resource.Networking{Exposures: []resource.Exposure{{Type: "hostNetwork"}}}
}

func mapServiceNetworking(svc corev1.Service) resource.Networking {
	var exposures []resource.Exposure
	switch svc.Spec.Type {
	case corev1.ServiceTypeNodePort:
		for _, p := range svc.Spec.Ports {
			exposures = append(exposures, resource.Exposure{Type: "NodePort", Port: p.NodePort})
		}
	case corev1.ServiceTypeLoadBalancer:
		for _, p := range svc.Spec.Ports {
			exposures = append(exposures, resource.Exposure{Type: "LoadBalancer", Port: p.Port})
		}
	}
	return resource.Networking{Exposures: exposures}
}

// podLevelAttributes uses the exact dotted field name a rule would reference
// (e.g. "resource.host_pid") as the map key, so it's queryable through
// internal/rule/evaluator's plain Attributes fallback with no dedicated
// resolveField case — hostNetwork itself is covered separately by
// Networking.Flatten via EnrichAttributes, so it isn't duplicated here.
func podLevelAttributes(spec corev1.PodSpec) map[string]any {
	return map[string]any{
		"resource.host_pid": spec.HostPID,
		"resource.host_ipc": spec.HostIPC,
	}
}

func mapMetadata(labels, annotations map[string]string) resource.Metadata {
	if labels == nil {
		labels = map[string]string{}
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	return resource.Metadata{Labels: labels, Annotations: annotations}
}

func mapOwner(refs []metav1.OwnerReference) *resource.Owner {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return &resource.Owner{Kind: ref.Kind, Name: ref.Name}
		}
	}
	if len(refs) > 0 {
		return &resource.Owner{Kind: refs[0].Kind, Name: refs[0].Name}
	}
	return nil
}

func resourceID(kind string, uid types.UID, namespace, name string) string {
	if uid != "" {
		return string(uid)
	}
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}
