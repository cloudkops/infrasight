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

// mapContainer mirrors providers/kubernetes's mapPod: the container's own
// security/attributes live on a nested resource.Container (so container.security.*
// and container.* rule fields resolve exactly like they do for Kubernetes), while
// the top-level Resource carries only the resource-wide rollup (aggregateSecurity),
// host-namespace sharing, and networking — a standalone Docker container has no
// separate "pod" wrapper, so the Resource IS the one container it wraps.
func mapContainer(c *container.InspectResponse) resource.Resource {
	name := strings.TrimPrefix(c.Name, "/")
	sec := mapContainerSecurity(c)

	nested := resource.Container{
		Name:       name,
		Image:      containerConfig(c).Image,
		User:       parseUID(containerConfig(c).User),
		Privileged: sec.Privileged,
		Security:   sec,
		Attributes: mapContainerAttributes(c),
	}

	return resource.Resource{
		ID:       shortID(c.ID),
		Name:     name,
		Kind:     "container",
		Provider: Name,
		Metadata: resource.Metadata{
			Labels: containerConfig(c).Labels,
		},
		Security:   aggregateSecurity([]resource.Container{nested}),
		Networking: resource.Networking{Exposures: mapContainerExposures(c)},
		Runtime:    resource.Runtime{Containers: []resource.Container{nested}},
		Owner:      mapOwner(c),
		Attributes: containerHostAttributes(c),
	}
}

// aggregateSecurity gives resource-level rules (resource.security.*) a
// conservative cross-container view: privileged/root if any container is. Kept
// identical in shape to providers/kubernetes's helper of the same name even though
// Docker only ever passes a single-element slice today, so a future
// multi-container grouping (e.g. one Resource per Compose stack) is a non-breaking
// change.
func aggregateSecurity(containers []resource.Container) resource.SecurityContext {
	var sec resource.SecurityContext
	for _, c := range containers {
		if c.Security.Privileged {
			sec.Privileged = true
		}
		if c.Security.RunAsRoot {
			sec.RunAsRoot = true
		}
	}
	return sec
}

// parseUID extracts a numeric UID from a Docker --user value ("", "root", "1000",
// or "1000:1000"). An unparseable non-numeric user (e.g. "appuser") returns 0, same
// as root — no docker-baseline rule reads this typed field today (root detection
// goes through mapContainerSecurity's string-based check instead), so the
// ambiguity is harmless; this only fills in resource.Container.User for parity
// with the Kubernetes provider's typed field.
func parseUID(user string) int64 {
	if user == "" || user == "root" {
		return 0
	}
	part, _, _ := strings.Cut(user, ":")
	if part == "root" {
		return 0
	}
	uid, err := strconv.ParseInt(part, 10, 64)
	if err != nil {
		return 0
	}
	return uid
}

// containerHostAttributes mirrors providers/kubernetes's podLevelAttributes:
// host-namespace sharing (--pid=host/--ipc=host) is a resource-wide signal, not a
// per-container one, exactly like Kubernetes' pod-level hostPID/hostIPC.
func containerHostAttributes(c *container.InspectResponse) map[string]any {
	hostCfg := containerHostConfig(c)
	return map[string]any{
		"resource.host_pid": hostCfg.PidMode == "host",
		"resource.host_ipc": hostCfg.IpcMode == "host",
	}
}

// containerConfig and containerHostConfig guard against a nil Config/HostConfig on
// InspectResponse — normally always populated by a successful inspect, but nothing
// in the client guarantees it, and every accessor in this file assumes non-nil.
func containerConfig(c *container.InspectResponse) *container.Config {
	if c.Config != nil {
		return c.Config
	}
	return &container.Config{}
}

func containerHostConfig(c *container.InspectResponse) *container.HostConfig {
	if c.HostConfig != nil {
		return c.HostConfig
	}
	return &container.HostConfig{}
}

// shortID truncates a Docker object ID to its conventional 12-character short
// form, without panicking if the API ever returns something shorter.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func mapImage(img *image.InspectResponse) resource.Resource {
	attrs := mapImageAttributes(img)
	var labels map[string]string
	if img.Config != nil {
		labels = img.Config.Labels
	}

	return resource.Resource{
		ID:       shortID(img.ID),
		Name:     imageName(img),
		Kind:     "image",
		Provider: Name,
		Metadata: resource.Metadata{
			Labels: labels,
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
		ID:         shortID(net.ID),
		Name:       net.Name,
		Kind:       "network",
		Provider:   Name,
		Metadata:   resource.Metadata{Labels: net.Labels},
		Attributes: attrs,
	}
}

func mapContainerSecurity(c *container.InspectResponse) resource.SecurityContext {
	cfg := containerConfig(c)
	hostCfg := containerHostConfig(c)

	runAsRoot := false
	if cfg.User == "" || cfg.User == "0" || cfg.User == "root" {
		runAsRoot = true
	} else if parts := strings.SplitN(cfg.User, ":", 2); len(parts) > 0 {
		if uid, err := strconv.ParseInt(parts[0], 10, 64); err == nil && uid == 0 {
			runAsRoot = true
		} else if parts[0] == "root" {
			runAsRoot = true
		}
	}

	readOnlyRootFS := hostCfg.ReadonlyRootfs
	return resource.SecurityContext{
		RunAsRoot:                runAsRoot,
		Privileged:               hostCfg.Privileged,
		AllowPrivilegeEscalation: nil,
		ReadOnlyRootFilesystem:   &readOnlyRootFS,
		CapabilitiesAdd:          hostCfg.CapAdd,
		CapabilitiesDrop:         hostCfg.CapDrop,
		Public:                   false,
		Encrypted:                nil,
	}
}

func mapContainerExposures(c *container.InspectResponse) []resource.Exposure {
	hostCfg := containerHostConfig(c)
	var exposures []resource.Exposure

	if hostCfg.NetworkMode == "host" {
		exposures = append(exposures, resource.Exposure{Type: "hostNetwork"})
	}

	for _, bindings := range hostCfg.PortBindings {
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
	labels := containerConfig(c).Labels
	if composeService, ok := labels["com.docker.compose.service"]; ok {
		project := labels["com.docker.compose.project"]
		return &resource.Owner{
			Kind: "compose-service",
			Name: fmt.Sprintf("%s/%s", project, composeService),
		}
	}
	return nil
}

// mapContainerAttributes returns per-container attributes only. "container.image",
// "container.user", and "container.privileged" are deliberately absent: they'd be
// shadowed anyway by resolveField's typed fast path for those exact field names
// (which reads Container.Image/User/Privileged directly and always wins over the
// Attributes fallback), so setting them here would just be dead, misleading data —
// same reasoning as providers/kubernetes's mapContainer. "container.cap_add" is
// dropped too: SecurityContext.Flatten already exposes the same HostConfig.CapAdd
// data as "container.security.capabilities_add".
func mapContainerAttributes(c *container.InspectResponse) map[string]any {
	cfg := containerConfig(c)
	hostCfg := containerHostConfig(c)

	attrs := map[string]any{
		"container.state":    c.State.Status,
		"container.running":  c.State.Running,
		"container.pid":      c.State.Pid,
		"container.platform": c.Platform,
	}

	if len(cfg.Env) > 0 {
		attrs["container.env_count"] = len(cfg.Env)
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
	if len(hostCfg.SecurityOpt) > 0 {
		attrs["container.security_opt"] = hostCfg.SecurityOpt
	}

	return attrs
}

func mapImageAttributes(img *image.InspectResponse) map[string]any {
	attrs := map[string]any{
		"image.id":           shortID(img.ID),
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
	return shortID(img.ID)
}
