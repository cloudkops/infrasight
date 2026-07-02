package kubernetes

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infraresource "github.com/cloudkops/infrasight/internal/resource"
)

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }

func TestMapPod_SecurityContextAndResources(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-abc123", Namespace: "prod", UID: "pod-uid-1",
			Labels: map[string]string{"app": "web"},
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  int64Ptr(0),
						Privileged: boolPtr(true),
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: k8sresource.MustParse("500m"),
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{Name: "init-setup", Image: "busybox"},
			},
		},
	}

	got := mapPod(pod, func(string, []metav1.OwnerReference) *infraresource.Owner { return nil })

	if got.ID != "pod-uid-1" || got.Name != "web-abc123" || got.Namespace != "prod" || got.Kind != "pod" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.Provider != Name {
		t.Errorf("expected provider %q, got %q", Name, got.Provider)
	}
	if !got.Networking.HasExposure("hostNetwork") {
		t.Error("expected hostNetwork exposure")
	}
	if len(got.Runtime.Containers) != 2 {
		t.Fatalf("expected 2 containers (init + main), got %d", len(got.Runtime.Containers))
	}

	init := got.Runtime.Containers[0]
	if init.Attributes["container_kind"] != "init" {
		t.Errorf("expected first container to be tagged init, got %+v", init.Attributes)
	}

	main := got.Runtime.Containers[1]
	if main.User != 0 {
		t.Errorf("expected root user, got %d", main.User)
	}
	if !main.Privileged {
		t.Error("expected privileged container")
	}
	if main.Limits.CPULimit != "500m" {
		t.Errorf("expected cpu limit 500m, got %q", main.Limits.CPULimit)
	}
	if main.Limits.MemoryLimit != "" {
		t.Errorf("expected no memory limit, got %q", main.Limits.MemoryLimit)
	}
	if !got.Security.Privileged || !got.Security.RunAsRoot {
		t.Errorf("expected resource-level security to aggregate from containers: %+v", got.Security)
	}
}

func TestMapService_NodePortExposure(t *testing.T) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "svc-uid-1"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{NodePort: 30080}},
		},
	}

	got := mapService(svc)
	if got.Kind != "service" {
		t.Fatalf("expected kind service, got %q", got.Kind)
	}
	if !got.Networking.HasExposure("NodePort") {
		t.Error("expected NodePort exposure")
	}
}

func TestNormalizeAll_ResolvesDeploymentOwnershipThroughReplicaSet(t *testing.T) {
	raw := &rawObjects{
		deployments: []appsv1.Deployment{
			{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "dep-1"}},
		},
		replicaSets: []appsv1.ReplicaSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "web-abc", Namespace: "prod",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "Deployment", Name: "web", Controller: boolPtr(true)},
					},
				},
			},
		},
		pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "web-abc-xyz", Namespace: "prod", UID: "pod-1",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "ReplicaSet", Name: "web-abc", Controller: boolPtr(true)},
					},
				},
			},
		},
	}

	resources := NormalizeAll(raw)

	var pod *infraresource.Resource
	for i := range resources {
		if resources[i].Kind == "pod" {
			pod = &resources[i]
		}
	}
	if pod == nil {
		t.Fatal("expected a pod resource in the result")
	}
	if pod.Owner == nil || pod.Owner.Kind != "Deployment" || pod.Owner.Name != "web" {
		t.Errorf("expected pod owner resolved to Deployment/web, got %+v", pod.Owner)
	}
}
