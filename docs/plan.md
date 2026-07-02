# InfraSight Multi-Provider Plan (Kubernetes, Docker, Host/Linux, IaC, Ansible)

> Status: proposal, not yet implemented. This revision follows `future_plan.md` and
> `high_level_architecture.md` literally for structure — `internal/` layout, Generic
> Resource Model, Provider Manager as the single resolution contract, no CLI-specific
> package name (no `cmd/isg`). Targets five providers: **Kubernetes, Docker,
> Host/Linux, Terraform (IaC), Ansible.**

---

## 1. Correction from the previous revision

An earlier draft of this plan kept `model.Workload`/`Container` and a `cmd/isg`
package to minimize churn against the (now-deleted) original codebase. That's no
longer the constraint — there is no legacy code to preserve, so this plan now follows
`high_level_architecture.md`'s "Recommended Project Structure" directly:

- **No `cmd/isg`.** `cmd/` holds only a slim binary entrypoint
  (`cmd/infrasight/main.go`) that calls `internal/app.Execute()`. The CLI Layer
  itself — cobra commands, flag parsing, config loading — lives in `internal/app`,
  per the doc's explicit rule: "Should never contain scanning logic."
- **Providers are first-class, resolved only through the Provider Manager.**
  Nothing in `internal/app` or `internal/scan` imports a `providers/*` package
  directly. The only lookup mechanism is `internal/provider.Manager.Get(name)` /
  `.List()`. Adding a provider means registering it at startup, not adding a case to
  a switch anywhere else in the codebase.
- **Generic Resource Model, not `Workload`+`Container`.** `high_level_architecture.md`
  is explicit: "The Rule Engine never sees provider-specific objects." Every
  provider normalizes into one `resource.Resource` type (`internal/resource/`),
  matching the doc's own file list (`resource.go`, `metadata.go`, `security.go`,
  `networking.go`). There is no separate `adapters/` layer sitting outside
  `providers/` — each provider package is self-contained (client + discovery +
  normalize + the `Provider` interface implementation), matching the doc's
  `providers/kubernetes`, `providers/docker`, ... layout with no external adapter.

The five-provider scope (Kubernetes/Docker/Host/Terraform/Ansible) and the
live-vs-static distinction from the previous revision still hold — see §2 — but
everything downstream of that is re-planned against the two source docs instead of
against convenience.

---

## 2. Provider kinds: live vs. static

`future_plan.md` and `high_level_architecture.md` both write every provider as
something you *connect to* and *discover live objects from*. That's true for
Kubernetes, Docker, and Host/Linux — it is **not** true for Ansible and Terraform,
which scan static files with no live target:

| Provider | What it scans | Shape of "resource" |
|---|---|---|
| Kubernetes | live cluster API | Pod/Deployment/StatefulSet/DaemonSet/Job/CronJob/Service |
| Docker | live daemon (socket/API) | Container/Image |
| Host/Linux | local OS (procfs, systemd, users) | Process/Service/User |
| Terraform (IaC) | `terraform show -json` plan output | `aws_instance`, `google_compute_instance`, ... |
| Ansible | playbooks + inventory (static YAML) | Play/Task/Role |

