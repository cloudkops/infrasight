# Docker Provider — As-Built Reference

> Scope: `providers/docker/` and the generic stages it depends on
> (`internal/scan/pipeline`, `internal/rule/evaluator`). This describes what is
> actually implemented and tested today, not a build plan — see `docs/plan.md`
> for the overall multi-provider roadmap, `docs/k8s.md` for the sibling
> Kubernetes provider this one is deliberately modeled after, and
> `docs/dev_manual.md` for how to extend any of this.

---

## 1. Package layout

One package, four files, each with one job — identical split to
`providers/kubernetes`:

| File | Owns |
|---|---|
| `providers/docker/provider.go` | Satisfies `internal/provider.Provider`. The only file that imports `internal/provider`; sequences the other three. Registers itself via `init()`. |
| `providers/docker/client.go` | Docker client bootstrap only — ambient environment (`DOCKER_HOST`, `DOCKER_TLS_VERIFY`, ...) or an explicit `--docker-host`. Returns the concrete `*client.Client` (no fake-client seam exists yet — see §10). |
| `providers/docker/discovery.go` | Lists/inspects native `github.com/docker/docker/api/types/*` objects. Returns native objects, not `resource.Resource` — no model knowledge here. |
| `providers/docker/normalize.go` | Maps native objects → `[]resource.Resource`. Pure functions, no API calls — this is what `normalize_test.go` exercises against hand-built fixtures. |

---

## 2. End-to-end flow

```
infrasight scan docker --ruleset ./rulesets/docker-baseline --fail-on HIGH
        │
        ▼
internal/app/scan_cmd.go        → parses flags into scanFlags
        │
        ▼
internal/app/dispatcher.go      → dispatch(): builds orchestrator.Request, calls Run
        │
        ▼
internal/scan/orchestrator.Run  → manager.Get("docker")  [the only lookup — never
        │                          a direct import of providers/docker]
        ▼
providers/docker.Provider.Discover(ctx, opts)
        │
        ├─ 1. client.go   :: newClient(opts.DockerHost)
        │        client.FromEnv (DOCKER_HOST/DOCKER_TLS_VERIFY/DOCKER_CERT_PATH)
        │        + client.WithAPIVersionNegotiation(), overridden by an explicit
        │        --docker-host if set
        │
        ├─ 2. discovery.go :: discoverAll() fans out one goroutine per resource
        │      *type*, shared ctx, partial-failure tolerant:
        │        containers · images · volumes · networks       (4 fetches)
        │      Containers/images are List()ed once, then each item is
        │      Inspect()ed individually (Docker's API has no bulk-inspect) —
        │      bounded to 8 concurrent inspects at a time (inspectConcurrency),
        │      not one goroutine per item, and not serial either. A single item
        │      failing to inspect (e.g. it was removed mid-scan) is recorded as
        │      a warning and skipped, never fails the whole type.
        │      A whole resource *type* failing (its List() call itself erroring)
        │      is also collected into rawObjects.warnings; only every type
        │      failing aborts discovery — see §7.
        │
        └─ 3. normalize.go :: NormalizeAll() → []resource.Resource
                (field-by-field mapping in §5)
        │
        ▼
internal/scan/pipeline.Run       → resource.Validate() each result, then:
        │                            1. EnrichAttributes()   (§6 — generic, every provider)
        │                            2. ResolveRelationships() (generic; a container's
        │                               Owner points at its compose-service, but no
        │                               compose-service resource is itself discovered,
        │                               so this is currently a no-op for Docker)
        ▼
internal/scan/orchestrator.Run  → parser.LoadRuleset(dir, "docker") or parser.Load(file)
        │                          → internal/scan/engine.Scan() → internal/rule/engine.Evaluate()
        ▼
internal/report                  → renderer resolved by --format (table/json/sarif/markdown),
                                    report.WriteSummary, report.ExitCode
```

---

## 3. `provider.go` — interface contract

