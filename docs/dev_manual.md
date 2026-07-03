# InfraSight Developer Manual

> Practical "how do I / where is" reference for working on this codebase. For the
> multi-provider roadmap and design rationale, see `plan.md`; for the
> Kubernetes provider specifically, see `k8s.md`.

---

## 1. Orientation: which docs are current

`docs/` contains two generations of documents. Know which is which before trusting one:

| Doc | Status |
|---|---|
| `future_plan.md`, `high_level_architecture.md` | **Direction-setting.** Written before any multi-provider code existed; describes the target architecture (Provider interface, Generic Resource Model, Provider Manager). The current code follows these. |
| `overview.md`, `scope.md` | **Historical / stale.** Describe an earlier, single-provider version of this codebase (`model.Workload`/`Container`, `cmd/isg`, `core/`). That code no longer exists — it was replaced by the structure described below. Useful for *why* certain decisions were made (e.g. the duplicate-Pod/Deployment-scan bug, missing `--kubeconfig`), not for *where things are now*. |
| `plan.md` | **Current plan.** The multi-provider architecture actually implemented, phase by phase. |
| `k8s.md` | **Current, as-built.** Kubernetes provider implementation detail, kept in sync with the code. |
| `dev_manual.md` (this file) | **Current.** How to extend any of it. |

If you're reading `overview.md`/`scope.md` and a file path doesn't exist, that's why —
check `plan.md` §6 or the repository map below instead.

---

## 2. Repository map

```
cmd/infrasight/main.go         Slim entrypoint. Blank-imports every provider and
                                every report format to trigger their init()
                                registration, then calls internal/app.Execute().

internal/app/                  CLI layer (cobra). No scanning logic lives here.
  app.go                         Execute(), root command construction
  scan_cmd.go                    `scan <provider>` subcommands, one per registered
                                  provider — built from provider.Manager.List(),
                                  never a hardcoded provider name
  dispatcher.go                  dispatch(): builds an orchestrator.Request, runs
                                  it, renders output, returns the exit code
  providers_cmd.go               `providers list`
  version_cmd.go                 `version`

internal/provider/              The Provider contract — see §3
  provider.go                     Provider interface, Kind, Options
  registry.go                     Register/Find/List (package-level map)
  manager.go                      Manager: thin wrapper + Initialize/Shutdown
                                   lifecycle hooks (currently no-ops — no provider
                                   needs them yet)

internal/resource/              The Generic Resource Model — see §4
  resource.go                     Resource, Runtime, Container, Limits, Owner
  metadata.go                     Metadata (labels/annotations)
  security.go                     SecurityContext + Flatten()
  networking.go                   Networking, Exposure + Flatten()
  validate.go                     Validate() — structural invariants

internal/rule/                  The Rule Engine — see §5
  parser/                          Rule/Condition YAML schema, loading, validation,
                                    severity ranking
  evaluator/                       Field resolution (resolveField) + Operator
                                    Registry (13 operators — see rulesets_manual.md §3)
  engine/                          Evaluate(): the resource x container x rule loop

internal/scan/                  Orchestration — see §6
  orchestrator/                    Full lifecycle: resolve provider -> pipeline ->
                                    load rules -> scan/engine -> return
  pipeline/                        Generic, provider-agnostic post-discovery
                                    stages: EnrichAttributes, ResolveRelationships
  engine/                          Thin ScanRequest/ScanResult wrapper around
                                    rule/engine.Evaluate

internal/report/                 Output — see §7
  finding.go, exitcode.go, summary.go   Shared across every format
  renderer.go, registry.go              Renderer contract + registry
  table/, json/, sarif/, markdown/      One renderer each, self-registering

providers/<name>/                One implementation per infrastructure source.
  kubernetes/                      Fully implemented (see k8s.md)
  docker/, host/, terraform/, ansible/   Stubs only (package + TODO comment) — not
                                          yet built

rulesets/<profile>/rules.yaml     YAML rule files. Four real Kubernetes profiles:
                                  security-baseline (11), dev-baseline (6),
                                  strict-runtime (11), ci-critical (2).
                                  docker/host/terraform/ansible-baseline dirs exist
                                  but are empty, waiting on their providers.

test/fixtures/<provider>/        Reserved for fixture files; Kubernetes tests
                                  currently build fixtures inline in Go rather than
                                  loading from here.
```

---

## End-to-end workflow diagram

Generic across every provider (Kubernetes is the only one implemented today, but
nothing below is Kubernetes-specific except inside the `Provider.Discover` box —
see `k8s.md` §2 for that box expanded in full Kubernetes detail).