`internal/provider.Provider` therefore exposes `Kind() Kind` (`KindLive` /
`KindStatic`, see §4.2) so the Orchestrator can give static providers different CLI
ergonomics (`--path` instead of `--namespace`) and different error framing ("could
not parse playbook" vs. "could not reach cluster") without needing a second
interface.

---

## 3. Target architecture

```
                               CLI
                                │
                          Cobra Commands            internal/app
                                │
                        Command Dispatcher          internal/app/dispatcher.go
                                │
                         Scan Orchestrator          internal/scan/orchestrator
                                │
                        Provider Manager            internal/provider
                                │  (Register / Find / List / Initialize / Shutdown)
        ┌───────────────┬───────────────┬───────────────┬───────────────┐
        ▼               ▼               ▼               ▼               ▼
   Kubernetes         Docker           Host          Terraform         Ansible
  providers/       providers/      providers/       providers/       providers/
  kubernetes        docker           host            terraform         ansible
  (client+          (client+        (discovery+      (discovery+       (discovery+
   discovery+         discovery+      normalize)       normalize)        normalize)
   normalize)          normalize)
        │               │               │               │               │
        └───────────────┴───────┬───────┴───────────────┴───────────────┘
                                 ▼
                     Resource Normalization  →  internal/resource.Resource
                                 ▼
                          Rule Engine            internal/rule/{parser,evaluator,engine}
                                 ▼
                             Findings              internal/report
                                 ▼
                       Report Engine (table / json / sarif / markdown)
```

Every arrow into "Provider Manager" is a name lookup (`"kubernetes"`, `"docker"`,
...), never a direct import — that's the concrete meaning of "provider as first-class
citizen, resolved via provider manager contract."

---

## 4. Core interfaces

### 4.1 `internal/provider` — the resolution contract

```go
package provider

type Kind string

const (
    KindLive   Kind = "live"   // kubernetes, docker, host
    KindStatic Kind = "static" // terraform, ansible
)

type Options struct {
    Namespace     string
    AllNamespaces bool
    LabelSelector string
    FieldSelector string
    Verbose       bool

    Kubeconfig string
    Context    string
    DockerHost string

    Path string // playbook dir, .tf dir, or plan JSON file — static providers only
}

type Provider interface {
    Name() string          // "kubernetes", "docker", "host", "terraform", "ansible"
    Kind() Kind
    Discover(ctx context.Context, opts Options) ([]resource.Resource, error)
}
```

`internal/provider/registry.go` owns `map[string]Provider` + `Register()`/`Find()`/
`List()` (Registry + Factory pattern). `internal/provider/manager.go` sits on top of
the registry and owns the lifecycle: `Initialize()`/`Shutdown()` for whichever
providers are enabled (config-driven, see Phase 10). `internal/app` and
`internal/scan/orchestrator` depend only on `Manager` — never on the registry map or
on any `providers/*` package directly. Each provider's `init()` (or an explicit
`RegisterAll()` called once from `cmd/infrasight/main.go`) calls
`provider.Register(name, constructor)`.

### 4.2 `internal/resource` — the Generic Resource Model

```go
package resource

type Resource struct {
    ID         string
    Name       string
    Namespace  string            // or file path / module path for static providers
    Kind       string            // pod, container, service, process, systemd_unit,
                                  // aws_instance, ansible_task, ...
    Provider   string            // kubernetes, docker, host, terraform, ansible
    Metadata   Metadata          // labels, annotations
    Security   SecurityContext   // cross-provider security signals
    Networking Networking        // exposure/ports/host network/public reachability
    Runtime    Runtime           // container(s)/process(es) composing this resource
    Owner      *Owner            // parsed controller/owner reference
    Attributes map[string]any    // provider-specific escape hatch for rule matching
}

type Runtime struct {
    Containers []Container // image, user, privileged, resource limits — empty for
                            // non-containerized resources (host process, ansible task)
}

type Owner struct {
    Kind string
    Name string
}
```

`Metadata`, `SecurityContext`, and `Networking` are their own files
(`metadata.go`/`security.go`/`networking.go`) per `high_level_architecture.md`'s file
list. `Container` and `Owner` live inside `resource.go` alongside `Resource` and
`Runtime` — they're compositional parts of one resource, not independent concerns.

**Relationships (ownership) are first-class, not a hack.** Every provider that has a
notion of "owned by" (K8s Pod→ReplicaSet→Deployment, Docker container→compose
service, Ansible task→play) populates `Owner`. This is what resolves `scope.md`'s
duplicate Pod/Deployment scanning: the orchestrator (or a pipeline stage) drops any
resource whose `Owner` resolves to another resource already in the result set,
generically, for every provider — not a Kubernetes-specific special case.

**`internal/resource/validate.go`** — `Validate(r Resource) error` is a required
contract, not optional polish: nothing else guarantees a provider's `normalize.go`
actually produced a well-formed `Resource` (non-empty `ID`/`Name`/`Provider`/`Kind`,
non-nil `Metadata` maps). This mirrors `internal/rule/parser`'s `ValidateRule` on the
other side of the Rule Engine boundary — rules are validated on the way in, resources
should be validated on the way in too. Called once per resource, right after
normalization, before anything downstream sees it.

**`internal/scan/pipeline/relationships.go`** — `ResolveRelationships` is the concrete
home for the Owner-based dedup described above. It is a plain function, not a
registry: there is exactly one dedup strategy, applied uniformly to the combined
result of every active provider, so it doesn't need dynamic dispatch — it needs to
exist as its own named pipeline stage (Discovery → Normalization → **Relationship
Resolution** → Rule Evaluation → Aggregation → Reporting) instead of being buried
inside the orchestrator as an unnamed side effect.

