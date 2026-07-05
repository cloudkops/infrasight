package docker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudkops/infrasight/internal/resource"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
)

func NormalizeAll(raw *rawObjects) []resource.Resource {
	var resources []resource.Resource
	for i := range raw.containers {
		resources = append(resources, mapContainer(&raw.containers[i]))
	}
	for i := range raw.images {
		resources = append(resources, mapImage(&raw.images[i]))
	}
	for i := range raw.volumes {
		resources = append(resources, mapVolume(&raw.volumes[i]))
	}
	for i := range raw.networks {
		resources = append(resources, mapNetwork(&raw.networks[i]))
	}
	return resources
}

func mapContainer(c *container.InspectResponse) resource.Resource {
	sec := mapContainerSecurity(c)
	exposures := mapContainerExposures(c)
	attrs := mapContainerAttributes(c)
	owner := mapOwner(c)

	return resource.Resource{
		ID:       c.ID[:12],
		Name:     strings.TrimPrefix(c.Name, "/"),
		Kind:     "container",
		Provider: Name,
		Metadata: resource.Metadata{
			Labels: c.Config.Labels,
		},
		Security:   sec,
		Networking: resource.Networking{Exposures: exposures},
		Runtime:    resource.Runtime{},
		Owner:      owner,
		Attributes: attrs,
	}
}

func mapImage(img *image.InspectResponse) resource.Resource {
	attrs := mapImageAttributes(img)

	return resource.Resource{
		ID:       img.ID[:12],
		Name:     imageName(img),
		Kind:     "image",
		Provider: Name,
		Metadata: resource.Metadata{
			Labels: img.Config.Labels,
		},
		Attributes: attrs,
	}
}

func mapVolume(vol *volume.Volume) resource.Resource {
	attrs := map[string]any{
		"volume.driver":     vol.Driver,
		"volume.scope":      vol.Scope,
		"volume.mountpoint": vol.Mountpoint,
		"volume.name":       vol.Name,
	}
	if vol.UsageData != nil {
		attrs["volume.size_bytes"] = vol.UsageData.Size
		attrs["volume.ref_count"] = vol.UsageData.RefCount
	}

	return resource.Resource{
		ID:         vol.Name,
		Name:       vol.Name,
		Kind:       "volume",
		Provider:   Name,
		Metadata:   resource.Metadata{Labels: vol.Labels},
		Attributes: attrs,
	}
}

func mapNetwork(net *network.Inspect) resource.Resource {
	attrs := map[string]any{
		"network.driver":           net.Driver,
		"network.scope":            net.Scope,
		"network.internal":         net.Internal,
		"network.attachable":       net.Attachable,
		"network.ingress":          net.Ingress,
		"network.enable_ipv4":      net.EnableIPv4,
		"network.enable_ipv6":      net.EnableIPv6,
		"network.containers_count": len(net.Containers),
	}

	return resource.Resource{
		ID:         net.ID[:12],
		Name:       net.Name,
		Kind:       "network",
		Provider:   Name,
		Metadata:   resource.Metadata{Labels: net.Labels},
		Attributes: attrs,
	}
}

func mapContainerSecurity(c *container.InspectResponse) resource.SecurityContext {
	runAsRoot := false
	if c.Config.User == "" || c.Config.User == "0" || c.Config.User == "root" {
		runAsRoot = true
	} else if parts := strings.SplitN(c.Config.User, ":", 2); len(parts) > 0 {
		if uid, err := strconv.ParseInt(parts[0], 10, 64); err == nil && uid == 0 {
			runAsRoot = true
		} else if parts[0] == "root" {
			runAsRoot = true
		}
	}

	return resource.SecurityContext{
		RunAsRoot:                runAsRoot,
		Privileged:               c.HostConfig.Privileged,
		AllowPrivilegeEscalation: nil,
		ReadOnlyRootFilesystem:   &c.HostConfig.ReadonlyRootfs,
		CapabilitiesAdd:          c.HostConfig.CapAdd,
		CapabilitiesDrop:         c.HostConfig.CapDrop,
		Public:                   false,
		Encrypted:                nil,
	}
}