```
User
  │  infrasight scan kubernetes --ruleset ./rulesets/security-baseline -n prod
  ▼
cmd/infrasight/main.go
  │  blank-imports every provider + report format package
  │  → each package's init() calls provider.Register(...) / report.Register(...)
  │  → app.Execute()
  ▼
internal/app                                   ── CLI layer, no scanning logic ──
  │  app.go        root cobra command
  │  scan_cmd.go   `scan <provider>` subcommands, built from provider.Manager.List()
  │  dispatcher.go dispatch() — the Command Dispatcher
  ▼
internal/scan/orchestrator.Run(ctx, manager, Request)
  │
  ├─ 1. manager.Get(providerName) ─────────────────────► internal/provider.Registry
  │                                                        name → Provider
  │                                                        (registered at startup,
  │                                                         each provider's own init())
  ├─ 2. manager.Initialize(provider)                       (no-op today)
  │
  ├─ 3. internal/scan/pipeline.Run(ctx, provider, opts)
  │        │
  │        ├─ a. Provider.Discover(ctx, opts)    ── provider-specific, self-contained ──
  │        │        client.go     connect (live providers only)
  │        │        discovery.go  list native objects, partial-failure tolerant
  │        │        normalize.go  native objects → []resource.Resource
  │        │
  │        ├─ b. resource.Validate(r)             for every resource — abort on malformed
  │        │
  │        ├─ c. EnrichAttributes(resources)       generic, every provider
  │        │        Security.Flatten() / Networking.Flatten() → Attributes
  │        │
  │        └─ d. ResolveRelationships(resources)   generic, every provider
  │                 drop a resource whose Owner matches another already kept
  │        ▼
  │     []resource.Resource
  │
  ├─ 4. internal/rule/parser.Load / LoadRuleset(dir, providerName)
  │        YAML → []Rule, validated, filtered to this provider's tag
  │
  ├─ 5. internal/scan/engine.Scan(ScanRequest{Resources, Rules})
  │        │
  │        └─ internal/rule/engine.Evaluate(resources, rules)
  │               for every resource × container × rule:
  │                 internal/rule/evaluator.Match(r, c, condition)
  │                   resolveField(field)         typed fast path, else Attributes
  │                   operatorNameFor(condition) ──────► internal/rule/evaluator.Registry
  │                                                         name → Operator
  │                                                         (13 operators — rulesets_manual.md §3)
  │                 match → report.Finding{...}
  │        ▼
  │     ScanResult{Findings, ResourceCount, RuleCount}
  │
  └─ 6. manager.Shutdown(provider)                          (no-op today)
  ▼
internal/app/dispatcher.go   (resumes after orchestrator.Run returns)
  │
  ├─ report.Get(format) ────────────────────────────────► internal/report.Registry
  │                                                          name → Renderer
  │                                                          (table/json/sarif/markdown,
  │                                                           self-registered via init())
  ├─ renderer.Write(out, findings)
  ├─ report.WriteSummary(stderr, findings, counts)
  └─ report.ExitCode(findings, failOn) ──► process exit code (0 / 1 / 2 / 3)
```

Three distinct points resolve something **by name from a registry** rather than a
hardcoded switch — `internal/provider.Registry` (provider name → `Provider`),
`internal/rule/evaluator.Registry` (operator name → `Operator`), and
`internal/report.Registry` (format name → `Renderer`). Everything else in the
diagram (`resource.Validate`, `EnrichAttributes`, `ResolveRelationships`,
`rule/engine.Evaluate`) is a single fixed implementation, not a registry — there's
exactly one way to do each, so a registry would just be indirection with nothing to
select between. Reach for a registry only when there's a real "given a name, pick
one of several interchangeable implementations" problem, the way these three have.

---

## 3. How the Provider contract works, and how to add a provider

`internal/provider.Provider` is the only thing `internal/app` and `internal/scan`
depend on:

```go
type Provider interface {
    Name() string
    Kind() Kind // KindLive (kubernetes, docker, host) or KindStatic (terraform, ansible)
    Discover(ctx context.Context, opts Options) ([]resource.Resource, error)
}
```

**To add a new provider** (say, filling in the `docker` stub):

1. In `providers/docker/`, implement `client.go` (connection bootstrap — live
   providers only), `discovery.go` (list native objects, tolerate partial
   failures the way `providers/kubernetes/discovery.go` does), `normalize.go`
   (native objects → `[]resource.Resource`, pure functions, no API calls),
   `provider.go` (wires the above, satisfies `Provider`).
2. In `provider.go`, register in an `init()`:
   ```go
   func init() { provider.Register(Name, New()) }
   ```
3. In `cmd/infrasight/main.go`, add a blank import: `_ "github.com/cloudkops/infrasight/providers/docker"`.
4. That's it — `internal/app/scan_cmd.go` builds `scan docker` automatically from
   `provider.Manager.List()`. No other file needs to change.

