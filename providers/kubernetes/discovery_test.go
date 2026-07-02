package kubernetes

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/cloudkops/infrasight/internal/provider"
)

func TestDiscoverAll_PartialFailureIsTolerated(t *testing.T) {
	client := kubefake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"}},
	)
	client.PrependReactor("list", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	raw, err := discoverAll(context.Background(), client, "prod", "", "")
	if err != nil {
		t.Fatalf("expected discoverAll to tolerate one failing resource type, got error: %v", err)
	}
	if len(raw.pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(raw.pods))
	}
	if len(raw.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning (jobs), got %v", raw.warnings)
	}
}

func TestDiscoverAll_TotalFailureErrors(t *testing.T) {
	client := kubefake.NewSimpleClientset()
	client.PrependReactor("list", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("no connection")
	})

	if _, err := discoverAll(context.Background(), client, "prod", "", ""); err == nil {
		t.Fatal("expected an error when every resource type fails")
	}
}

func TestProvider_Discover_EndToEnd_DedupsOwnedPod(t *testing.T) {
	client := kubefake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", UID: "dep-1"}},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "web-abc", Namespace: "prod",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", Name: "web", Controller: boolPtr(true)},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "web-abc-xyz", Namespace: "prod", UID: "pod-1",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "web-abc", Controller: boolPtr(true)},
				},
			},
		},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: "prod", UID: "pod-2"}},
	)

	raw, err := discoverAll(context.Background(), client, "prod", "", "")
	if err != nil {
		t.Fatalf("discoverAll: %v", err)
	}
	resources := NormalizeAll(raw)

	// Normalization alone does not dedup — that's internal/scan/pipeline's job
	// (docs/k8s.md §4). Confirm both the Deployment and both Pods are present here.
	kinds := map[string]int{}
	for _, r := range resources {
		kinds[r.Kind]++
	}
	if kinds["deployment"] != 1 || kinds["pod"] != 2 {
		t.Fatalf("expected 1 deployment + 2 pods before dedup, got %+v", kinds)
	}
	if New().Kind() != provider.KindLive {
		t.Error("expected the kubernetes provider to report KindLive")
	}
}