```go
package docker

const Name = "docker"

type Provider struct{}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}

func (p *Provider) Name() string        { return Name }
func (p *Provider) Kind() provider.Kind { return provider.KindLive }

func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	client, err := newClient(opts.DockerHost)
	if err != nil {
		return nil, fmt.Errorf("docker: client: %w", err)
	}
	defer client.Close()
	raw, err := discoverAll(ctx, client)
	if err != nil {
		return nil, err
	}
	return NormalizeAll(raw), nil
}
```

Registration happens in this package's own `init()`, exactly like Kubernetes —
`cmd/infrasight/main.go` only needs a blank import
(`_ "github.com/cloudkops/infrasight/providers/docker"`) to trigger it. `Namespace`,
`AllNamespaces`, `LabelSelector`, `FieldSelector`, `Kubeconfig`, `Context` on
`provider.Options` are all Kubernetes-only and simply unused here — only
`DockerHost` applies.

---

## 4. Normalize flow — native object → `resource.Resource`

**This is the section that changed shape to align with Kubernetes.** A standalone
Docker container has no separate "pod" wrapper the way a Kubernetes container has
a Pod — but the rule engine's field-naming convention (`container.*` vs.
`resource.*`/`networking.*`) assumes exactly that two-level split (see
`docs/k8s.md` §5–6). `mapContainer()` therefore does what `mapPod()` does for
Kubernetes: it treats **the Resource as wrapping exactly one nested
`resource.Container`**, not as being the container itself.

```go
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
		ID:         shortID(c.ID),
		Name:       name,
		Kind:       "container",
		Provider:   Name,
		Metadata:   resource.Metadata{Labels: containerConfig(c).Labels},
		Security:   aggregateSecurity([]resource.Container{nested}),
		Networking: resource.Networking{Exposures: mapContainerExposures(c)},
		Runtime:    resource.Runtime{Containers: []resource.Container{nested}},
		Owner:      mapOwner(c),
		Attributes: containerHostAttributes(c),
	}
}
```

**Why this matters — the bug this shape fixes.** `internal/rule/engine.Evaluate`
skips a rule entirely for any resource with zero entries in `Runtime.Containers`
if that rule references a `container.*`-prefixed field (the same mechanism that
correctly skips `container.resources.cpu_limit` rules for a Kubernetes ConfigMap —
see `docs/k8s.md` §5). An earlier version of this provider put the container's
`SecurityContext` directly on the top-level `Resource` and left `Runtime` empty,
which meant `container.security.capabilities_add`, `container.docker_sock_mount`,
and every other `container.*`-scoped rule in `docker-baseline` silently never
fired — for any container, on any host, no error surfaced. Populating
`Runtime.Containers` with one real entry is what makes those rules resolve at all.

Per-container `resource.SecurityContext` (`mapContainerSecurity()`):

| Docker field | `resource.SecurityContext` field |
|---|---|
| `Config.User` is `""`, `"0"`, `"root"`, or `<uid>:<gid>` with uid `0` | `RunAsRoot = true` |
| `HostConfig.Privileged` | `Privileged` |
| `HostConfig.ReadonlyRootfs` | `ReadOnlyRootFilesystem` (`*bool`, always non-nil — Docker's own field is a plain `bool`, so this is always an explicit `true`/`false`, never "unset" the way Kubernetes' pointer field can be) |
| `HostConfig.CapAdd` / `CapDrop` | `CapabilitiesAdd` / `CapabilitiesDrop` |
| *(no equivalent)* | `AllowPrivilegeEscalation` — always `nil`; Docker has no direct analogue to Kubernetes' `allowPrivilegeEscalation` |

Resource-level `Security` (`aggregateSecurity()`) is the exact same helper
Kubernetes uses, kept identical in shape even though Docker only ever calls it
with a one-element slice today: `Privileged`/`RunAsRoot` are true if *any*
container in the slice is. This keeps a future multi-container grouping (e.g. one
Resource per Compose stack, mirroring a Kubernetes Pod's multiple containers) a
non-breaking change rather than a redesign.

Host-namespace sharing is resource-level, not per-container — exactly like
Kubernetes' pod-level `hostPID`/`hostIPC` (`containerHostAttributes()`, going
straight into `Resource.Attributes` under the literal dotted key a rule uses, no
dedicated evaluator case):