### 4.3 `internal/rule` — provider-agnostic evaluation

Three packages, matching `high_level_architecture.md`'s file list exactly:

- `internal/rule/parser` — `Rule`/`Condition` structs, YAML loading
  (`loader.go`/`ruleset.go`), validation, severity ranking. A `Rule.Provider` tag
  (empty = applies everywhere) lets `LoadRuleset()` filter rules to the providers
  actually in scope for a given scan (resolves the "existing rules silently
  mismatching new providers" risk).
- `internal/rule/evaluator` — condition matching against `resource.Resource`. Typed
  fast-path fields (`security.privileged`, `networking.exposed`, ...) plus an
  attribute-path fallback into `Resource.Attributes` so a new provider can introduce
  queryable fields without editing the evaluator.

  **Operator Registry (`operator.go`/`registry.go`)** — condition operators
  (`equals`, `not_equals`, `contains`, `exists`, and `scope.md`'s wishlist —
  `regex`, `greater_than`, `less_than`, `in`, `not_in`, `starts_with`, `ends_with`)
  are resolved by name from a registry, the same Registry pattern as
  `internal/provider`, instead of a hardcoded switch. This is the same shape of
  problem as provider resolution — "given a name string, pick one of several
  implementations at runtime" — so it gets the same contract: an `Operator`
  interface, one implementation per operator, registered under its YAML name.
  Adding an operator is additive (implement + register); `Evaluate()` never grows a
  new case.
- `internal/rule/engine` — `Evaluate(resources, rules) []report.Finding`, the
  stateless computation core. Called by `internal/scan/engine`, never by a provider
  or the CLI directly.

### 4.4 `internal/scan` — orchestration, not logic

- `internal/scan/orchestrator` — the full lifecycle: resolve provider(s) via
  `provider.Manager` → `Initialize()` → `Discover()` → resolve relationships/dedup →
  run `internal/rule/engine` → render via `internal/report` → `Shutdown()`. Knows
  nothing about Kubernetes/Docker/etc. — it only calls interfaces.
- `internal/scan/pipeline` — the reusable staged execution model (Discovery →
  Normalization → Rule Evaluation → Aggregation → Reporting) that the orchestrator
  drives; this is what makes Phase 9 (concurrent multi-provider scan) additive later
  rather than a rewrite — each stage already treats "how many providers" as a
  parameter.
- `internal/scan/engine` — the thin `Evaluate`-level entry point tying
  `internal/resource` + `internal/rule` together for a single scan request; kept
  separate from `orchestrator` so it stays a pure, easily-testable function with no
  provider-lifecycle concerns mixed in.

### 4.5 `internal/report`

`finding.go`/`exitcode.go`/`summary.go` at the package root (shared across formats);
`table/`, `json/`, `sarif/`, `markdown/` each own their renderer, matching
`high_level_architecture.md`'s explicit format list (Markdown is new — addresses the
`scope.md` P1 output-format gap).

**Renderer Registry (`renderer.go`/`registry.go`)** — same reasoning as the Operator
Registry: selecting a renderer by `--format` name is "resolve by name" exactly like
provider or operator resolution, so it gets the same contract instead of a hardcoded
`switch opts.format` in `internal/app`. A `Renderer` interface
(`Name() string; Write(w io.Writer, findings []Finding) error`); each of
`table`/`json`/`sarif`/`markdown` registers itself under its format name from its
own package `init()`. Adding a new output format (CSV, JUnit XML — `scope.md` P1)
never touches `internal/app`.

---

## 5. Provider-by-provider notes

### 5.1 Kubernetes — reference implementation
- `providers/kubernetes/{provider,client,discovery,normalize}.go`. `client.go`:
  in-cluster config first, falls back to `--kubeconfig`/`~/.kube/config`, applies
  `--context`. `discovery.go`: concurrent fan-out over Pods, ReplicaSets,
  Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, Services — one resource
  type forbidden by RBAC must not abort the whole scan (partial-failure tolerant).
  `normalize.go`: native object → `resource.Resource`, including init/ephemeral
  containers, expanded security context fields, and `Owner` population (§4.2).
- This is the reference every other provider's `discovery.go`/`normalize.go` split
  is validated against, and the rulesets that exercise the full field set first.

