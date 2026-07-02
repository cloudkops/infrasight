# Infrasight – Project Overview

## What is Infrasight?

**Infrasight** is an infrastructure visibility, security, and cost analysis CLI tool built by **CloudKops** (`github.com/cloudkops/infrasight`). Written in Go, it scans Kubernetes workloads (Pods, Deployments, Services) against YAML-defined security, governance, and resource rules, then produces structured findings in table, JSON, or SARIF format.

---

## Architecture

```
CLI (cmd/isg/)                    Adapters (adapters/)
  scan.go                           kubernetes/
  scan_pipeline.go                    client.go      → K8s API
  scan_options.go                     pod.go         → PodList
  scan_sources.go                     deployment.go  → DeploymentList
                                      service.go     → ServiceList
                                      mapper.go      → K8s objects → model.Workload
        │
        ▼
   Core Engine (core/engine/)
     engine.go  → Scan(ScanRequest) ScanResult
        │
        ▼
   Rule Evaluator (core/rules/)
     evaluator.go  → Evaluate(workloads, rules) []Finding
     loader.go     → Load(file) []Rule
     ruleset.go    → LoadRuleset(dir) []Rule
     validate.go   → ValidateRule / ValidateRules
     rule.go       → Rule, Condition structs
     severity.go   → Severity constants & ranking
        │
        ▼
   Report (core/report/)
     finding.go  → Finding struct
     json.go     → WriteJSON
     sarif.go    → WriteSARIF (SARIF 2.1.0)
     table.go    → WriteTable
     summary.go  → WriteSummary
     exitcode.go → ExitCode (severity-based)
```

**Data flow:** K8s API → Adapters → `model.Workload` → Rule Loader → Evaluator → Findings → Report Renderers + Exit Code

---

## Directory Structure

```
infrasight/
├── main.go                         # Entry point → calls isg.Execute()
├── go.mod / go.sum                 # Module: github.com/cloudkops/infrasight
├── Makefile                        # Build, test, lint, coverage, clean
├── README.MD                       # User-facing docs
├── cmd/isg/
│   ├── root.go                     # Root cobra command
│   ├── scan.go                     # `scan` + `scan k8s` subcommands
│   ├── scan_pipeline.go            # Full scan orchestration
│   ├── scan_options.go             # CLI flag definitions
│   ├── scan_sources.go             # Workload loading dispatch
│   └── version.go                  # `version` subcommand (0.0.1)
├── core/
│   ├── engine/engine.go            # Scan orchestrator
│   ├── model/
│   │   ├── workload.go             # Workload struct
│   │   ├── container.go            # Container struct
│   │   ├── resource.go             # Resource struct (CPU/memory)
│   │   ├── metadata.go             # Metadata struct (labels/annotations)
│   │   └── exposure.go             # Exposure struct (port/hostNetwork/publicIP)
│   ├── rules/
│   │   ├── rule.go                 # Rule + Condition structs
│   │   ├── severity.go             # Severity constants, ranking, validation
│   │   ├── evaluator.go            # Core evaluation engine
│   │   ├── loader.go               # Single YAML file loader
│   │   ├── ruleset.go              # Directory-based ruleset loader
│   │   └── validate.go             # Rule validation
│   ├── report/
│   │   ├── finding.go              # Finding struct
│   │   ├── json.go                 # JSON output
│   │   ├── sarif.go                # SARIF 2.1.0 output
│   │   ├── table.go                # Table output (tablewriter)
│   │   ├── summary.go              # Human-readable summary
│   │   └── exitcode.go             # Severity-based exit code
│   └── policy/exitcode.go          # Deprecated stub
├── adapters/kubernetes/
│   ├── client.go                   # K8s client (in-cluster + kubeconfig)
│   ├── pod.go                      # Pod listing
│   ├── deployment.go               # Deployment listing + mapping
│   ├── service.go                  # Service listing + mapping (NodePort/LoadBalancer)
│   └── mapper.go                   # K8s → model mapping
├── rulesets/                       # Pre-built ruleset profiles
│   ├── security-baseline/          # 11 rules (security, governance, resources)
│   ├── dev-baseline/               # 6 rules (lower-severity dev guardrails)
│   ├── strict-runtime/             # 11 rules (hardened production controls)
│   └── ci-critical/                # 4 rules (CRITICAL-only CI gate)
└── test/
    └── 001_ruleset_001.yaml        # Test fixture (2 rules)
```