func mapContainerExposures(c *container.InspectResponse) []resource.Exposure {
	var exposures []resource.Exposure

	if c.HostConfig.NetworkMode == "host" {
		exposures = append(exposures, resource.Exposure{Type: "hostNetwork"})
	}

	for _, bindings := range c.HostConfig.PortBindings {
		for _, b := range bindings {
			hostPort, _ := strconv.ParseInt(b.HostPort, 10, 32)
			exposures = append(exposures, resource.Exposure{
				Type: "port",
				Port: int32(hostPort),
			})
		}
	}

	return exposures
}

func mapOwner(c *container.InspectResponse) *resource.Owner {
	if composeService, ok := c.Config.Labels["com.docker.compose.service"]; ok {
		project := c.Config.Labels["com.docker.compose.project"]
		return &resource.Owner{
			Kind: "compose-service",
			Name: fmt.Sprintf("%s/%s", project, composeService),
		}
	}
	return nil
}

func mapContainerAttributes(c *container.InspectResponse) map[string]any {
	attrs := map[string]any{
		"container.image":    c.Config.Image,
		"container.state":    c.State.Status,
		"container.running":  c.State.Running,
		"container.pid":      c.State.Pid,
		"container.platform": c.Platform,
	}

	if c.Config.User != "" {
		attrs["container.user"] = c.Config.User
	}
	if len(c.Config.Env) > 0 {
		attrs["container.env_count"] = len(c.Config.Env)
	}
	if len(c.Mounts) > 0 {
		attrs["container.mounts_count"] = len(c.Mounts)
		var dockerSock bool
		for _, m := range c.Mounts {
			if strings.Contains(m.Source, "docker.sock") {
				dockerSock = true
				break
			}
		}
		attrs["container.docker_sock_mount"] = dockerSock
	}
	if c.HostConfig.Privileged {
		attrs["container.privileged"] = true
	}
	if len(c.HostConfig.CapAdd) > 0 {
		attrs["container.cap_add"] = c.HostConfig.CapAdd
	}
	if len(c.HostConfig.SecurityOpt) > 0 {
		attrs["container.security_opt"] = c.HostConfig.SecurityOpt
	}
	if c.HostConfig.PidMode == "host" {
		attrs["container.host_pid"] = true
	}
	if c.HostConfig.IpcMode == "host" {
		attrs["container.host_ipc"] = true
	}

	return attrs
}

func mapImageAttributes(img *image.InspectResponse) map[string]any {
	attrs := map[string]any{
		"image.id":           img.ID[:12],
		"image.os":           img.Os,
		"image.architecture": img.Architecture,
		"image.size":         img.Size,
		"image.tags":         img.RepoTags,
		"image.digests":      img.RepoDigests,
	}

	if img.Config != nil {
		if img.Config.User != "" {
			attrs["image.user"] = img.Config.User
		} else {
			attrs["image.user"] = "root"
			attrs["image.runs_as_root"] = true
		}
		if len(img.Config.ExposedPorts) > 0 {
			ports := make([]string, 0, len(img.Config.ExposedPorts))
			for p := range img.Config.ExposedPorts {
				ports = append(ports, p)
			}
			attrs["image.exposed_ports"] = ports
		}
		if len(img.Config.Volumes) > 0 {
			vols := make([]string, 0, len(img.Config.Volumes))
			for v := range img.Config.Volumes {
				vols = append(vols, v)
			}
			attrs["image.volumes"] = vols
			for v := range img.Config.Volumes {
				if strings.Contains(v, "docker.sock") {
					attrs["image.docker_sock_volume"] = true
					break
				}
			}
		}
		if len(img.Config.Env) > 0 {
			attrs["image.env_count"] = len(img.Config.Env)
		}
		attrs["image.entrypoint"] = img.Config.Entrypoint
		attrs["image.cmd"] = img.Config.Cmd
		attrs["image.working_dir"] = img.Config.WorkingDir
		attrs["image.stop_signal"] = img.Config.StopSignal
	}

	if img.Architecture != "" && img.Os != "" {
		attrs["image.platform"] = fmt.Sprintf("%s/%s", img.Os, img.Architecture)
	}

	return attrs
}

func imageName(img *image.InspectResponse) string {
	if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
		return img.RepoTags[0]
	}
	if len(img.RepoDigests) > 0 {
		parts := strings.SplitN(img.RepoDigests[0], "@", 2)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return img.ID[:12]
}
