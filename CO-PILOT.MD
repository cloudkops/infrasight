Project: Kubernetes Security Scanner (Go)

Think of it as a mini Trivy / kube-bench, but simpler and fully yours.

🧭 Final Goal (very clear)

By the end:

A working Go CLI tool
Scans a Kubernetes cluster
Detects real security issues
Clean GitHub repo
You can explain every line
🗂 Project Features (MVP – must have)
Security checks:
Pod running as root
privileged: true
Missing CPU / memory limits
Containers with hostNetwork: true
Services exposed as NodePort / LoadBalancer
Output:
Table output in terminal
JSON export (--output json)
Severity: LOW / MEDIUM / HIGH
🛠 Tech Stack (fixed)

Do not change this:

Language: Go
Kubernetes API: client-go
CLI: cobra
Output formatting: tablewriter
🗓 30-Day Execution Plan
🟢 Week 1 — Go for Systems (not tutorial hell)

You only learn what you need.

Day 1–2

Go modules
Structs & methods
Error handling

Day 3–4

Interfaces
Context
Reading configs

Day 5–7

Build a simple CLI (hello, version, scan)

👉 Deliverable: CLI binary that runs

🟡 Week 2 — Kubernetes internals

Learn only security-relevant parts.

Pods, Containers
SecurityContext
Services
RBAC (basic)

👉 Deliverable: Go code that lists Pods from a cluster

🟠 Week 3 — Scanner logic

Now real engineering starts.

Fetch Pods
Analyze configs
Assign severity
Collect results

👉 Deliverable: Scan result in memory

🔵 Week 4 — Output & polish
Table output
JSON output
Flags & configs
README + examples

👉 Deliverable: Portfolio-ready tool