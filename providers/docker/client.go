package docker

import "github.com/docker/docker/client"

// newClient builds a Docker client from the environment (DOCKER_HOST, DOCKER_TLS_VERIFY,
// ...), or from an explicit host if dockerHost is set — mirroring how the Kubernetes
// provider's newClient prefers an explicit --kubeconfig/--context over ambient config.
func newClient(dockerHost string) (*client.Client, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if dockerHost != "" {
		opts = append(opts, client.WithHost(dockerHost))
	}
	return client.NewClientWithOpts(opts...)
}
