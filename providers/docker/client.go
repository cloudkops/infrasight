package docker

import "github.com/docker/docker/client"

func newClient() (*client.Client, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return c, nil
}