---

## Core Data Structures

### Model Layer (`core/model/`)

| Struct | Fields | Purpose |
|---|---|---|
| `Workload` | ID, Name, Namespace, Type, Platform, Containers, Metadata | Unit of infrastructure to scan |
| `Container` | Name, Image, User (UID), Privileged, Resources, Exposures | Single container in a workload |
| `Resource` | CPURequest, CPULimit, MemoryRequest, MemoryLimit | CPU/memory constraints |
| `Metadata` | Labels, Annotations (maps) | K8s metadata |
| `Exposure` | Type (port/hostNetwork/publicIP/NodePort), Port | Network exposure |

### Rules Layer (`core/rules/`)

| Struct | Fields | Purpose |
|---|---|---|
| `Rule` | ID, Title, Severity, Category, Description, Remediation, DocsUrl, Condition, Conditions, AnyOf | Security/compliance/governance check |
| `Condition` | Field, Equals, NotEquals, Contains, Exists | Field-match predicate |

Three condition modes:
- **Single** (`condition`): one condition (backward-compatible)
- **Compound AND** (`conditions`): ALL must match
- **Compound OR** (`any_of`): ANY must match

### Report Layer (`core/report/`)

| Struct | Fields | Purpose |
|---|---|---|
| `Finding` | RuleID, Title, Severity, Category, Description, Remediation, DocsUrl, Workload, Container, Field | Output of a rule match |

---

## CLI Commands & Flags

### Commands

| Command | Description |
|---|---|
| `infrasight scan` | Generic scan entry point (use `--k8s` for Kubernetes) |
| `infrasight scan k8s` | Explicit Kubernetes scan mode |
| `infrasight version` | Prints version (0.0.1) |

### Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--rules` | `-r` | `""` | Single rules YAML file path |
| `--ruleset` | | `""` | Directory of `.yaml` rule files |
| `--format` | `-f` | `"table"` | Output: `table`, `json`, `sarif` |
| `--fail-on` | | `"LOW"` | Min severity for non-zero exit (`LOW\|MEDIUM\|HIGH\|CRITICAL`) |
| `--output` | `-o` | `""` | Write findings to file |
| `--k8s` | | `false` | Enable Kubernetes scan mode |
| `--namespace` | `-n` | `""` | K8s namespace (empty = all) |
| `--all-namespaces` | `-A` | `false` | Scan all namespaces |
| `--ci` | | `false` | CI mode (forces JSON output) |
| `--verbose` | `-v` | `false` | Debug logging |
| `--exit-1-on-findings` | | `false` | Exit 1 on any finding |

---

## Rule Engine

The evaluator (`core/rules/evaluator.go`) performs a cross-product scan: for every workload × container × rule, it checks if the rule matches.

### Supported Fields

| Field | Type | Operators |
|---|---|---|
| `container.user` | int64 | equals, not_equals |
| `container.privileged` | bool | equals, not_equals |
| `container.image` | string | equals, not_equals, contains |
| `container.name` | string | equals, not_equals, contains |
| `container.resources.cpu_limit` | string | exists, equals, not_equals |
| `container.resources.memory_limit` | string | exists, equals, not_equals |
| `container.resources.cpu_request` | string | exists, equals, not_equals |
| `container.resources.memory_request` | string | exists, equals, not_equals |
| `container.exposure.hostNetwork` | bool | equals |
| `container.exposure.publicIP` | bool | equals |
| `container.exposure.NodePort` | bool | equals |
| `workload.name` | string | equals, not_equals, contains |
| `workload.namespace` | string | equals, not_equals, contains |
| `workload.platform` | string | equals, not_equals, contains |
| `workload.type` | string | equals, not_equals, contains |
| `workload.labels.<key>` | string | equals, not_equals, contains |
| `workload.annotations.<key>` | string | equals, not_equals, contains |

---

## Rule YAML Schema