**Why registration lives in the provider's own `init()`, not in `internal/app`:**
Go guarantees every imported package's `init()` runs before `main()`'s body. Command
construction happens inside `app.Execute()` (called from `main()`), which is
strictly after every blank-imported provider's `init()` has already registered
itself — so `provider.Manager.List()` is always complete by the time commands are
built. (See `internal/app/app.go`'s doc comment — this ordering is deliberate, not
incidental.)

**Do not** make `internal/app` or `internal/scan` import a `providers/*` package
directly, even for a "just this once" reason — that's exactly the pattern this
architecture replaced (the original codebase's `cmd/isg/scan_sources.go` was a
hardcoded switch on provider name).

---

## 4. The Generic Resource Model, and how it stays extensible

`internal/resource.Resource` is what every provider normalizes into and the only
thing the Rule Engine ever sees. Key fields: `Metadata` (labels/annotations),
`Security` (`SecurityContext`), `Networking`, `Runtime.Containers` (empty for
non-containerized resources — the Rule Engine synthesizes one empty `Container` per
resource at evaluation time so resource-level-only rules still fire once), `Owner`,
`Attributes map[string]any` (the escape hatch).

**A field is queryable in a rule YAML one of two ways:**

- **Typed fast path** (`internal/rule/evaluator/evaluator.go`'s `resolveField`) —
  reserved for the small set of universally-hot fields: `resource.name`,
  `resource.namespace`, `resource.kind`, `resource.provider`, `resource.labels.*`,
  `resource.annotations.*`, `container.name`, `container.image`, `container.user`,
  `container.privileged`, `container.resources.*`.
- **`Attributes` fallback** — everything else, including **all**
  `SecurityContext`/`Networking` fields. These are populated automatically by
  `internal/scan/pipeline.EnrichAttributes` (generic, runs for every resource from
  every provider) calling `SecurityContext.Flatten()` / `Networking.Flatten()`.

**This means:** adding a new rule that references an *existing* `SecurityContext`/
`Networking` field, or any provider-specific `Attributes` key, is pure YAML — no Go
change. Adding a *new* field to `SecurityContext`/`Networking` only requires
updating that struct's `Flatten()` method (`internal/resource/security.go` or
`networking.go`) — never `evaluator.go`. This was a real gap in an earlier revision
(exposing two new `SecurityContext` fields required editing the evaluator directly)
and was fixed by moving the typed→attribute bridge into the generic pipeline stage;
don't reintroduce a per-field case in `resolveField` for anything under
Security/Networking.

**If the data doesn't exist on `Resource` at all yet** (a genuinely new signal no
provider collects), that's the one case that does need code: extend the relevant
struct in `internal/resource`, populate it in the provider's `normalize.go`, and (if
it's a Security/Networking field) add it to that struct's `Flatten()`.

---

## 5. The Rule Engine

Three packages, one direction of dependency (`engine` → `evaluator` → `parser`):

- **`internal/rule/parser`** — `Rule`/`Condition` YAML schema
  (`rule.go`), single-file and directory loading (`loader.go`/`ruleset.go`,
  the latter filtering by `Rule.Provider` tag), validation (`validate.go`),
  severity ranking (`severity.go`).
- **`internal/rule/evaluator`** — `Match(resource, container, condition) bool`.
  Resolves the condition's `Field` (§4), infers which operator the condition
  expresses from which of `Equals`/`NotEquals`/`Contains`/`Exists` is set
  (`operatorNameFor`), and delegates to the **Operator Registry**
  (`operator.go`/`registry.go`).
- **`internal/rule/engine`** — `Evaluate(resources, rules) []report.Finding`, the
  resource × container × rule loop, handling `condition`/`conditions` (AND)/
  `any_of` (OR).

**To add a new rule to an existing ruleset:** edit the YAML file directly — e.g.
`rulesets/security-baseline/rules.yaml`. No code change, ever, as long as the rule
only references fields already covered by §4.

**To add a new ruleset profile:** create `rulesets/<name>/rules.yaml` (or multiple
`.yaml` files — `LoadRuleset` concatenates every YAML file in the directory) tagged
`provider: kubernetes` (or whichever provider). Nothing else needs registering;
`--ruleset <dir>` just points at it.

**To add a new condition operator** (13 exist today — see `rulesets_manual.md` §3
for the full list — but the process is the same for the next one):
1. Implement the `Operator` interface in `internal/rule/evaluator/operator.go`:
   `Name() string`, `Match(actual any, cond parser.Condition) bool`.