| Field | True when |
|---|---|
| `resource.host_pid` | `HostConfig.PidMode == "host"` (`--pid=host`) |
| `resource.host_ipc` | `HostConfig.IpcMode == "host"` (`--ipc=host`) |

`hostNetwork` is **not** duplicated as an attribute — it's covered by
`Resource.Networking` (below) via `EnrichAttributes` (§6), same as Kubernetes.

Per-container attributes (`mapContainerAttributes()`), always present as
`true`/`false`/actual-value, never omitted unless explicitly noted:

| Field | Meaning |
|---|---|
| `container.state` | raw state string (`"running"`, `"exited"`, ...) |
| `container.running` | `State.Running` |
| `container.pid` | `State.Pid` |
| `container.platform` | `Platform` (e.g. `"linux"`) |
| `container.env_count` | `len(Config.Env)` — only set if non-zero |
| `container.mounts_count` | `len(Mounts)` — only set if non-zero |
| `container.docker_sock_mount` | any mount's `Source` contains `docker.sock` |
| `container.security_opt` | `HostConfig.SecurityOpt` — only set if non-empty |

**Deliberately absent from `Attributes`:** `container.image`, `container.user`,
`container.privileged`, `container.cap_add`. The first three would be dead,
misleading data — `internal/rule/evaluator.resolveField`'s typed fast path for
`container.image`/`container.user`/`container.privileged` reads the real
`resource.Container` fields directly and always wins over the `Attributes`
fallback, so a stale/differently-typed copy in `Attributes` could never actually
be consulted by a rule, only confuse a future reader. `container.cap_add` is
redundant with `SecurityContext.Flatten`'s `container.security.capabilities_add`
(§6) — same underlying `HostConfig.CapAdd` data, one canonical rule-facing name.

Exposure maps into `Resource.Networking.Exposures` (`mapContainerExposures()`),
resource-level, shared conceptually with Kubernetes' `Resource.Networking`:

| Source | `Exposure.Type` |
|---|---|
| `HostConfig.NetworkMode == "host"` (`--network=host`) | `"hostNetwork"` |
| `HostConfig.PortBindings` (`-p host:container`) | `"port"` (one per binding, `Exposure.Port` = host port) |

**Compose ownership, not a full relationship graph.** `mapOwner()` reads the
`com.docker.compose.service`/`com.docker.compose.project` labels Compose sets on
every container it creates and, if present, sets `Owner{Kind: "compose-service",
Name: "<project>/<service>"}`. No Compose-service or Compose-project resource is
itself discovered or normalized — unlike Kubernetes' Pod→Deployment resolution
(`docs/k8s.md` §4), there's no second resource for `ResolveRelationships` to
de-duplicate against, so this `Owner` field today only annotates *which* logical
service a container belongs to; it doesn't collapse multiple containers of the
same Compose service into one reported resource.