### 5.2 Docker
- `providers/docker/{provider,client,discovery,normalize}.go`. Client against
  `Options.DockerHost` (default `unix:///var/run/docker.sock`). Normalize:
  `Privileged` from `HostConfig.Privileged`, `User` from `Config.User` (Docker
  defaults to root when empty — call this out, it differs from Kubernetes),
  resource limits from `HostConfig.Memory`/`NanoCPUs`, exposure from
  `NetworkSettings.Ports`.
- Net-new rule opportunities: Docker-socket bind-mount detection
  (`/var/run/docker.sock` — a known container-escape vector), missing restart
  policy, default bridge network usage.

### 5.3 Host / Linux
- `providers/host/{provider,discovery,normalize}.go` (no `client.go` — nothing to
  connect to, this provider only runs on the machine being scanned; document that
  constraint in `--help`). One `Resource{Kind: "host"}` with `Runtime.Containers`
  holding one synthetic entry per process/systemd unit worth checking.
- Discovery: `/proc/<pid>/status` (uid), `systemctl show` (`NoNewPrivileges`,
  `ProtectSystem`), `/etc/passwd` (root-equivalent accounts), `/proc/net/tcp`
  (listening sockets → `Networking`).

### 5.4 Terraform (IaC) — static provider
- `providers/terraform/{provider,discovery,normalize}.go` (no `client.go`).
  `discovery.go` reads `terraform show -json <plan>` from `Options.Path` (user runs
  `terraform plan -out=tfplan && terraform show -json tfplan > plan.json`) — native
  HCL parsing is a later increment, not v1.
- `normalize.go`: `planned_values.root_module.resources[]` → one `Resource` each,
  `Kind: resource.type` (e.g. `aws_instance`), `Attributes` populated from `values`
  verbatim. `Security`/`Networking` best-effort mapped for well-known resource types
  only (e.g. `aws_s3_bucket` → `acl`/`server_side_encryption_configuration`);
  everything else stays queryable only via `Attributes`. Findings are naturally
  "will create" rather than "is running" — no `Finding` schema change needed.

### 5.5 Ansible — static provider
- `providers/ansible/{provider,discovery,normalize}.go` (no `client.go`).
  `discovery.go` parses playbook YAML + referenced `roles/*/tasks/*.yml` from
  `Options.Path` directly with `gopkg.in/yaml.v3` — no need to shell out to
  `ansible` for static analysis.
- `normalize.go`: each **task** → `Resource{Kind: "ansible_task"}`, `Namespace` =
  playbook file path, `Attributes` = raw module args plus `become`/`no_log`/
  `ignore_errors`.
- Net-new rule opportunities: `no_log: false` near secret-shaped variable names,
  `shell`/`command` module usage instead of idempotent modules, unpinned package
  versions in `apt`/`yum` tasks, `become: true` without `become_user`.
- Known limitation, stated up front rather than half-solved: Jinja2-templated
  values (`{{ vault_password }}`) are not resolved — flagged as opaque, not
  evaluated.

---

## 6. Directory structure (current skeleton)

```
cmd/
  infrasight/
    main.go                    # slim entrypoint -> internal/app.Execute()
internal/
  app/                         # CLI layer — cobra commands, flags, config; no scan logic
    app.go
    scan_cmd.go
    dispatcher.go
    providers_cmd.go
    version_cmd.go
  scan/
    orchestrator/orchestrator.go
    pipeline/pipeline.go
    engine/engine.go
  provider/
    provider.go                 # interface + Kind + Options
    registry.go                  # Register/Find/List
    manager.go                    # Initialize/Shutdown lifecycle on top of registry
  resource/
    resource.go                 # Resource, Runtime, Container, Owner
    metadata.go
    security.go
    networking.go
  rule/
    parser/{rule,loader,ruleset,validate,severity}.go
    evaluator/evaluator.go
    engine/engine.go
  report/
    finding.go
    exitcode.go
    summary.go
    table/table.go
    json/json.go
    sarif/sarif.go
    markdown/markdown.go
providers/
  kubernetes/{provider,client,discovery,normalize}.go
  docker/{provider,client,discovery,normalize}.go
  host/{provider,discovery,normalize}.go
  terraform/{provider,discovery,normalize}.go
  ansible/{provider,discovery,normalize}.go
pkg/                           # reserved for externally-importable helpers (empty for now)
rulesets/
  security-baseline/ dev-baseline/ strict-runtime/ ci-critical/   # tag provider: kubernetes
  docker-baseline/ host-baseline/ terraform-baseline/ ansible-baseline/
test/
  fixtures/{kubernetes,docker,host,terraform,ansible}/
configs/
bin/
plugins/                       # reserved for future external/gRPC provider processes (not now)
```