```yaml
- id: "ISG-SEC-001"                    # REQUIRED – unique identifier
  title: "Container running as root"   # REQUIRED
  severity: "HIGH"                     # REQUIRED – LOW | MEDIUM | HIGH | CRITICAL
  category: "security"                 # Optional
  description: "..."                   # Optional
  remediation: "..."                   # Optional
  docs_url: "https://..."             # Optional

  # Mode 1: Single condition (backward-compatible)
  condition:
    field: "container.user"
    equals: 0

  # Mode 2: Compound AND – ALL conditions must match
  conditions:
    - field: "container.user"
      equals: 0
    - field: "container.privileged"
      equals: true

  # Mode 3: Compound OR – ANY condition must match
  any_of:
    - field: "container.exposure.hostNetwork"
      equals: true
    - field: "container.exposure.publicIP"
      equals: true
```

### Condition Operators

| Operator | Type | Description |
|---|---|---|
| `equals` | string/int/bool | Exact match |
| `not_equals` | string/int/bool | Inverse match |
| `contains` | string | Substring match |
| `exists` | bool | Check if field is set/unset (for resources) |

---

## Ruleset Profiles

| Profile | Rules | Purpose |
|---|---|---|
| `security-baseline` | 11 (security, governance, resources) | Standard baseline for root, privileged, labels, resource limits |
| `dev-baseline` | 6 | Lower-severity developer-oriented guardrails |
| `strict-runtime` | 11 | Hardened controls for production workloads |
| `ci-critical` | 4 | Strict CI gate – only CRITICAL privilege violations |

### Rule Categories

- **Security** (container): root user (HIGH), privileged (CRITICAL), hostNetwork (HIGH), publicIP (CRITICAL)
- **Resources** (reliability): missing CPU limit (MEDIUM), missing memory limit (MEDIUM), missing CPU request (LOW), missing memory request (LOW)
- **Governance** (labels): missing team/env/app labels (LOW)

---

## Kubernetes Adapter

- **Client creation**: tries in-cluster config first, falls back to `~/.kube/config`
- **Pods**: maps to `Workload{Type: "pod"}` with container security context (RunAsUser, Privileged), resources, and port exposures
- **Deployments**: maps to `Workload{Type: "deployment"}` from template spec
- **Services**: maps to `Workload{Type: "service"}` with a virtual container carrying NodePort/LoadBalancer exposures

---

## Output Formats

| Format | Description |
|---|---|
| **table** | Formatted table via `tablewriter` with remediation section |
| **json** | JSON array (compact or pretty) |
| **sarif** | Full SARIF 2.1.0 with deduplicated rules, severity-mapped levels |

---

## Exit Code Logic

| Code | Meaning |
|---|---|
| 0 | No findings meet the `--fail-on` threshold |
| 1 | LOW severity finding |
| 2 | MEDIUM severity finding |
| 3 | HIGH or CRITICAL severity finding |

---

## Build & Development

```bash
make build          # → bin/infrasight
make test           # go test ./...
make coverage       # coverage report (HTML)
make lint           # golangci-lint
make tidy           # go mod tidy
make install        # install to $GOBIN
make clean          # remove artifacts
```

### Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/olekukonko/tablewriter` | v1.1.4 | Table rendering |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing |
| `k8s.io/client-go` | v0.35.0 | Kubernetes client |
| `k8s.io/api` | v0.35.0 | Kubernetes API types |
| `k8s.io/apimachinery` | v0.35.0 | Kubernetes API machinery |

---

## Tests

| Test File | What It Tests |
|---|---|
| `core/rules/evaluator_test.go` | Full evaluation engine: all field types, type coercion, end-to-end matching |
| `core/rules/compound_test.go` | Compound conditions: AND, OR, single fallback, not_equals, contains |
| `core/rules/severity_test.go` | Severity normalization, validation, ranking, threshold comparison |
| `core/rules/validate_test.go` | Rule validation: missing ID/title, invalid severity, missing condition |
| `core/rules/loader_test.go` | YAML loading: valid/invalid files, directory loading, non-YAML skipping |
| `core/report/exitcode_test.go` | Exit code computation: threshold filtering, severity ranking, case insensitivity |
| `adapters/kubernetes/mapper_test.go` | K8s-to-model mapping: security context, resources, exposures, multiple containers |
