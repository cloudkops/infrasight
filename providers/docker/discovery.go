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
	failed := 0

	// failed counts only whole-resource-type failures (a List call itself erroring),
	// never per-item inspect warnings — those are still visible via raw.warnings but
	// must not trip the "every resource type failed" gate below.
	fetch := func(name string, fn func() error) {
		total++
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				failed++
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
		containers, warnings := getAllContainers(ctx, c, list)
		mu.Lock()
		raw.containers = containers
		raw.warnings = append(raw.warnings, warnings...)
		mu.Unlock()
		return nil
	})

	fetch("images", func() error {
		list, err := c.ImageList(ctx, image.ListOptions{All: true})
		if err != nil {
			return err
		}
		images, warnings := getAllImages(ctx, c, list)
		mu.Lock()
		raw.images = images
		raw.warnings = append(raw.warnings, warnings...)
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

	if failed == total {
		return nil, fmt.Errorf("docker: every resource type failed: %v", raw.warnings)
	}
	return raw, nil
}

// inspectConcurrency bounds how many ContainerInspect/ImageInspect calls run at
// once — discoverAll's own fetch() only parallelizes across the 4 resource *types*;
// without this, inspecting each item of a given type would still happen one at a
// time, serially, which doesn't scale past a handful of containers/images.
const inspectConcurrency = 8

func getAllContainers(ctx context.Context, c *client.Client, cs []container.Summary) ([]container.InspectResponse, []string) {
	results := make([]container.InspectResponse, len(cs))
	ok := make([]bool, len(cs))
	var warnings []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, inspectConcurrency)

	for i, summary := range cs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, id string) {
			defer wg.Done()
			defer func() { <-sem }()
			inspect, err := c.ContainerInspect(ctx, id)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("container %s: %v", id, err))
				mu.Unlock()
				return
			}
			results[i] = inspect
			ok[i] = true
		}(i, summary.ID)
	}
	wg.Wait()

	out := make([]container.InspectResponse, 0, len(cs))
	for i, present := range ok {
		if present {
			out = append(out, results[i])
		}
	}
	return out, warnings
}

func getAllImages(ctx context.Context, c *client.Client, imgs []image.Summary) ([]image.InspectResponse, []string) {
	results := make([]image.InspectResponse, len(imgs))
	ok := make([]bool, len(imgs))
	var warnings []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, inspectConcurrency)

	for i, img := range imgs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, id string) {
			defer wg.Done()
			defer func() { <-sem }()
			inspect, err := c.ImageInspect(ctx, id)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("image %s: %v", id, err))
				mu.Unlock()
				return
			}
			results[i] = inspect
			ok[i] = true
		}(i, img.ID)
	}
	wg.Wait()

	out := make([]image.InspectResponse, 0, len(imgs))
	for i, present := range ok {
		if present {
			out = append(out, results[i])
		}
	}
	return out, warnings
}
