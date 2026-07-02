package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// newClient builds a Kubernetes client: in-cluster config first, falling back to an
// explicit --kubeconfig path or the default ~/.kube/config, with --context applied
// if set. Returns kubernetes.Interface (not the concrete *Clientset) so tests can
// substitute a fake clientset.
func newClient(kubeconfigPath, kubeContext string) (kubernetes.Interface, error) {
	config, err := loadConfig(kubeconfigPath, kubeContext)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func loadConfig(kubeconfigPath, kubeContext string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes: load kubeconfig: %w", err)
	}
	return cfg, nil
}