No `adapters/` directory — each provider package under `providers/` is
self-contained (client + discovery + normalize), matching
`high_level_architecture.md`'s layout with nothing sitting outside it.

---

## 7. Phased delivery plan

| # | Phase | Deliverable | Depends on |
|---|---|---|---|
| 1 | **Provider contract + registry/manager** | `internal/provider/{provider,registry,manager}.go`; nothing else depends on a concrete provider package yet | — |
| 2 | **Generic resource model** | `internal/resource/*`; `internal/rule/evaluator` attribute-path fallback | — (parallel to 1) |
| 3 | **Kubernetes provider** | `providers/kubernetes/*` fully implemented against 1+2; first end-to-end scan works | 1, 2 |
| 4 | **Orchestrator + CLI wiring** | `internal/scan/{orchestrator,pipeline,engine}`, `internal/app/*`, `cmd/infrasight/main.go`; `infrasight scan kubernetes` works end-to-end | 3 |
| 5 | **Docker provider** | `providers/docker/*`, `docker-baseline` ruleset | 1, 2, 4 |
| 6 | **Host/Linux provider** | `providers/host/*`, `host-baseline` ruleset | 1, 2, 4 |
| 7 | **Terraform provider** | `providers/terraform/*`, `terraform-baseline` ruleset | 1, 2, 4 |
| 8 | **Ansible provider** | `providers/ansible/*`, `ansible-baseline` ruleset | 1, 2, 4 |
| 9 | **Rule provider-tagging** | `Rule.Provider` field, `LoadRuleset()` filtering, existing rulesets tagged `kubernetes` | 4 |
| 10 | **Multi-provider concurrent scan** | `infrasight scan --provider kubernetes,docker,host` runs providers concurrently, merges findings via one `internal/scan/pipeline` run | 5, 6, 7, 8 |
| 11 | **Config file + provider metadata** | `infrasight.yaml` for default provider set/connection settings; `infrasight providers list`/`info` | 4 |

Suggested order: **1 → 2 (parallel) → 3 → 4 → (5, 6, 7, 8 any order/parallel) → 9 → 10
→ 11**. Kubernetes goes first among providers (Phase 3) because it's the richest
resource shape — anything the Generic Resource Model is missing surfaces there
before four more providers depend on it.

---

## 8. Testing strategy

- **Phase 1–2**: `internal/provider/manager_test.go` (register/duplicate-register/
  get-missing/list); `internal/rule/evaluator` tests for typed fields + attribute
  fallback, using hand-built `resource.Resource` fixtures (no provider needed yet).
- **Phase 3 (Kubernetes)**: table-driven normalize tests per resource type against
  fixtures in `test/fixtures/kubernetes/`; a `Discover()` integration test using
  `k8s.io/client-go/kubernetes/fake` covering all resource types plus one forbidden
  resource type (assert partial-failure tolerance, not a hard abort).
- **Phase 4 (orchestrator)**: an orchestrator test with two fake registered
  providers returning known resources — first place `provider.Manager` resolution
  is exercised outside a unit test.
- **Phase 5–8 (each new provider)**: same table-driven normalize-test pattern as
  Kubernetes; Terraform/Ansible tests are pure-function (parse fixture file → assert
  resources), no fake client needed.
- **Phase 10 (concurrency)**: `go test -race` once providers run concurrently inside
  `internal/scan/pipeline`.

---

## 9. Open questions

1. Docker: rootless Docker / Podman socket compatibility in v1 scope or deferred?
2. Terraform: is native HCL parsing (no `terraform` binary) a real requirement, or
   is requiring a JSON plan acceptable long-term?
3. Ansible: Jinja2-templated values are explicitly out of scope (see §5.5) — confirm
   that's acceptable rather than a gap to close later.
4. Should `internal/scan/pipeline` be built generically enough in Phase 4 to support
   concurrent multi-provider execution from day one, or deliberately sequential
   until Phase 10 needs it? Leaning sequential-first — the pipeline *stage* shape
   doesn't change, only how many providers feed it concurrently.