2. Register it in `registry.go`'s `init()`.
3. Add a corresponding field to `parser.Condition` (`rule.go`) if the operator
   needs a new YAML key (e.g. `greater_than: 2`), and one entry to
   `conditionOperators` (`evaluator.go`) — a slice of `{name, set-predicate}`
   pairs checked in order by `operatorNameFor`, not a switch — so a condition
   using that key resolves to your operator's name. Where you insert the entry in
   the slice is its precedence when a condition (incorrectly) sets more than one
   operator key at once.
4. If the operator's argument can be malformed in a way that would otherwise fail
   silently at scan time (regex did — see `validateCondition` in
   `parser/validate.go`), add a load-time check there instead of letting it
   silently never-match.

---

## 6. Orchestration

- **`internal/scan/orchestrator.Run(ctx, manager, Request)`** — the full lifecycle:
  resolve provider by name → `Initialize` (no-op today) → `pipeline.Run` → load
  rules (`RulesFile` or `RulesetDir`, filtered by provider name) →
  `scan/engine.Scan` → `Shutdown` (no-op today). Knows nothing about any concrete
  provider.
- **`internal/scan/pipeline.Run(ctx, provider, opts)`** — `Discover()` → validate
  every resource (`resource.Validate`) → `EnrichAttributes` (§4) →
  `ResolveRelationships` (drops a resource whose `Owner` matches another resource
  already in the result set — generic, provider-agnostic; see `k8s.md` §4 for the
  Kubernetes-specific half of this story, which is resolving the ownership chain in
  the first place).
- **`internal/scan/engine.Scan(ScanRequest) ScanResult`** — thin wrapper: calls
  `rule/engine.Evaluate`, adds `ResourceCount`/`RuleCount` for the summary.

`internal/scan/orchestrator/orchestrator_test.go` has a self-contained example: a
`stubProvider` implementing `Provider` in-memory, run through the real
`orchestrator.Run`, asserting on findings — the fastest way to exercise the whole
chain without a live cluster or a fake K8s clientset.

---

## 7. Adding a new report format

`internal/report.Renderer`: `Name() string`, `Write(w io.Writer, findings []Finding) error`.

1. New package under `internal/report/<format>/`, one file, implementing `Renderer`.
2. `report.Register(renderer{})` in that package's `init()`.
3. Blank-import it from `cmd/infrasight/main.go`.
4. `--format <format>` works immediately — `internal/app/dispatcher.go` resolves the
   renderer by name via `report.Get()`, never a hardcoded switch.

`internal/report/renderers_test.go` exercises all four registered formats against
one `Finding` and prints each — the fastest way to sanity-check a new one.

---

## 8. Testing conventions

- **Table-driven** for anything with more than two cases (see
  `internal/rule/parser/validate_test.go`, `internal/rule/evaluator/evaluator_test.go`).
- **Fixtures built inline in Go**, not loaded from `test/fixtures/` — every current
  test constructs its `corev1.Pod`/`resource.Resource`/etc. directly in the test
  file. `test/fixtures/kubernetes/` exists per `plan.md`'s original plan but
  nothing reads from it yet; either convention is fine going forward, but inline is
  what's actually in use today.
- **Fake clientset for Kubernetes**: `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(...)`,
  with `client.PrependReactor(...)` to simulate a resource-type-specific failure
  (see `providers/kubernetes/discovery_test.go`).
- **Race detector**: run `go test -race ./...` after touching anything with
  goroutines (`providers/kubernetes/discovery.go`'s fan-out is the only concurrent
  code today).
- **Real rulesets are the integration fixture**: `internal/rule/parser/loader_test.go`
  loads all four actual `rulesets/*/rules.yaml` files and asserts exact rule
  counts — if you add/remove a rule, update the expected count there.

Standard commands:
```sh
go build ./...
go vet ./...
go test ./...
go test -race ./...
gofmt -l .          # should print nothing
```

---

## 9. Known gaps (intentional, not oversights)

- `providers/{docker,host,terraform,ansible}` are stubs (package + one-line TODO
  comment) — see `plan.md` §5 for the design notes on each before
  implementing.
- No config file (`infrasight.yaml`) yet — every flag is passed on the command
  line. Planned as `plan.md` phase 11.
- Numeric operators (`greater_than`/`less_than`/`greater_than_or_equal`/
  `less_than_or_equal`) parse plain numbers only, not Kubernetes resource
  quantities (`"500m"`, `"2Gi"`) — see `rulesets_manual.md` §3. Quantity-aware
  parsing is still on the backlog (`scope.md`).
- Per-provider flag sets aren't separated — every `scan <provider>` subcommand
  currently binds the same flag set (including Kubernetes-only ones like
  `--kubeconfig`), since only one provider exists. This needs revisiting once a
  second provider lands (see `k8s.md` §8).
- No live-cluster integration test exists anywhere in this repo — the fake
  clientset and the in-memory stub provider (§6) are the practical ceiling.
