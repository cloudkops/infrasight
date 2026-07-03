# Rulesets Manual

> How to write and organize InfraSight rules. This is aimed at whoever is authoring
> detection rules — no Go required for anything in this document except the last
> section. For how the engine that runs these rules is built, see `dev_manual.md`
> §5 and `k8s.md` §5–6.

---

## 1. What a ruleset is

A ruleset is a directory under `rulesets/` containing one or more `.yaml`/`.yml`
files, each holding a list of rules. `LoadRuleset` reads every YAML file in the
directory and concatenates them — split a large ruleset across multiple files if
that's easier to maintain (e.g. `security.yaml`, `governance.yaml` in the same
profile directory); there's no requirement to keep one file per profile.

```sh
infrasight scan kubernetes --ruleset ./rulesets/security-baseline -n prod
```

A single ad hoc rules file also works without a whole directory:

```sh
infrasight scan kubernetes --rules ./my-rule.yaml -n prod
```

Current profiles, all Kubernetes today:

| Profile | Rules | Use when |
|---|---|---|
| `security-baseline` | 11 | Standard first gate — root, privileged, hostNetwork, NodePort, missing resource limits/requests, missing governance labels |
| `dev-baseline` | 6 | Same risk categories as security-baseline, reduced severities, so local/dev clusters don't fail CI over things that matter more in prod |
| `strict-runtime` | 11 | Production hardening — everything in security-baseline at its highest severity, plus privilege escalation, read-only root filesystem, host PID/IPC namespaces |
| `ci-critical` | 2 | Minimal CRITICAL-only gate for a fast CI check |

---

## 2. Rule schema

```yaml
- id: "ISG-SEC-001"                    # required, unique within a scan
  title: "Container running as root"   # required
  severity: "HIGH"                     # required: LOW | MEDIUM | HIGH | CRITICAL
  category: "security"                 # optional, free text (used for grouping only)
  description: "..."                   # optional
  remediation: "..."                   # optional
  docs_url: "https://..."              # optional
  provider: "kubernetes"               # optional — see §5

  # exactly one of the three condition modes below:
  condition: { ... }                   # single condition
  conditions: [ { ... }, { ... } ]      # ALL must match (AND)
  any_of: [ { ... }, { ... } ]          # ANY must match (OR)
```

`id`, `title`, and a valid `severity` are required — `ValidateRule` rejects the
whole ruleset at load time otherwise (fails loudly, not silently). Duplicate `id`
values within a ruleset are also rejected. At least one of `condition`/`conditions`/
`any_of` must be set with a non-empty `field`.

**Nesting is one level only** — you cannot AND a group of ORs, or vice versa. If a
rule needs that, split it into two rules, or wait for `scope.md`'s nested-condition
backlog item.

---

## 3. Condition schema and operators

```yaml
condition:
  field: "container.user"     # required — dotted path, see §4 for the full list
  equals: 0                   # OR any one of the operator keys below
  not_equals: 0
  contains: "sub"
  not_contains: "sub"
  starts_with: "prefix"
  ends_with: "suffix"
  regex: "^pattern$"
  in: [0, 1000]
  not_in: ["latest"]
  greater_than: 2
  less_than: 2
  greater_than_or_equal: 2
  less_than_or_equal: 2
  exists: true                 # can be used alone, or as the implicit default below
```

Set **at most one** operator key per condition — whichever one is set determines
which operator runs (see the precedence table below for what happens if you set
more than one). If none of them are set, the condition is an `exists` check:
`exists: true` means "field must be present/set/truthy", `exists: false` means
"field must be absent/unset". This is why you'll see `exists: false` used for
"missing X" rules (§6) rather than a `not_equals`.