**Images, volumes, networks map straight to `Attributes`** — no `SecurityContext`,
no `Networking`, no `Runtime` (they aren't containers):

| Resource | Attributes |
|---|---|
| Image | `image.id`, `image.os`, `image.architecture`, `image.size`, `image.tags` ([]string), `image.digests` ([]string), `image.user` (falls back to `"root"` + `image.runs_as_root=true` if the Dockerfile never set `USER`), `image.exposed_ports`/`image.volumes` (only if declared), `image.docker_sock_volume` (a declared `VOLUME` path containing `docker.sock`), `image.env_count`, `image.entrypoint`, `image.cmd`, `image.working_dir`, `image.stop_signal`, `image.platform` (`"<os>/<arch>"`) |
| Volume | `volume.driver`, `volume.scope`, `volume.mountpoint`, `volume.name`, `volume.size_bytes`/`volume.ref_count` (only if `UsageData` is populated — requires `docker system df -v`-style detail, not always returned) |
| Network | `network.driver`, `network.scope`, `network.internal`, `network.attachable`, `network.ingress`, `network.enable_ipv4`, `network.enable_ipv6`, `network.containers_count` |

**Nil-safety.** `Config`/`HostConfig` on `container.InspectResponse` (and `Config`
on `image.InspectResponse`) are pointers the Docker API does not guarantee are
populated. `containerConfig()`/`containerHostConfig()` return an empty zero-value
struct instead of dereferencing a nil pointer — every accessor in `normalize.go`
goes through these two helpers rather than touching `c.Config`/`c.HostConfig`
directly. `mapImage()` similarly guards `img.Config` before reading `.Labels`.

**ID truncation.** Docker's convention is a 12-character short ID. `shortID()`
truncates to 12 chars but passes a shorter string through unchanged instead of
panicking — used uniformly for container/image/network IDs (`imageName()`'s
digest-based fallback included).

---

## 5. `EnrichAttributes` — how Security/Networking become rule-queryable

Identical generic code to the Kubernetes path (`internal/scan/pipeline/enrich.go`,
§6 in `docs/k8s.md`) — `normalize.go` never touches this file:

```go
r.Attributes = mergeAttributes(r.Attributes, r.Security.Flatten("resource.security"))
r.Attributes = mergeAttributes(r.Attributes, r.Networking.Flatten("networking"))
for j := range r.Runtime.Containers {
	c := &r.Runtime.Containers[j]
	c.Attributes = mergeAttributes(c.Attributes, c.Security.Flatten("container.security"))
}
```

Because `mapContainer()` now populates `Runtime.Containers` with a real entry
(§4), this stage produces `container.security.capabilities_add`,
`container.security.read_only_root_filesystem`, and
`container.security.capabilities_drop` for Docker exactly the way it already did
for Kubernetes — no changes needed here, no Docker-specific case anywhere in
`internal/rule/evaluator`. `resource.security.run_as_root`/`privileged` come from
the `aggregateSecurity()` rollup on the top-level `Resource.Security`.

---

## 6. Discovery fan-out

Concurrent, one goroutine per resource *type* (4 total), all sharing the request
`ctx`, plus a bounded worker pool per *item* within the containers/images types:

| Resource type | List call | Per-item inspect? | Returned as a Resource? |
|---|---|---|---|
| Container | `ContainerList(All: true)` | `ContainerInspect` (bounded to `inspectConcurrency`=8 concurrent) | yes |
| Image | `ImageList(All: true)` | `ImageInspect` (bounded to `inspectConcurrency`=8 concurrent) | yes |
| Volume | `VolumeList` | no | yes |
| Network | `NetworkList` | no | yes |

A whole type's `List()` call failing is recorded in `rawObjects.warnings` and
counted toward a `failed` counter; only `failed == total` (every one of the 4
types failed outright) aborts `discoverAll()` with an error. A single item within
a type failing to inspect (e.g. removed between `List()` and `Inspect()`) is also
recorded as a warning but never counted toward `failed` — it's dropped from the
result, the rest of that type's items are unaffected, and the scan proceeds.

No live-daemon integration test exists (none of this repo's tooling can reach a
real Docker socket) — `providers/docker/normalize_test.go`'s hand-built
`container.InspectResponse`/`image.InspectResponse` fixtures are the practical
ceiling for "verified without a daemon," same caveat `docs/k8s.md` §10 notes for
the Kubernetes provider's fake-clientset tests. Unlike Kubernetes, this package
has no fake-client seam yet — `newClient()` returns the concrete `*client.Client`
rather than an interface, so `discoverAll()`/`Discover()` themselves aren't
unit-tested against a fake daemon today; only the pure `normalize.go` mapping
functions are.

---

## 7. CLI flags

Docker-specific flag on every `scan <provider>` subcommand (the Kubernetes-only
flags — `--namespace`, `--all-namespaces`, `--label-selector`,
`--field-selector`, `--kubeconfig`, `--context` — are harmless no-ops for Docker,
same shared-flag-set tradeoff `docs/k8s.md` §8 documents):

- `--docker-host` (default `""` → `DOCKER_HOST` env / platform default socket)

---

## 8. Error handling summary

| Failure | Behavior |
|---|---|
| Daemon unreachable (bad `--docker-host`, socket not running) | `Discover()` returns a wrapped error immediately, no partial scan; CLI exits 1 |
| One resource type's `List()` fails (e.g. permission denied on a restricted socket) | Recorded as a warning; scan continues with the other 3 types |
| Every resource type's `List()` fails | `discoverAll()` returns an aggregated error, scan aborts |
| A container/image is removed between `List()` and `Inspect()` | That item is dropped, recorded as a warning; the rest of that type's items are unaffected |

---

## 9. Bundled ruleset — `rulesets/docker-baseline`

25 rules, provider-scoped to `"docker"`. Field prefixes follow the same
convention §4–§5 establish: `container.*`/`container.security.*` for per-container
signals, `resource.*`/`resource.security.*` for resource-wide rollups and
host-namespace sharing, `networking.*` for exposure, `image.*`/`volume.*`/
`network.*` for those resource kinds. See `docs/rulesets_manual.md` for the full
operator reference; the table below is Docker-specific field usage only:

| Category | Example rule | Field(s) used |
|---|---|---|
| Container security | Privileged, root, docker.sock mount, host PID/IPC, dangerous capabilities added, no capability dropped, read-only rootfs not enforced | `resource.security.privileged`/`run_as_root`, `container.docker_sock_mount`, `resource.host_pid`/`host_ipc`, `container.security.capabilities_add`/`capabilities_drop`/`read_only_root_filesystem` |
| Container governance/ops | Missing team/env label, container not running, unpinned `:latest` tag, custom `--security-opt` applied | `resource.labels.team`/`env`, `container.running`, `container.image`, `container.security_opt` |
| Image security | Runs as root, exposes ports, declares a docker.sock volume, unpinned tag, no entrypoint, declares volumes | `image.runs_as_root`, `image.exposed_ports`, `image.docker_sock_volume`, `image.tags`, `image.entrypoint`, `image.volumes` |
| Network | Not internal, attachable, ingress network | `network.internal`, `network.attachable`, `network.ingress` |
| Volume | Local driver, no labels | `volume.driver`, `resource.labels` |

---

## 10. Tests (all passing, including `-race`)

| File | Covers |
|---|---|
| `providers/docker/normalize_test.go` | Container→Resource shape parity with Kubernetes' `mapPod` (`TestMapContainer_ShapeMirrorsKubernetes`), that shadowed/dead attributes stay absent (`TestMapContainer_DeadAttributesRemoved`), non-root UID parsing + read-only rootfs (`TestMapContainer_NonRootReadOnly`), nil `Config`/`HostConfig` safety, `shortID` truncation, nil `Config` on images, volume/network mapping, and an evaluation-level regression test (`TestDockerBaseline_KeyRulesFireOnPrivilegedContainer`) that loads the real `docker-baseline` ruleset and asserts the CRITICAL/HIGH rules actually produce findings for a realistic privileged container — the class of test a parse-only check cannot catch |
| `internal/rule/parser/loader_test.go` | `docker-baseline` (25 rules) parses and validates via `TestLoadRuleset_DockerBaseline` |
| `internal/resource/security_test.go`, `networking_test.go` | `Flatten()` output — shared with Kubernetes, not Docker-specific, but what makes §5 work |

No discovery-level test exists for this provider (`discovery_test.go` has no
Docker counterpart) — see §6's fake-client-seam gap. Adding one would mean giving
`newClient`/`discoverAll` an interface seam the way `providers/kubernetes/client.go`
does with `kubernetes.Interface`, which the Docker SDK doesn't provide out of the
box the same way; deferred rather than done partially.
