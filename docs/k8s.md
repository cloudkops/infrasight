# Kubernetes Provider — As-Built Reference

> Scope: `providers/kubernetes/` and the generic stages it depends on
> (`internal/scan/pipeline`, `internal/rule/evaluator`). This describes what is
> actually implemented and tested today, not a build plan — see `docs/plan.md`
> for the overall multi-provider roadmap and `docs/dev_manual.md` for how to extend
> any of this.

---

## 1. Package layout

One package, four files, each with one job:

| File | Owns |
|---|---|
| `providers/kubernetes/provider.go` | Satisfies `internal/provider.Provider`. The only file that imports `internal/provider`; sequences the other three. Registers itself via `init()`. |
| `providers/kubernetes/client.go` | K8s client bootstrap only — in-cluster config, `--kubeconfig`, `--context`. Returns `kubernetes.Interface` (not the concrete `*Clientset`) so tests inject a fake. |
| `providers/kubernetes/discovery.go` | Lists native `k8s.io/api` objects. Returns native objects, not `resource.Resource` — no model knowledge here. |
| `providers/kubernetes/normalize.go` | Maps native objects → `[]resource.Resource`. Pure functions, no API calls — this is what `normalize_test.go` exercises against hand-built fixtures. |

No `adapters/kubernetes` package exists — everything K8s-specific lives in this one
provider package.

---

## 2. End-to-end flow

```
infrasight scan kubernetes --ruleset ./rulesets/security-baseline -n prod --fail-on HIGH
        │
        ▼
internal/app/scan_cmd.go        → parses flags into scanFlags
        │
        ▼
internal/app/dispatcher.go      → dispatch(): builds orchestrator.Request, calls Run
        │
        ▼
internal/scan/orchestrator.Run  → manager.Get("kubernetes")  [the only lookup — never
        │                          a direct import of providers/kubernetes]
        ▼
providers/kubernetes.Provider.Discover(ctx, opts)
        │
        ├─ 1. client.go   :: newClient(opts.Kubeconfig, opts.Context)
        │        in-cluster config → --kubeconfig path → ~/.kube/config fallback chain
        │
        ├─ 2. discovery.go :: discoverAll() fans out one goroutine per resource type,
        │      shared ctx, partial-failure tolerant:
        │        Pods · ReplicaSets · Deployments · StatefulSets · DaemonSets ·
        │        Jobs · CronJobs · Services · ConfigMaps · Secrets · Ingresses ·
        │        NetworkPolicies · ClusterRoleBindings · RoleBindings
        │                                      (14 fetches; ReplicaSets never
        │                                       returned as a resource, see §4)
        │      ClusterRoleBindings are cluster-scoped — fetched regardless of
        │      the requested --namespace/-A, unlike every other type here.
        │      A single resource type failing (e.g. RBAC forbids `list jobs`) is
        │      collected into rawObjects.warnings; only every type failing aborts.
        │
        └─ 3. normalize.go :: NormalizeAll() → []resource.Resource
                (field-by-field mapping in §5; Owner already resolved through the
                 ReplicaSet hop here, see §4)
        │
        ▼
internal/scan/pipeline.Run       → resource.Validate() each result, then:
        │                            1. EnrichAttributes()   (§6 — generic, every provider)
        │                            2. ResolveRelationships() (§4 — generic, every provider)
        ▼
internal/scan/orchestrator.Run  → parser.LoadRuleset(dir, "kubernetes") or parser.Load(file)
        │                          → internal/scan/engine.Scan() → internal/rule/engine.Evaluate()
        ▼
internal/report                  → renderer resolved by --format (table/json/sarif/markdown),
                                    report.WriteSummary, report.ExitCode
```

---

## 3. `provider.go` — interface contract

```go
package kubernetes

const Name = "kubernetes"

type Provider struct{}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}

func (p *Provider) Name() string        { return Name }
func (p *Provider) Kind() provider.Kind { return provider.KindLive }

func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	client, err := newClient(opts.Kubeconfig, opts.Context)
	if err != nil {
		return nil, fmt.Errorf("kubernetes: client: %w", err)
	}
	namespace := opts.Namespace
	if opts.AllNamespaces {
		namespace = ""
	}
	raw, err := discoverAll(ctx, client, namespace, opts.LabelSelector, opts.FieldSelector)
	if err != nil {
		return nil, err
	}
	return NormalizeAll(raw), nil
}
```

