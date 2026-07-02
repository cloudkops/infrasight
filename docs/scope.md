# Infrasight – Scope & Areas of Improvement

## Current Scope

Infrasight is currently an **early-stage MVP** (v0.0.1) focused on:

1. **Kubernetes workload scanning** – Pod, Deployment, and Service resources
2. **YAML-defined rule evaluation** – Single, compound AND, and compound OR conditions
3. **Three output formats** – Table, JSON, SARIF 2.1.0
4. **CI/CD integration** – Exit codes based on severity thresholds
5. **Four pre-built rulesets** – Security baseline, dev baseline, strict runtime, CI critical

---

## Areas of Improvement

### 1. Kubernetes Adapter Gaps

| Area | Current State | Improvement Needed |
|---|---|---|
| **Resource types** | Only Pods, Deployments, Services | Add StatefulSets, DaemonSets, Jobs, CronJobs, ReplicaSets |
| **Init containers** | Ignored entirely | Map `spec.initContainers` to model |
| **Ephemeral containers** | Not supported | Map `spec.ephemeralContainers` |
| **Pod security context** | Only pod-level `HostNetwork` | Add `RunAsNonRoot`, `ReadOnlyRootFilesystem`, `AllowPrivilegeEscalation`, `Capabilities`, `SELinux` |
| **Container security context** | Only `RunAsUser` and `Privileged` | Add `RunAsNonRoot`, `ReadOnlyRootFilesystem`, `AllowPrivilegeEscalation`, `Capabilities.Add/Drop`, `SeccompProfile` |
| **NodePort exposure** | Only `NodePort` type Services | Add Ingress resources, ExternalIPs |
| **Label selectors** | No filtering | Add `--label-selector` and `--field-selector` flags |
| **Multi-cluster** | Single cluster only | Support kubeconfig with multiple contexts |
| **Client authentication** | Only in-cluster and default kubeconfig | Add `--kubeconfig` flag, OIDC, exec-based auth |

### 2. Rule Engine Limitations

| Area | Current State | Improvement Needed |
|---|---|---|
| **Operators** | `equals`, `not_equals`, `contains`, `exists` | Add `greater_than`, `less_than`, `in`, `not_in`, `regex`, `starts_with`, `ends_with` |
| **Numeric comparisons** | Only exact equality for `int64` | Add `greater_than`, `less_than`, `greater_than_or_equal`, `less_than_or_equal` for resource values |
| **Resource value parsing** | String comparison only (`"500m"`) | Parse CPU/memory strings to numeric for proper comparison (e.g., `500m > 250m`) |
| **Nested conditions** | Single level of AND/OR | Add nested compound conditions (AND of ORs, OR of ANDs) |
| **Negation** | `not_equals` only | Add `not` operator to negate entire condition blocks |
| **Regex matching** | Not supported | Add `matches` operator with regex support |
| **Array/set operations** | Not supported | Add `contains_any`, `contains_all` for label/annotation map checks |
| **Rule inheritance** | Not supported | Allow rules to extend/override base rules |
| **Rule groups/tags** | Not supported | Add tagging system for selective rule execution |
| **Conditional execution** | Not supported | Add `when` clauses to skip rules based on workload properties |

### 3. Output & Reporting Gaps

| Area | Current State | Improvement Needed |
|---|---|---|
| **Output formats** | Table, JSON, SARIF | Add CSV, HTML, JUnit XML, GitHub Actions annotation format |
| **Summary report** | Basic severity counts | Add per-namespace breakdown, per-rule breakdown, trend data |
| **Finding deduplication** | Not implemented | Deduplicate findings for same workload+container+rule |
| **Finding grouping** | Not implemented | Group findings by severity, namespace, workload, or rule |
| **Remediation output** | Inline in table only | Dedicated `--remediation` flag for separate remediation report |
| **Baseline/diff mode** | Not supported | Compare scan results against previous scan, show regressions |
| **SARIF output** | Basic compliance | Add `runs[].tool.driver.rules[].defaultConfiguration.level`, `relationships`, `fixes` |

### 4. CLI & UX Improvements

| Area | Current State | Improvement Needed |
|---|---|---|
| **Configuration file** | None | Support `infrasight.yaml` config file for defaults |
| **Dry-run mode** | Not supported | `--dry-run` to show what would be scanned without executing |
| **Progress indication** | None | Add progress bar/spinner for large clusters |
| **Color output** | None (table only) | Color-coded severity in terminal output |
| **Interactive mode** | Not supported | Interactive rule builder or scanner |
| **Shell completion** | Not implemented | `infrasight completion bash/zsh/fish/powershell` |
| **Man pages** | Not implemented | Generate man pages from cobra commands |
| **JSON output pretty** | Only with `--format json` | Auto-detect TTY for pretty vs compact output |

### 5. Testing Gaps

| Area | Current State | Improvement Needed |
|---|---|---|
| **Unit test coverage** | Core rules engine covered | Add tests for `cmd/isg/` pipeline, `core/engine/`, report renderers |
| **Integration tests** | None | End-to-end tests with real YAML files and mock workloads |
| **Kubernetes adapter tests** | Only mapper tests | Add tests for `client.go`, `pod.go`, `deployment.go`, `service.go` |
| **Table-driven tests** | Partially used | Standardize all tests to table-driven format |
| **Benchmark tests** | None | Add benchmarks for rule evaluation at scale |
| **Test fixtures** | Single test file | Create comprehensive fixtures for all rule types and edge cases |
| **CI pipeline** | Not defined | Add GitHub Actions workflow with lint, test, build, release |

