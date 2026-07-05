# Infrasight

Infrasight is a CLI tool by **CloudKops** that scans live infrastructure and reports security, governance, and reliability issues against YAML-defined rules — no cluster-side agent, no server, just a binary you run wherever you have credentials.

Today it scans **Kubernetes**. Docker, Terraform, Ansible, and bare-host providers are scaffolded in the codebase but not yet implemented (see [Providers](#providers)).

```
$ infrasight scan kubernetes -A --ruleset rulesets/strict-runtime
RULE ID        SEVERITY  RESOURCE                     FINDING
ISG-STR-002    CRITICAL  pod/prod/payments-api-7f8b   Privileged container
ISG-STR-014    HIGH      pod/prod/payments-api-7f8b   Dangerous capability: SYS_ADMIN
ISG-STR-018    HIGH      clusterrolebinding/edge-sa   Binds cluster-admin

3 findings (1 CRITICAL, 2 HIGH) across 214 resources, 19 rules evaluated.
```

## How it works

```
provider.Discover()  →  resource.Resource  →  rule engine (resource × container × rule)  →  report.Finding  →  renderer
```

1. **Providers** (`providers/*`) connect to real infrastructure and discover resources — for Kubernetes, that means talking to the API server via `client-go`.
2. Discovered objects are normalized into a provider-agnostic `resource.Resource` shape (metadata, containers, security context, networking, arbitrary attributes) so the rule engine never needs to know about Kubernetes types directly.
3. The **rule engine** (`internal/rule/*`) evaluates every resource (and, where relevant, every container within it) against every loaded rule, using an extensible operator registry (equals, contains, regex, numeric comparisons, etc.).
4. Matches become `report.Finding`s, rendered as a table, JSON, SARIF, or Markdown, with a severity-based process exit code for CI gating.

Providers, rule operators, and output renderers are each pluggable via a self-registering `init()` pattern — adding a new one doesn't require touching the CLI, the dispatcher, or each other.

## Install

Requires Go 1.25+.

```bash
git clone https://github.com/cloudkops/infrasight.git
cd infrasight
make build       # → bin/infrasight
```

Or install straight into `$GOBIN`:

```bash
make install
```

## Usage

```bash
# Scan the current namespace with a single rules file
infrasight scan kubernetes -n default --rules rulesets/security-baseline/rules.yaml

# Scan every namespace against a whole ruleset directory
infrasight scan kubernetes -A --ruleset rulesets/strict-runtime

# Filter to a label selector, write JSON to a file, gate CI on HIGH+
infrasight scan kubernetes -A -l app=payments --ruleset rulesets/strict-runtime \
  --format json --output findings.json --fail-on HIGH

# Scan Docker containers/images/volumes/networks on this machine
infrasight scan docker --ruleset rulesets/docker-baseline

# Scan systemd units and listening processes on this machine (run as root for complete results)
sudo infrasight scan host --ruleset rulesets/host-baseline

# List registered providers
infrasight providers list

# Print version / commit
infrasight version
```

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--rules` | `-r` | `""` | Single rules YAML file |
| `--ruleset` | | `""` | Directory of rule YAML files |
| `--format` | `-f` | `table` | `table`, `json`, `sarif`, or `markdown` |
| `--fail-on` | | `LOW` | Minimum severity that sets a non-zero exit code |
| `--exit-1-on-findings` | | `false` | Exit 1 on any finding, regardless of severity |
| `--output` | `-o` | `""` | Write findings to a file instead of stdout |
| `--verbose` | `-v` | `false` | Print debug info to stderr |
| `--namespace` | `-n` | `""` | Namespace (Kubernetes; empty = all) |
| `--all-namespaces` | `-A` | `false` | Scan every namespace (Kubernetes) |
| `--label-selector` | `-l` | `""` | Label selector (Kubernetes) |
| `--field-selector` | | `""` | Field selector (Kubernetes) |
| `--kubeconfig` | | `""` | Path to kubeconfig | 
| `--context` | | `""` | Kubeconfig context to use |
| `--docker-host` | | `""` | Docker daemon socket/host (empty = `DOCKER_HOST` env or default socket; Docker) |

Exit code is severity-ranked: 0 (nothing at/above `--fail-on`), or a code reflecting the highest severity found — see `internal/report/exitcode.go`.

## Providers

| Provider | Status | Kind |
|---|---|---|
| `kubernetes` | Implemented | Live |
| `docker` | Implemented | Live |
| `host` | Implemented | Live |
| `terraform` | Scaffolded, not implemented | Static |
| `ansible` | Scaffolded, not implemented | Static |

The Kubernetes provider discovers Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, Services, ConfigMaps, Secrets, Ingresses, NetworkPolicies, ClusterRoleBindings, and RoleBindings, fetched concurrently. Per-container attributes include user/privileged/capabilities, resource limits, probes, image, and secret-sourced env vars; per-resource attributes include host namespace usage (`hostNetwork`/`hostPID`/`hostIPC`), hostPath volumes, service account token automount, and (for RBAC bindings) whether they bind the `cluster-admin` ClusterRole.

RBAC findings are reported as standalone `clusterrolebinding`/`rolebinding` resources — there is no Pod → ServiceAccount → Binding resolution yet, so a finding tells you *a binding grants cluster-admin*, not *which running pods can use it*.

The Docker provider discovers containers, images, volumes, and networks via the Docker Engine API (`DOCKER_HOST` or `--docker-host`). A container maps to a Resource wrapping exactly one nested Container — deliberately mirroring the Kubernetes Pod/Container split — so `container.*`/`container.security.*` rule fields resolve the same way for both providers. Per-container attributes include privileged/capabilities/read-only-rootfs, docker-socket mounts, and security options; resource-level attributes cover host PID/IPC namespace sharing and `--network=host`; Compose-labeled containers report their compose-service as `Owner`. See `docs/docker.md` for the full field/attribute reference.

The Host provider scans only the local Linux machine InfraSight is running on (no remote/SSH fan-out) via `/proc` and `systemctl`. It discovers systemd `.service` units (root/privilege-escalation/capability/namespace hardening flags under `resource.security.*` and `systemd.*`) plus any process listening on a TCP port that isn't attributable to a systemd unit (reported standalone as `Kind: "process"`, so nothing exposed is silently dropped). Unlike Docker/Kubernetes, host resources have no nested-container concept at all — `Security` lives directly on the Resource. Full results require running as root (socket-ownership correlation needs access to other users' `/proc/<pid>/fd`). See `docs/host.md`.

## Rules and rulesets

A rule is a YAML document matched against every resource (and container, for container-scoped fields):

```yaml
- id: "ISG-STR-002"
  title: "Privileged container"
  severity: "CRITICAL"
  category: "security"
  description: "Container runs with securityContext.privileged: true."
  remediation: "Remove privileged: true; grant only the specific capabilities needed."
  provider: "kubernetes"
  condition:
    field: "container.privileged"
    equals: true
```

Three condition modes, same as above but for multi-field rules:

```yaml
conditions:          # ALL must match
  - field: "container.user"
    equals: 0
  - field: "container.privileged"
    equals: true

any_of:               # ANY must match
  - field: "networking.hostNetwork"
    equals: true
  - field: "networking.publicIP"
    equals: true
```

### Operators

`equals`, `not_equals`, `exists`, `contains`, `not_contains`, `starts_with`, `ends_with`, `regex`, `in`, `not_in`, `greater_than`, `less_than`, `greater_than_or_equal`, `less_than_or_equal`.

`contains`/`not_contains` do substring matching on strings and membership matching on string lists (e.g. `container.security.capabilities_add`). Numeric operators parse plain `float64` values only — Kubernetes quantity suffixes like `500m` or `1Gi` are not parsed as numbers. Regex patterns are validated at load time, so a malformed pattern fails the scan up front rather than silently never matching. Full operator semantics and precedence (when a condition sets more than one operator key) are in `docs/rulesets_manual.md`.

### Bundled rulesets

| Ruleset | Rules | Purpose |
|---|---|---|
| `security-baseline` | 11 | General security/governance/resource baseline (Kubernetes) |
| `dev-baseline` | 6 | Lower-severity guardrails for dev clusters (Kubernetes) |
| `strict-runtime` | 19 | Hardened production controls (root, privileged, host namespaces, image tags, probes, capabilities, hostPath, secret env vars, cluster-admin bindings, ...) (Kubernetes) |
| `ci-critical` | 2 | CRITICAL-only gate for CI pipelines (Kubernetes) |
| `docker-baseline` | 25 | Container/image/network/volume security and governance checks (Docker) |
| `host-baseline` | 11 | systemd-unit hardening flags and unmanaged listening processes (Host) |

Point `--ruleset` at any directory of rule YAML files to build your own.

## Development

```bash
make build       # build bin/infrasight
make run ARGS="scan kubernetes -A"   # build and run
make test        # go test ./...
make test-race   # go test -race ./...
make coverage    # coverage.html
make fmt         # gofmt -w
make fmt-check   # fail if gofmt would change anything
make vet         # go vet
make lint        # golangci-lint (must be installed separately)
make ci          # fmt-check + vet + test-race + build
make tidy        # go mod tidy
make clean       # remove build artifacts
```

`make help` lists all targets.

### Layout

```
cmd/infrasight/        entrypoint; wires in providers and renderers via blank imports
internal/app/           cobra command tree, flag binding, dispatch
internal/provider/      Provider interface + registry
internal/resource/      provider-agnostic resource/container model
internal/rule/parser/   Rule/Condition types, YAML loading, validation
internal/rule/evaluator/ operator registry + field resolution
internal/rule/engine/   resource × container × rule evaluation
internal/report/        Finding type, renderer registry, exit codes
internal/report/{json,sarif,table,markdown}/  output renderers
internal/scan/orchestrator/  ties provider discovery + rule loading + evaluation together
providers/{kubernetes,docker,host}/  implemented providers
providers/{terraform,ansible}/  scaffolding for future providers
rulesets/               bundled rule YAML profiles
docs/                   architecture, per-provider, and rulesets manuals
```

See `docs/high_level_architecture.md`, `docs/k8s.md`, `docs/docker.md`, `docs/host.md`, and `docs/rulesets_manual.md` for deeper detail on each layer.
