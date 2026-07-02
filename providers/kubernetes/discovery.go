package kubernetes

import (
	"context"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// rawObjects holds every native object listed from the cluster, plus non-fatal
// per-resource-type warnings. ReplicaSets are listed only to resolve the
// Pod->ReplicaSet->Deployment ownership chain (see normalize.go) — they are never
// themselves returned as a scanned resource.
type rawObjects struct {
	pods         []corev1.Pod
	replicaSets  []appsv1.ReplicaSet
	deployments  []appsv1.Deployment
	statefulSets []appsv1.StatefulSet
	daemonSets   []appsv1.DaemonSet
	jobs         []batchv1.Job
	cronJobs     []batchv1.CronJob
	services     []corev1.Service
	warnings     []string
}

// discoverAll fans out one goroutine per resource type. A single resource type
// failing (e.g. RBAC forbids "list jobs") is collected as a warning, not a
// scan-aborting error — only a client-wide failure (every type fails) aborts.
func discoverAll(ctx context.Context, client kubernetes.Interface, namespace, labelSelector, fieldSelector string) (*rawObjects, error) {
	listOpts := metav1.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector}

	raw := &rawObjects{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	total := 0

	fetch := func(name string, fn func() error) {
		total++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				raw.warnings = append(raw.warnings, fmt.Sprintf("%s: %v", name, err))
				mu.Unlock()
			}
		}()
	}

	fetch("pods", func() error {
		list, err := client.CoreV1().Pods(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.pods = list.Items
		mu.Unlock()
		return nil
	})

	fetch("replicasets", func() error {
		list, err := client.AppsV1().ReplicaSets(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.replicaSets = list.Items
		mu.Unlock()
		return nil
	})

	fetch("deployments", func() error {
		list, err := client.AppsV1().Deployments(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.deployments = list.Items
		mu.Unlock()
		return nil
	})

	fetch("statefulsets", func() error {
		list, err := client.AppsV1().StatefulSets(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.statefulSets = list.Items
		mu.Unlock()
		return nil
	})

	fetch("daemonsets", func() error {
		list, err := client.AppsV1().DaemonSets(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.daemonSets = list.Items
		mu.Unlock()
		return nil
	})

	fetch("jobs", func() error {
		list, err := client.BatchV1().Jobs(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.jobs = list.Items
		mu.Unlock()
		return nil
	})

	fetch("cronjobs", func() error {
		list, err := client.BatchV1().CronJobs(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.cronJobs = list.Items
		mu.Unlock()
		return nil
	})

	fetch("services", func() error {
		list, err := client.CoreV1().Services(namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		mu.Lock()
		raw.services = list.Items
		mu.Unlock()
		return nil
	})

	wg.Wait()

	if len(raw.warnings) == total {
		return nil, fmt.Errorf("kubernetes: every resource type failed: %v", raw.warnings)
	}
	return raw, nil
}