### 6. Performance & Scalability

| Area | Current State | Improvement Needed |
|---|---|---|
| **Concurrency** | Sequential scanning | Use goroutines with worker pool for parallel workload scanning |
| **Memory usage** | Loads all workloads into memory | Stream processing for large clusters |
| **Caching** | None | Cache K8s API responses for repeated scans |
| **Pagination** | Not implemented | Handle large result sets with K8s API pagination |
| **Rate limiting** | Not implemented | Add K8s API rate limiting to avoid throttling |

### 7. Security & Hardening

| Area | Current State | Improvement Needed |
|---|---|---|
| **Secret scanning** | Not supported | Detect secrets in environment variables, volumes |
| **Image vulnerability scanning** | Not supported | Integrate with Trivy, Grype, or Snyk |
| **OPA/Rego support** | Not supported | Allow OPA policies alongside YAML rules |
| **RBAC analysis** | Not supported | Detect overly permissive ClusterRoleBindings |
| **Network policy analysis** | Not supported | Detect missing NetworkPolicies |
| **Admission controller integration** | Not supported | Webhook mode for pre-deployment validation |

### 8. Extensibility & Integration

| Area | Current State | Improvement Needed |
|---|---|---|
| **Plugin system** | Not supported | Allow custom adapters, evaluators, and reporters |
| **API server mode** | Not supported | HTTP API for remote scanning and result retrieval |
| **Webhook notifications** | Not supported | Send findings to Slack, PagerDuty, Jira |
| **Dashboard** | Not supported | Web UI for visualizing findings and trends |
| **Terraform/Pulumi integration** | Not supported | Scan IaC-generated resources |
| **Docker adapter** | Not supported | Scan local Docker containers |
| **VM adapter** | Not supported | Scan VM workloads (model.Platform exists but unused) |

### 9. Rule Management

| Area | Current State | Improvement Needed |
|---|---|---|
| **Rule registry** | Local filesystem only | Central registry for sharing rules across teams |
| **Rule versioning** | Not supported | Version rules with backward compatibility |
| **Rule signing** | Not supported | Cryptographic signing for trusted rules |
| **Rule templating** | Not supported | Parameterized rules (e.g., `{{.namespace}}`) |
| **Rule import/export** | Not supported | Export rules to/from different formats (Rego, Regal) |
| **Rule documentation** | Inline YAML only | Auto-generate rule documentation from YAML |

### 10. Deprecated Code Cleanup

| File | Status | Action |
|---|---|---|
| `core/policy/exitcode.go` | Deprecated (3 lines) | Remove entirely – logic lives in `core/report/exitcode.go` |

---

## Priority Matrix

### P0 – Critical (Must Have)
- Add StatefulSet, DaemonSet, Job, CronJob support
- Expand container security context (RunAsNonRoot, ReadOnlyRootFilesystem, Capabilities)
- Add numeric comparison operators (greater_than, less_than)
- Add `--kubeconfig` flag
- Fix duplicate workload scanning (Pod + Deployment for same workload)
- Remove deprecated `core/policy/` package

### P1 – High (Should Have)
- Add CSV and JUnit XML output formats
- Add shell completion
- Add configuration file support
- Add concurrency for large cluster scanning
- Add integration test suite
- Add GitHub Actions CI pipeline

### P2 – Medium (Nice to Have)
- Add regex operator
- Add Ingress resource scanning
- Add baseline/diff mode
- Add rule tagging and filtering
- Add SARIF enhancements
- Add `--label-selector` filtering

### P3 – Low (Future)
- Plugin system
- API server mode
- Web dashboard
- OPA/Rego support
- Docker/VM adapters
- Rule registry and versioning

---

## Technical Debt

1. **Duplicate workload scanning**: When scanning a namespace, both Pods and Deployments are loaded. A Deployment's Pods are scanned twice – once as Pod workloads, once as Deployment workloads. Need deduplication or relationship mapping.

2. **No `--kubeconfig` flag**: The K8s client hardcodes `~/.kube/config` fallback. Should accept explicit kubeconfig path and context.

3. **Resource string comparison**: Resource values are compared as raw strings (`"500m"`, `"256Mi"`), which breaks for cross-unit comparisons. Need proper resource quantity parsing.

4. **Version hardcoded**: Version `0.0.1` is hardcoded in both `cmd/isg/version.go` and `core/report/sarif.go`. Should be injected via ldflags consistently.

5. **Deprecated package**: `core/policy/exitcode.go` is a dead package marked deprecated. Should be removed.

6. **No error handling in renderers**: `renderFindings()` in `scan_pipeline.go` discards errors from `WriteSARIF()` and `WriteJSON()` with `_ =`. Errors should be propagated.

7. **Missing `init container` mapping**: The Kubernetes adapter only maps `spec.containers`, ignoring `spec.initContainers` and `spec.ephemeralContainers`.

8. **No validation for unknown fields**: The evaluator silently returns `false` for unknown fields. Should log a warning or error in verbose mode.

9. **Inconsistent naming**: The `Finding` struct uses `json:"rule_id"` (snake_case) while Go convention is PascalCase. The `Rule` struct uses `yaml:"docs_url"` but the field is `DocsUrl`.

10. **Missing context propagation**: All K8s API calls use `context.Background()`. Should accept context for cancellation and timeout support.
