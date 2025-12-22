# Infrasight

Infrasight is an infrastructure visibility, security, and cost analysis tool by **CloudKops**.

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