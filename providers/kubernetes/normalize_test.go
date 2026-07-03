package kubernetes

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

func TestMapConfigMap_NoDataValuesLeakIntoAttributes(t *testing.T) {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "prod", UID: "cm-1"},
		Data:       map[string]string{"log_level": "debug"},
		Immutable:  boolPtr(true),
	}

	got := mapConfigMap(cm)

	if got.Kind != "configmap" || got.Provider != Name {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.Attributes["configmap.data_keys_count"] != 1 {
		t.Errorf("expected data_keys_count=1, got %v", got.Attributes["configmap.data_keys_count"])
	}
	if got.Attributes["configmap.immutable"] != true {
		t.Errorf("expected immutable=true, got %v", got.Attributes["configmap.immutable"])
	}
	for k, v := range got.Attributes {
		if k == "log_level" || v == "debug" {
			t.Fatalf("configmap data value leaked into attributes: %+v", got.Attributes)
		}
	}
}

func TestMapSecret_NoDataValuesLeakIntoAttributes(t *testing.T) {
	sec := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "prod", UID: "sec-1"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}

	got := mapSecret(sec)

	if got.Kind != "secret" || got.Provider != Name {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.Attributes["secret.type"] != "Opaque" {
		t.Errorf("expected secret.type=Opaque, got %v", got.Attributes["secret.type"])
	}
	if got.Attributes["secret.data_keys_count"] != 1 {
		t.Errorf("expected data_keys_count=1, got %v", got.Attributes["secret.data_keys_count"])
	}
	for k, v := range got.Attributes {
		if k == "password" || v == "hunter2" {
			t.Fatalf("secret data value leaked into attributes: %+v", got.Attributes)
		}
	}
}

func TestMapIngress_ExposureAndAttributes(t *testing.T) {
	className := "nginx"
	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "ing-1"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules:            []networkingv1.IngressRule{{Host: "example.com"}},
			TLS:              []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
		},
	}

	got := mapIngress(ing)

	if got.Kind != "ingress" {
		t.Fatalf("expected kind ingress, got %q", got.Kind)
	}
	if !got.Networking.HasExposure("Ingress") {
		t.Error("expected Ingress exposure")
	}
	if got.Attributes["ingress.tls_enabled"] != true {
		t.Errorf("expected tls_enabled=true, got %v", got.Attributes["ingress.tls_enabled"])
	}
	if got.Attributes["ingress.class"] != "nginx" {
		t.Errorf("expected class=nginx, got %v", got.Attributes["ingress.class"])
	}
}

func TestMapNetworkPolicy_SelectsAllPodsAndRuleCounts(t *testing.T) {
	np := networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: "prod", UID: "np-1"},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}

	got := mapNetworkPolicy(np)

	if got.Kind != "networkpolicy" {
		t.Fatalf("expected kind networkpolicy, got %q", got.Kind)
	}
	if got.Attributes["networkpolicy.selects_all_pods"] != true {
		t.Errorf("expected selects_all_pods=true, got %v", got.Attributes["networkpolicy.selects_all_pods"])
	}
	if got.Attributes["networkpolicy.ingress_rules_count"] != 0 || got.Attributes["networkpolicy.egress_rules_count"] != 0 {
		t.Errorf("expected zero ingress/egress rules for a default-deny policy, got %+v", got.Attributes)
	}
}

func TestMapPod_ProbesHostPathAutomountAndSecretEnv(t *testing.T) {
	falseVal := false
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "pod-1"},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &falseVal,
			Volumes: []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib"}}},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					Env: []corev1.EnvVar{
						{Name: "DB_PASSWORD", ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{Key: "password"},
						}},
					},
					// No ReadinessProbe/LivenessProbe set.
				},
			},
		},
	}

	got := mapPod(pod, func(string, []metav1.OwnerReference) *infraresource.Owner { return nil })

	if got.Attributes["resource.host_path_volume"] != true {
		t.Errorf("expected host_path_volume=true, got %v", got.Attributes["resource.host_path_volume"])
	}
	if got.Attributes["resource.service_account_token_automount"] != false {
		t.Errorf("expected service_account_token_automount=false, got %v", got.Attributes["resource.service_account_token_automount"])
	}

	c := got.Runtime.Containers[0]
	if c.Attributes["container.probes.readiness_configured"] != false {
		t.Errorf("expected readiness_configured=false, got %v", c.Attributes["container.probes.readiness_configured"])
	}
	if c.Attributes["container.probes.liveness_configured"] != false {
		t.Errorf("expected liveness_configured=false, got %v", c.Attributes["container.probes.liveness_configured"])
	}
	if c.Attributes["container.secret_env_vars"] != true {
		t.Errorf("expected secret_env_vars=true, got %v", c.Attributes["container.secret_env_vars"])
	}
}

func TestMapPod_AutomountDefaultsTrueWhenUnset(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "pod-1"},
	}
	got := mapPod(pod, func(string, []metav1.OwnerReference) *infraresource.Owner { return nil })
	if got.Attributes["resource.service_account_token_automount"] != true {
		t.Errorf("expected automount to default true when unset, got %v", got.Attributes["resource.service_account_token_automount"])
	}
	if got.Attributes["resource.host_path_volume"] != false {
		t.Errorf("expected host_path_volume=false with no volumes, got %v", got.Attributes["resource.host_path_volume"])
	}
}

func TestMapClusterRoleBinding_DetectsClusterAdmin(t *testing.T) {
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "dangerous-binding", UID: "crb-1"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: "prod"}},
	}

	got := mapClusterRoleBinding(crb)

	if got.Kind != "clusterrolebinding" {
		t.Fatalf("expected kind clusterrolebinding, got %q", got.Kind)
	}
	if got.Attributes["rbac.binds_cluster_admin"] != true {
		t.Errorf("expected binds_cluster_admin=true, got %v", got.Attributes["rbac.binds_cluster_admin"])
	}
	if got.Attributes["rbac.subjects_count"] != 1 {
		t.Errorf("expected subjects_count=1, got %v", got.Attributes["rbac.subjects_count"])
	}
}

func TestMapClusterRoleBinding_NonAdminRoleNotFlagged(t *testing.T) {
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "view-binding", UID: "crb-2"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
	}

	got := mapClusterRoleBinding(crb)
	if got.Attributes["rbac.binds_cluster_admin"] != false {
		t.Errorf("expected binds_cluster_admin=false for the view role, got %v", got.Attributes["rbac.binds_cluster_admin"])
	}
}

func TestMapRoleBinding_DetectsClusterAdminClusterRole(t *testing.T) {
	rb := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-admin-binding", Namespace: "prod", UID: "rb-1"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
	}

	got := mapRoleBinding(rb)

	if got.Kind != "rolebinding" || got.Namespace != "prod" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.Attributes["rbac.binds_cluster_admin"] != true {
		t.Errorf("expected binds_cluster_admin=true, got %v", got.Attributes["rbac.binds_cluster_admin"])
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
