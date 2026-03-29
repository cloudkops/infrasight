package kubernetes

import (
	"testing"

	"github.com/cloudkops/infrasight/core/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ptrBool(b bool) *bool    { return &b }
func ptrInt64(i int64) *int64 { return &i }

func TestPodToWorkload_BasicMetadata(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:         types.UID("abc-123"),
			Name:        "my-pod",
			Namespace:   "production",
			Labels:      map[string]string{"team": "platform", "env": "production"},
			Annotations: map[string]string{"owner": "jane"},
		},
	}

	w := PodToWorkload(pod)

	if w.ID != "abc-123" {
		t.Errorf("ID = %q, want abc-123", w.ID)
	}
	if w.Name != "my-pod" {
		t.Errorf("Name = %q, want my-pod", w.Name)
	}
	if w.Namespace != "production" {
		t.Errorf("Namespace = %q, want production", w.Namespace)
	}
	if w.Type != "pod" {
		t.Errorf("Type = %q, want pod", w.Type)
	}
	if w.Platform != "kubernetes" {
		t.Errorf("Platform = %q, want kubernetes", w.Platform)
	}
	if w.Metadata.Labels["team"] != "platform" {
		t.Errorf("Labels[team] = %q, want platform", w.Metadata.Labels["team"])
	}
	if w.Metadata.Annotations["owner"] != "jane" {
		t.Errorf("Annotations[owner] = %q, want jane", w.Metadata.Annotations["owner"])
	}
}

func TestPodToWorkload_NoContainers(t *testing.T) {
	pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "empty"}}
	w := PodToWorkload(pod)
	if len(w.Containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(w.Containers))
	}
}

func TestPodToWorkload_SecurityContext_RootUser(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "root-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "app",
					Image: "nginx:latest",
					SecurityContext: &v1.SecurityContext{
						RunAsUser:  ptrInt64(0),
						Privileged: ptrBool(false),
					},
				},
			},
		},
	}

	w := PodToWorkload(pod)
	if len(w.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(w.Containers))
	}
	c := w.Containers[0]
	if c.User != 0 {
		t.Errorf("User = %d, want 0", c.User)
	}
	if c.Privileged {
		t.Error("Privileged = true, want false")
	}
}

func TestPodToWorkload_SecurityContext_NonRootUser(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nonroot-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "app",
					Image: "alpine:3.18",
					SecurityContext: &v1.SecurityContext{
						RunAsUser:  ptrInt64(1000),
						Privileged: ptrBool(false),
					},
				},
			},
		},
	}

	w := PodToWorkload(pod)
	c := w.Containers[0]
	if c.User != 1000 {
		t.Errorf("User = %d, want 1000", c.User)
	}
}

func TestPodToWorkload_SecurityContext_Privileged(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "priv-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "app",
					SecurityContext: &v1.SecurityContext{
						Privileged: ptrBool(true),
					},
				},
			},
		},
	}

	w := PodToWorkload(pod)
	if !w.Containers[0].Privileged {
		t.Error("Privileged = false, want true")
	}
}

func TestPodToWorkload_NoSecurityContext_Defaults(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "default-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "app", Image: "busybox"},
			},
		},
	}

	w := PodToWorkload(pod)
	c := w.Containers[0]
	if c.User != 0 {
		t.Errorf("default User = %d, want 0", c.User)
	}
	if c.Privileged {
		t.Error("default Privileged should be false")
	}
}

func TestPodToWorkload_Resources(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "res-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "app",
					Image: "app:v1",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("250m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("500m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	w := PodToWorkload(pod)
	r := w.Containers[0].Resources

	if r.CPURequest == "" {
		t.Error("CPURequest should not be empty")
	}
	if r.MemoryRequest == "" {
		t.Error("MemoryRequest should not be empty")
	}
	if r.CPULimit == "" {
		t.Error("CPULimit should not be empty")
	}
	if r.MemoryLimit == "" {
		t.Error("MemoryLimit should not be empty")
	}
}

func TestPodToWorkload_NoResources_EmptyStrings(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nores-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "app"},
			},
		},
	}

	w := PodToWorkload(pod)
	r := w.Containers[0].Resources
	if r.CPULimit != "" || r.MemoryLimit != "" || r.CPURequest != "" || r.MemoryRequest != "" {
		t.Errorf("expected all resource fields empty without resource spec, got %+v", r)
	}
}

func TestPodToWorkload_HostNetwork(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "hostnet-pod"},
		Spec: v1.PodSpec{
			HostNetwork: true,
			Containers:  []v1.Container{{Name: "app"}},
		},
	}

	w := PodToWorkload(pod)
	exposures := w.Containers[0].Exposures

	found := false
	for _, e := range exposures {
		if e.Type == "hostNetwork" {
			found = true
		}
	}
	if !found {
		t.Error("expected hostNetwork in exposures")
	}
}

func TestPodToWorkload_NoHostNetwork(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "normal-pod"},
		Spec: v1.PodSpec{
			HostNetwork: false,
			Containers:  []v1.Container{{Name: "app"}},
		},
	}

	w := PodToWorkload(pod)
	for _, e := range w.Containers[0].Exposures {
		if e.Type == "hostNetwork" {
			t.Error("unexpected hostNetwork exposure on non-host-network pod")
		}
	}
}

func TestPodToWorkload_ContainerPorts(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ports-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "app",
					Ports: []v1.ContainerPort{
						{ContainerPort: 8080},
						{ContainerPort: 9090},
					},
				},
			},
		},
	}

	w := PodToWorkload(pod)
	ports := []int32{}
	for _, e := range w.Containers[0].Exposures {
		if e.Type == "port" {
			ports = append(ports, e.Port)
		}
	}
	if len(ports) != 2 {
		t.Errorf("expected 2 port exposures, got %d", len(ports))
	}
}

func TestPodToWorkload_MultipleContainers(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "main", Image: "app:v1", SecurityContext: &v1.SecurityContext{RunAsUser: ptrInt64(1000)}},
				{Name: "sidecar", Image: "envoy:v2", SecurityContext: &v1.SecurityContext{RunAsUser: ptrInt64(0)}},
			},
		},
	}

	w := PodToWorkload(pod)
	if len(w.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(w.Containers))
	}

	byName := map[string]model.Container{}
	for _, c := range w.Containers {
		byName[c.Name] = c
	}

	if byName["main"].User != 1000 {
		t.Errorf("main container user = %d, want 1000", byName["main"].User)
	}
	if byName["sidecar"].User != 0 {
		t.Errorf("sidecar container user = %d, want 0", byName["sidecar"].User)
	}
}