| YAML key | Operator | Matches when |
|---|---|---|
| `equals` | `equals` | resolved value == the given value (works for strings, bools, and numbers) |
| `not_equals` | `not_equals` | resolved value != the given value |
| `contains` | `contains` | resolved value (as a string) contains the substring |
| `not_contains` | `not_contains` | resolved value (as a string) does **not** contain the substring |
| `starts_with` | `starts_with` | resolved value (as a string) has the given prefix |
| `ends_with` | `ends_with` | resolved value (as a string) has the given suffix |
| `regex` | `regex` | resolved value (as a string) matches the given RE2 pattern (Go's `regexp` syntax) |
| `in` | `in` | resolved value equals one of the given list's entries |
| `not_in` | `not_in` | resolved value equals none of the given list's entries |
| `greater_than` / `less_than` / `greater_than_or_equal` / `less_than_or_equal` | same name | resolved value, parsed as a plain number, compares as expected — see the numeric caveat below |
| *(none of the above)* | `exists` | field is present (`exists: true`) or absent (`exists: false`) |

**Numeric operators don't understand Kubernetes quantity suffixes.** `greater_than`
et al. parse the resolved value as a plain `float64` (via `strconv.ParseFloat`) —
a value like `"2"` or `2` works, but `container.resources.cpu_limit`/`memory_limit`
are Kubernetes quantity strings (`"500m"`, `"2Gi"`) and will simply fail to parse,
making the condition never match rather than compare incorrectly. Use `exists`
against those two fields (as `dev-baseline`/`security-baseline` already do), not
numeric comparison, until quantity-aware parsing is added (`scope.md` backlog).
Numeric operators work well against plain numeric attributes instead — e.g. a
future provider exposing `resource.replica_count` as a bare integer.

**`in`/`not_in` and empty/missing fields:** both require the field to resolve to a
value first — same as every operator except `exists` (§3's `Match` short-circuits
to `false` on an unresolved field before calling any operator, `exists` excepted).
An absent field is never "in" anything, and is also never "not_in" anything
(despite the name) — if you need "field is absent OR not in this list," use
`any_of` with an explicit `exists: false` alongside the `not_in` condition.

---

## 4. Field reference

Every rule is evaluated once per (resource × container) pair — a Pod with 3
containers gets each condition checked 3 times, once per container, with the
resource-level fields identical across all three. **This means a resource-level-only
rule (e.g. a missing-label governance check) produces one finding per container**,
not one per resource — a known quirk inherited from the original evaluator design,
not something ruleset authors can work around from YAML.

Resources with no containers (a bare Service) still get evaluated once against a
synthetic empty container, so resource-level rules fire correctly for them.

### Resource-level fields

| Field | Type | Notes |
|---|---|---|
| `resource.name` | string | |
| `resource.namespace` | string | |
| `resource.kind` | string | `pod`, `deployment`, `statefulset`, `daemonset`, `job`, `cronjob`, `service`, `configmap`, `secret`, `ingress`, `networkpolicy` |
| `resource.provider` | string | `kubernetes` today |
| `resource.labels.<key>` | string | e.g. `resource.labels.team` |
| `resource.annotations.<key>` | string | e.g. `resource.annotations.example.com/owner` |

### Resource-level security (from `SecurityContext`, aggregated across containers)

| Field | Type |
|---|---|
| `resource.security.run_as_root` | bool |
| `resource.security.privileged` | bool |
| `resource.security.public` | bool |
| `resource.security.allow_privilege_escalation` | bool — only present if at least one container sets it |
| `resource.security.read_only_root_filesystem` | bool — only present if set |
| `resource.security.encrypted` | bool — only present if set (not populated by the Kubernetes provider today) |
| `resource.security.capabilities_add` / `capabilities_drop` | list of strings — only present if non-empty |

### Networking (resource-level exposure)

| Field | Type | True when |
|---|---|---|
| `networking.hostNetwork` | bool | Pod/template `spec.hostNetwork: true` |
| `networking.publicIP` | bool | not populated by the Kubernetes provider today (always false) |
| `networking.nodePort` | bool | Service `type: NodePort` |
| `networking.loadBalancer` | bool | Service `type: LoadBalancer` |

These four support `equals: false` (i.e. "must NOT be exposed this way") because
they're explicitly computed either way. Any other exposure type a future provider
reports is queryable as `networking.<type>` but only supports `equals: true`/
`exists: true` until it's promoted into this known list (see
`internal/resource/networking.go`'s `Flatten`).

### Kubernetes-specific workload attributes

| Field | Type |
|---|---|
| `resource.host_pid` | bool |
| `resource.host_ipc` | bool |

### ConfigMap / Secret / Ingress / NetworkPolicy fields

These four kinds have no containers — every condition against them runs once,
against the synthetic empty container (§4's opening note). They also carry no
`Runtime`, so all their fields live in the resource-level Attributes escape hatch,
not a typed struct:

| Field | Type | Resource kind |
|---|---|---|
| `configmap.data_keys_count` | int | `configmap` |
| `configmap.immutable` | bool | `configmap` |
| `secret.type` | string | `secret` — e.g. `"Opaque"`, `"kubernetes.io/tls"` |
| `secret.data_keys_count` | int | `secret` — a count only; **actual secret data is never queryable, ever** — it's not read into any Resource field |
| `secret.immutable` | bool | `secret` |
| `ingress.tls_enabled` | bool | `ingress` |
| `ingress.rules_count` | int | `ingress` |
| `ingress.class` | string | `ingress` — `""` if `ingressClassName` is unset |
| `networking.Ingress` | bool | `ingress` — via `Networking.Flatten`; only supports `equals: true`/`exists: true` (not yet in the well-known exposure list, see the Networking table above) |
| `networkpolicy.policy_types` | list of strings | `networkpolicy` — e.g. `["Ingress", "Egress"]` |
| `networkpolicy.ingress_rules_count` / `egress_rules_count` | int | `networkpolicy` |
| `networkpolicy.selects_all_pods` | bool | `networkpolicy` — true when `spec.podSelector` is empty (namespace-wide); combined with zero ingress rules this is the standard "default deny" shape |

### Container-level fields

| Field | Type |
|---|---|
| `container.name` | string |
| `container.image` | string |
| `container.user` | int | the container's `runAsUser`; `0` means root |
| `container.privileged` | bool |
| `container.resources.cpu_limit` | string | e.g. `"500m"` — use `exists: false`/`true`, not numeric comparison (no `greater_than` yet) |
| `container.resources.cpu_request` | string | |
| `container.resources.memory_limit` | string | |
| `container.resources.memory_request` | string | |
| `container.security.privileged` | bool | same value as `container.privileged`, exposed under both names |
| `container.security.run_as_root` | bool | |
| `container.security.public` | bool | |
| `container.security.allow_privilege_escalation` | bool | present only if the container sets `allowPrivilegeEscalation` |
| `container.security.read_only_root_filesystem` | bool | present only if the container sets `readOnlyRootFilesystem` |
| `container.security.capabilities_add` / `capabilities_drop` | list of strings | present only if non-empty |
| `container_kind` | string (via `exists`/`equals`) | `"init"` or `"ephemeral"` for those container types; absent (not the empty string) for regular containers |

### Escape hatch

Any field name not in the tables above is looked up directly in the resource's or
container's raw attribute map. This is how provider-specific data (a Docker
bind-mount path, an Ansible task's `become` flag) becomes queryable without a
schema change — write the exact key the provider used as your `field`.

---

## 5. Provider tagging

```yaml
provider: "kubernetes"
```

An empty/omitted `provider` matches every provider a scan runs against. Set it
explicitly once more than one provider exists (today, everything is implicitly
`kubernetes`, but tag rules anyway — untagged rules will silently start matching
Docker/Terraform/etc. resources the moment those providers ship, which is almost
never what you want for a Kubernetes-shaped rule like `container.user`).

`LoadRuleset(dir, activeProvider)` filters at load time: a rule with a `provider`
that doesn't match the active provider is dropped before evaluation, not just
"never matches" — so `infrasight scan docker --ruleset ./rulesets/security-baseline`
(once Docker exists) would load zero rules from that directory, not eleven rules
that all silently fail to match.

---

## 6. Worked examples

**Single condition — flag root containers:**
```yaml
- id: "ISG-SEC-001"
  title: "Container running as root"
  severity: "HIGH"
  provider: "kubernetes"
  condition:
    field: "container.user"
    equals: 0
```

**Missing-value check — flag containers with no CPU limit:**
```yaml
- id: "ISG-RES-001"
  title: "Missing CPU limit"
  severity: "MEDIUM"
  provider: "kubernetes"
  condition:
    field: "container.resources.cpu_limit"
    exists: false
```

**AND — both conditions must hold (highest-risk combination):**
```yaml
- id: "ISG-CI-002"
  title: "Root and privileged"
  severity: "CRITICAL"
  provider: "kubernetes"
  conditions:
    - field: "container.user"
      equals: 0
    - field: "container.privileged"
      equals: true
```

**OR — either exposure type is a finding:**
```yaml
- id: "ISG-STR-008"
  title: "NodePort or LoadBalancer service exposed"
  severity: "HIGH"
  provider: "kubernetes"
  any_of:
    - field: "networking.nodePort"
      equals: true
    - field: "networking.loadBalancer"
      equals: true
```

**Substring match — flag a specific base image family:**
```yaml
- id: "ISG-IMG-001"
  title: "Uses a deprecated base image"
  severity: "LOW"
  provider: "kubernetes"
  condition:
    field: "container.image"
    contains: "legacy-base"
```

**Regex — flag images from any registry other than an approved one:**
```yaml
- id: "ISG-IMG-002"
  title: "Image not pulled from the approved registry"
  severity: "MEDIUM"
  provider: "kubernetes"
  condition:
    field: "container.image"
    regex: "^(?!registry\\.internal\\.example\\.com/)"
```

**`in` — flag a specific set of tags regardless of registry:**
```yaml
- id: "ISG-IMG-003"
  title: "Uses a floating/unpinned tag"
  severity: "MEDIUM"
  provider: "kubernetes"
  condition:
    field: "container.image"
    ends_with: ":latest"
```

**Numeric comparison against a plain attribute (not a quantity string):**
```yaml
- id: "ISG-GOV-004"
  title: "Too many replicas for a dev namespace"
  severity: "LOW"
  provider: "kubernetes"
  condition:
    field: "resource.replica_count"   # hypothetical — only if a provider sets this
    greater_than: 3
```

---

## 7. Validating a new rule before shipping it

No cluster needed — loading and validation happen independently of discovery:

```sh
# Fails fast on bad YAML, missing id/title, invalid severity, or a condition with
# no field, without ever touching a cluster:
infrasight scan kubernetes --ruleset ./rulesets/my-ruleset --kubeconfig /nonexistent
# -> if you see "error: pipeline: kubernetes: discover: ..." your rules were fine
#    (it got past loading and failed at the (expected, in this sandbox) cluster
#    connection step)
# -> if you see "error: parser: ..." or "error: orchestrator: ..." fix the YAML first
```

Or, if you're comfortable with Go, add your ruleset's expected rule count to
`internal/rule/parser/loader_test.go`'s `TestLoadRuleset_AllKubernetesProfiles` —
`go test ./internal/rule/parser/...` then validates it on every future change.

---

## 8. Common pitfalls

- **`exists: false` is not the same as `not_equals`.** `not_equals: 0` matches a
  field that's present *and* different from `0` — it does **not** match a field
  that's absent. Use `exists: false` for "this wasn't set at all."
- **Don't set more than one operator key** on the same condition — only one runs,
  in this precedence (highest first): `not_equals` → `not_in` → `not_contains` →
  `in` → `contains` → `starts_with` → `ends_with` → `regex` →
  `greater_than_or_equal` → `less_than_or_equal` → `greater_than` → `less_than` →
  `equals` (see `conditionOperators` in `internal/rule/evaluator/evaluator.go`,
  checked top-to-bottom by `operatorNameFor`). If you set both `equals` and
  `contains`, `contains` silently wins and `equals` is ignored — write two
  separate conditions instead if you need both checks.
- **`greater_than`/`less_than`/`greater_than_or_equal`/`less_than_or_equal` parse
  the field as a plain number** (`strconv.ParseFloat`), not a Kubernetes quantity —
  see §3's numeric caveat. Using them against `container.resources.cpu_limit` with
  a value like `"500m"` will never match, silently, same as an unknown field would.
- **A malformed `regex` pattern is the one operator input that fails loudly** —
  `ValidateRule` compiles it at load time and rejects the whole ruleset if it
  doesn't parse, rather than letting it silently never-match at scan time like an
  unknown field does.
- **An unknown field name fails silently**, it does not error at load time —
  `resolveField` just returns "not found," so a typo like `containre.user` makes
  the condition permanently false rather than crashing. Double-check field names
  against §4, or grep `internal/rule/evaluator/evaluator.go`'s `resolveField` for
  the authoritative list if this doc drifts.
- **Resource-level rules fire once per container** (see §4's opening note) — a
  3-container Pod missing a `team` label produces 3 findings, not 1. This is worth
  knowing when eyeballing finding counts, not something to "fix" from YAML.
- **`--fail-on` filters findings for the exit code, not the report** — `table`/
  `json`/etc. output always shows every finding regardless of `--fail-on`; only the
  process exit code (and `--exit-1-on-findings`) is affected.
