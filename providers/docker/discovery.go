package docker

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type rawObjects struct {
	containers []container.InspectResponse
	images     []image.InspectResponse
	volumes    []volume.Volume
	networks   []network.Inspect
	warnings   []string
}

func discoverAll(ctx context.Context, c *client.Client) (*rawObjects, error) {
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

	fetch("containers", func() error {
		list, err := c.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return err
		}
		containers := getAllContainers(ctx, c, list)
		mu.Lock()
		raw.containers = containers
		mu.Unlock()
		return nil
	})

	fetch("images", func() error {
		list, err := c.ImageList(ctx, image.ListOptions{All: true})
		if err != nil {
			return err
		}
		images := getAllImages(ctx, c, list)
		mu.Lock()
		raw.images = images
		mu.Unlock()
		return nil
	})

	fetch("volumes", func() error {
		resp, err := c.VolumeList(ctx, volume.ListOptions{})
		if err != nil {
			return err
		}
		mu.Lock()
		for _, v := range resp.Volumes {
			if v != nil {
				raw.volumes = append(raw.volumes, *v)
			}
		}
		mu.Unlock()
		return nil
	})

	fetch("networks", func() error {
		list, err := c.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return err
		}
		mu.Lock()
		raw.networks = list
		mu.Unlock()
		return nil
	})

	wg.Wait()

	if len(raw.warnings) == total {
		return nil, fmt.Errorf("docker: every resource type failed: %v", raw.warnings)
	}
	return raw, nil
}

func getAllContainers(ctx context.Context, c *client.Client, cs []container.Summary) []container.InspectResponse {
	var cr []container.InspectResponse
	for _, summary := range cs {
		inspect, err := c.ContainerInspect(ctx, summary.ID)
		if err != nil {
			continue
		}
		cr = append(cr, inspect)
	}
	return cr
}

func getAllImages(ctx context.Context, c *client.Client, imgs []image.Summary) []image.InspectResponse {
	var ir []image.InspectResponse
	for _, img := range imgs {
		inspect, _, err := c.ImageInspectWithRaw(ctx, img.ID)
		if err != nil {
			continue
		}
		ir = append(ir, inspect)
	}
	return ir
}