Registration happens in this package's own `init()`, not in `cmd/infrasight/main.go` —
`main.go` only needs a blank import (`_ "github.com/cloudkops/infrasight/providers/kubernetes"`)
to trigger it. Command construction in `internal/app.Execute()` runs *after* every
package's `init()` has completed (Go guarantees this), so `provider.List()` always
sees "kubernetes" registered by the time `scan` subcommands are built — see
`docs/dev_manual.md` §"Adding a provider" for why this ordering matters.

---

## 4. Relationship resolution (resolves `scope.md` P0: duplicate workload scanning)

Two separate concerns, deliberately split across provider-specific and generic code:

1. **Provider-specific: resolving the ownership chain.** `NormalizeAll()` builds a
   lookup `rsOwner: map["namespace/name"]*resource.Owner` from every listed
   ReplicaSet's own `OwnerReferences`. When normalizing a Pod, if its immediate
   owner is a ReplicaSet, that lookup resolves one hop further to the ReplicaSet's
   owner (normally the Deployment) — so a Pod's `Owner` field points directly at
   the Deployment, never at an intermediate ReplicaSet (which is never itself
   returned as a resource — see §6). This two-hop resolution is Kubernetes-specific
   and lives entirely in `normalize.go`; other providers' ownership chains (if any)
   are one hop and don't need this.
2. **Generic: dropping the redundant child.** `internal/scan/pipeline.ResolveRelationships`
   drops any resource whose `Owner.Kind`+`Owner.Name` matches another resource
   already in the same result set — regardless of which provider produced either
   one. This is provider-agnostic code, shared by every future provider with an
   ownership concept (Docker container → compose service, etc.); it is not
   duplicated in `providers/kubernetes`.

Visible behavior: findings report against `Deployment/my-app` instead of
`Pod/my-app-7f8b9-abcde` once a Pod's owner chain resolves to a tracked Deployment.
Orphan Pods (no controller) pass through unaffected.

---

## 5. Normalize flow — native object → `resource.Resource`

`mapPodSpec()` maps a `corev1.PodSpec` (embedded in every workload-shaped object,
including `CronJob.Spec.JobTemplate.Spec.Template.Spec`) into `Resource.Runtime.Containers`:

```
PodSpec.InitContainers      → resource.Container, Attributes["container_kind"]="init"
PodSpec.Containers          → resource.Container
PodSpec.EphemeralContainers → resource.Container, Attributes["container_kind"]="ephemeral"
                              (via the documented corev1.Container(ec.EphemeralContainerCommon)
                               conversion — the two types are declared field-identical)
```

Per-container `resource.SecurityContext` (`mapContainer()`):

