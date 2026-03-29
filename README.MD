# Infrasight

Infrasight is an infrastructure visibility, security, and cost analysis tool by **CloudKops**.

## Scan Manual

InfraSight now supports decentralized scan entry points:

- `infrasight scan` (umbrella command)
- `infrasight scan k8s` (explicit Kubernetes command)

### Command Behavior

- `scan`: generic scan entry point. Use flags to choose behavior.
- `scan --k8s`: enables Kubernetes scan mode from the umbrella command.
- `scan k8s`: forces Kubernetes scan mode without needing `--k8s`.

### Common Flags

- `--rules, -r`: path to a single rules YAML file.
- `--ruleset`: path to a directory containing `.yaml` rule files.
- `--format, -f`: output format (`table` or `json`).
- `--fail-on`: severity threshold for exit code (`LOW`, `MEDIUM`, `HIGH`, `CRITICAL`).
- `--ci`: prints JSON output intended for CI/CD.

### Kubernetes Flags

- `--namespace, -n`: namespace to scan in Kubernetes mode.
- Default namespace: `kube-system`.

### Real-Life Ruleset Profiles

- `rulesets/security-baseline`: baseline controls for root and privileged containers.
- `rulesets/dev-baseline`: lower-severity developer-oriented guardrails.
- `rulesets/strict-runtime`: hardened runtime controls for production workloads.
- `rulesets/ci-critical`: strict CI gate focused on critical privilege violations.

### Examples

```sh
# File-based rules scan (existing behavior)
infrasight scan --rules ./test/001_ruleset_001.yaml --format json

# Kubernetes scan via umbrella command
infrasight scan --k8s --ruleset ./rulesets/security-baseline --namespace kube-system

# Kubernetes scan via explicit subcommand
infrasight scan k8s --ruleset ./rulesets/security-baseline -n kube-system

# CI gate example (fails only on critical privilege risk)
infrasight scan k8s --ruleset ./rulesets/ci-critical --fail-on CRITICAL --ci
```

## Milestones
- Day 4 – Rule Engine Working
- Day 5 
  - 1️⃣ YAML rules are successfully loaded
  - 2️⃣ Rule engine evaluates multiple rule types
  - 3️⃣ Findings are structured & deterministic
  - 4️⃣ Output is CI/CD & API ready
  - 5️⃣ CLI behaves like a real security tool

  ## Example Day 5

      ```sh
      infrasight scan --rules ./test/001_ruleset_001.yaml --format json
      ```
      ```json
      [
        {
          "rule_id":"ISG-SEC-001",
          "title":"Container running as root",
          "severity":"HIGH",
          "workload":"App",
          "container":"nginx",
          "field":"container.user"
        },
        {
          "rule_id":"ISG-SEC-002",
          "title":"Privileged container",
          "severity":"CRITICAL",
          "workload":"DB",
          "container":"mysql",
          "field":"container.privileged"
          }
        ]
      ```