| K8s field | `resource.SecurityContext` field |
|---|---|
| `securityContext.runAsUser == 0` | `RunAsRoot = true` (checked first — the more direct signal) |
| `securityContext.runAsNonRoot` (if `runAsUser` isn't 0) | `RunAsRoot = !runAsNonRoot` |
| `securityContext.allowPrivilegeEscalation` | `AllowPrivilegeEscalation` (`*bool`, nil if unset) |
| `securityContext.readOnlyRootFilesystem` | `ReadOnlyRootFilesystem` (`*bool`, nil if unset) |
| `securityContext.capabilities.add`/`drop` | `CapabilitiesAdd`/`CapabilitiesDrop` |
| `securityContext.privileged` | `Privileged` |

Resource-level `Security` (`aggregateSecurity()`) is a conservative OR across every
container: `Privileged`/`RunAsRoot` are true if *any* container is.

Pod-level (not per-container) signals go straight into `Resource.Attributes` under
the *exact* dotted key a rule would use — `podLevelAttributes()` sets
`"resource.host_pid"`, `"resource.host_ipc"`, `"resource.host_path_volume"`
(true if any `spec.volumes[]` has a non-nil `hostPath`), and
`"resource.service_account_token_automount"` (see the caveat below) directly, so
they're queryable through the plain `Attributes` fallback with no dedicated
evaluator case at all. `hostNetwork` is **not** duplicated here — it's covered by
`Resource.Networking` (below) via `EnrichAttributes` (§6) instead.

**`resource.service_account_token_automount` is a documented approximation.** It
reflects only `pod.Spec.AutomountServiceAccountToken` (default `true` if unset,
matching the Pod's own runtime default) — a Kubernetes ServiceAccount object can
independently default this to `false` too, and this provider doesn't discover or
normalize ServiceAccounts today, so that half of the effective calculation isn't
accounted for. This matches how most Kubernetes security scanners approximate the
check, but isn't a complete accounting.

Per-container signals beyond `SecurityContext`, set directly in `mapContainer()`
(always present as `true`/`false`, never omitted, so `equals: false` works without
an `exists` check first):

| Field | True when |
|---|---|
| `container.probes.readiness_configured` | `container.readinessProbe` is set |
| `container.probes.liveness_configured` | `container.livenessProbe` is set |
| `container.secret_env_vars` | any `env[].valueFrom.secretKeyRef` or `envFrom[].secretRef` is set — never the Secret's actual value, just that a reference exists |

Exposure maps into `Resource.Networking.Exposures`, a resource-level list (not
per-container) shared conceptually with every other provider:

| Source | `Exposure.Type` |
|---|---|
| Pod/template `spec.hostNetwork: true` | `"hostNetwork"` |
| Service `type: NodePort` | `"NodePort"` (one per port) |
| Service `type: LoadBalancer` | `"LoadBalancer"` (one per port) |
| Ingress (has `spec.rules` or a `defaultBackend`) | `"Ingress"` (one per rule) |

ConfigMap/Secret/NetworkPolicy carry no `Networking` exposures — they're
config/governance objects, not reachability surfaces.

**ConfigMap, Secret, NetworkPolicy — shape only, never content:** these three map
to `Resource.Attributes` directly (no dedicated typed struct, no `Runtime`, no
`Networking`), and deliberately expose only counts/flags, never the underlying
data:

| Resource | Attributes |
|---|---|
| ConfigMap | `configmap.data_keys_count` (int), `configmap.immutable` (bool) |
| Secret | `secret.type` (string, e.g. `"Opaque"`), `secret.data_keys_count` (int), `secret.immutable` (bool) — **never** `.Data`/`.StringData` values |
| Ingress | `ingress.tls_enabled` (bool), `ingress.rules_count` (int), `ingress.class` (string, `""` if unset) |
| NetworkPolicy | `networkpolicy.policy_types` ([]string), `networkpolicy.ingress_rules_count`/`egress_rules_count` (int), `networkpolicy.selects_all_pods` (bool — true when `spec.podSelector` is empty, i.e. namespace-wide) |
| ClusterRoleBinding / RoleBinding | `rbac.role_ref_kind` (string), `rbac.role_ref_name` (string), `rbac.binds_cluster_admin` (bool), `rbac.subjects_count` (int) |

A Secret's actual key/value data never passes through `mapSecret()` into
`Resource` at all — there is no code path by which it could reach a report or a
rule condition. See `providers/kubernetes/normalize_test.go`'s
`TestMapSecret_NoDataValuesLeakIntoAttributes`.

**`rbac.binds_cluster_admin`** is `true` only when `roleRef.kind == "ClusterRole"`
and `roleRef.name == "cluster-admin"` — the built-in ClusterRole every stock
cluster ships with. This provider does **not** evaluate a custom ClusterRole's
actual rules to detect an equivalent-but-differently-named admin role, and does
**not** resolve which Pods use the bound ServiceAccount — a ClusterRoleBinding/
RoleBinding is surfaced as its own finding, not linked to the workloads that might
be affected by it. See `docs/dev_manual.md` §9.

Anything without a typed field (`seccompProfile`, `seLinuxOptions`, ...) can go into
`Container.Attributes`/`Resource.Attributes` directly — but note the fields above
that already have a typed home (`SecurityContext`, `Networking`) do **not** need a
raw `Attributes` entry from this package; §6 covers how they become queryable.

---

## 6. `EnrichAttributes` — how Security/Networking become rule-queryable

This is generic code in `internal/scan/pipeline/enrich.go`, run once for every
resource from every provider (not Kubernetes-specific), immediately after
`resource.Validate()` and before `ResolveRelationships()`:

```go
func EnrichAttributes(resources []resource.Resource) []resource.Resource {
	for i := range resources {
		r := &resources[i]
		r.Attributes = mergeAttributes(r.Attributes, r.Security.Flatten("resource.security"))
		r.Attributes = mergeAttributes(r.Attributes, r.Networking.Flatten("networking"))
		for j := range r.Runtime.Containers {
			c := &r.Runtime.Containers[j]
			c.Attributes = mergeAttributes(c.Attributes, c.Security.Flatten("container.security"))
		}
	}
	return resources
}
```

`SecurityContext.Flatten(prefix)` (`internal/resource/security.go`) and
`Networking.Flatten(prefix)` (`internal/resource/networking.go`) turn every
populated typed field into an `Attributes` entry under that prefix — e.g.
`container.security.allow_privilege_escalation`, `networking.hostNetwork`,
`resource.security.run_as_root`. `mergeAttributes` never overwrites a key the
provider set explicitly.

**Why this matters for Kubernetes specifically:** `normalize.go` never needs to
know about this — it just populates the typed `SecurityContext`/`Networking`
structs the way §5 describes, and every field on them becomes queryable
automatically. This is what makes `strict-runtime`'s
`container.security.allow_privilege_escalation`/`read_only_root_filesystem` rules
and `networking.loadBalancer` work with zero changes to
`internal/rule/evaluator` — see `docs/dev_manual.md` for the general principle.

---

## 7. Discovery fan-out (resolves `scope.md` P0: resource type coverage)

Concurrent, one goroutine per resource type, all sharing the request `ctx`:

| Resource type | Returned as a Resource? |
|---|---|
| Pod | yes |
| Deployment | yes |
| StatefulSet | yes |
| DaemonSet | yes |
| Job | yes |
| CronJob | yes |
| Service | yes (NodePort/LoadBalancer exposure) |
| ConfigMap | yes (shape only — no data values, see §5) |
| Secret | yes (shape only — no data values, see §5) |
| Ingress | yes (Ingress exposure + TLS/class attributes, see §5) |
| NetworkPolicy | yes (rule-count/selector attributes, see §5) |
| ClusterRoleBinding | yes (cluster-scoped — fetched regardless of `--namespace`, see §5) |
| RoleBinding | yes (namespaced, see §5) |
| ReplicaSet | **no** — listed only to resolve the Pod→ReplicaSet→Deployment ownership chain (§4) |

A resource-type list call failing independently (RBAC denies `list jobs`) does not
fail `discoverAll()` — it's recorded in `rawObjects.warnings`. Only every resource
type failing returns an error (`discovery_test.go`'s
`TestDiscoverAll_TotalFailureErrors` covers this; `TestDiscoverAll_PartialFailureIsTolerated`
covers the tolerant case).

---

## 8. CLI flags

Bound uniformly on every `scan <provider>` subcommand today (only Kubernetes is
registered, so this is moot in practice; when a second provider lands, the
Kubernetes-only flags below stay harmless no-ops for it — see `docs/dev_manual.md`
open question on per-provider flag sets):

- `--kubeconfig` (default `""` → in-cluster, then `~/.kube/config`)
- `--context` (default `""` → current context)
- `--namespace` / `-n`, `--all-namespaces` / `-A`
- `--label-selector` / `-l`, `--field-selector`

---

## 9. Error handling summary

| Failure | Behavior |
|---|---|
| No reachable cluster (bad/missing kubeconfig) | `Discover()` returns a wrapped error immediately, no partial scan; CLI exits 1 |
| RBAC denies one resource type | Recorded as a warning (§7); scan continues with the rest |
| RBAC denies everything | `discoverAll()` returns an aggregated error, scan aborts |
| Namespace doesn't exist | Empty result, not an error (matches K8s API behavior) |

---

## 10. Tests (all passing, including `-race`)

| File | Covers |
|---|---|
| `providers/kubernetes/normalize_test.go` | Security context + resource mapping (`TestMapPod_SecurityContextAndResources`), Service NodePort exposure, Owner resolution through the ReplicaSet hop, ConfigMap/Secret shape-only mapping (incl. the no-data-leak guarantee), Ingress exposure/attributes, NetworkPolicy rule counts, probes/hostPath/automount/secret-env-var attributes, ClusterRoleBinding/RoleBinding cluster-admin detection |
| `providers/kubernetes/discovery_test.go` | Partial-failure tolerance, total-failure error, end-to-end `Discover()` via a fake clientset (pre-dedup resource counts) |
| `internal/resource/security_test.go`, `networking_test.go` | `Flatten()` output, including the "known type gets explicit false" vs. "unknown type gets implicit true" distinction |
| `internal/scan/pipeline/enrich_test.go` | `EnrichAttributes` merges correctly and never overwrites a provider-set attribute |
| `internal/scan/pipeline/relationships_test.go` | `ResolveRelationships` drops an owned child, keeps orphans, keeps a resource whose owner isn't in the result set |
| `internal/scan/orchestrator/orchestrator_test.go` | Full `Run()` against a stub in-memory provider — the closest thing to an end-to-end test without a live cluster |
| `internal/rule/parser/loader_test.go` | All four real rulesets (`security-baseline`=11, `dev-baseline`=6, `strict-runtime`=11, `ci-critical`=2) parse and validate |

No live-cluster integration test exists (none of this repo's tooling can reach a
real API server) — `discovery_test.go` and `orchestrator_test.go` are the practical
ceiling for "verified without a cluster."
