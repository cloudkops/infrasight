# InfraSight Architecture Improvement Proposal

> **Objective:** Evolve InfraSight from a Kubernetes-specific scanner into a modular Infrastructure Analysis Platform capable of scanning Kubernetes, Docker, Linux Hosts, Cloud Providers, Terraform, and future infrastructure targets without modifying the core scanning engine.

---

# Current Architecture

```
                     CLI

                      │

                 Cobra Commands

                      │

                      ▼

              Kubernetes Scanner

                      │

                      ▼

                Rule Engine

                      │

                      ▼

                 Report Output
```

## Current Limitations

* Scanner is tightly coupled to Kubernetes.
* Rule engine implicitly depends on Kubernetes resource structures.
* Adding a new infrastructure target requires modifying the scanning pipeline.
* No unified abstraction for infrastructure resources.
* Provider lifecycle is unmanaged.
* Difficult to support third-party integrations.

---

# Design Goals

The future architecture should satisfy the following principles:

* Provider-agnostic scanning engine.
* Pluggable infrastructure providers.
* Unified resource model.
* Extensible reporting system.
* Clear separation of responsibilities.
* Easy onboarding of new providers.
* Future compatibility with external plugins.

---

# High-Level Architecture

```
                               CLI

                                │

                         Cobra Commands

                                │

                                ▼

                        Command Dispatcher

                                │

                                ▼

                         Scan Orchestrator

                                │

                                ▼

                        Provider Manager

        ┌───────────────┼───────────────┼───────────────┐

        ▼               ▼               ▼

   Kubernetes        Docker         Linux Host

        ▼               ▼               ▼

   Discovery       Discovery       Discovery

        ▼               ▼               ▼

             Resource Normalization

                        │

                        ▼

                 Generic Resources

                        │

                        ▼

                  Rule Evaluation

                        │

                        ▼

                     Findings

                        │

                        ▼

                    Report Engine

        ┌──────────────┼──────────────┐

        ▼              ▼              ▼

      Table          JSON          SARIF
```

---

# Layer Responsibilities

---

## CLI Layer

Responsible for:

* Parsing CLI arguments
* Loading configuration
* Initializing the application

Should **never** contain scanning logic.

---

## Command Dispatcher

Responsible for:

* Selecting the correct workflow.
* Validating arguments.
* Invoking the Scan Orchestrator.

Example:

```
infrasight scan k8s

↓

Dispatch ScanRequest
```

---

## Scan Orchestrator

Acts as the coordinator.

Responsibilities:

* Load provider
* Initialize provider
* Start discovery
* Execute rule engine
* Produce report
* Shutdown provider

The orchestrator should know **nothing** about Kubernetes or Docker.

---

# Provider Layer

Providers represent infrastructure sources.

Examples:

```
Kubernetes

Docker

Linux Host

Terraform

AWS

Azure

GCP

Helm

Nomad
```

Every provider should expose a common interface.

Example responsibilities:

* Connect
* Authenticate
* Discover resources
* Normalize resources
* Return generic resources

---

# Provider Manager

Responsible for:

```
Register()

Find()

Load()

Initialize()

Shutdown()
```

Internally:

```
map[string]Provider
```

Example:

```
"kubernetes"

↓

Kubernetes Provider
```

No switch statements should exist inside the core application.

---

# Resource Normalization

This is the most important architectural improvement.

Instead of exposing Kubernetes Pods directly to the Rule Engine,

convert every provider-specific object into a generic resource.

Example:

```
Kubernetes Pod

↓

Generic Resource
```

Docker Container

↓

Generic Resource

Linux Process

↓

Generic Resource

Terraform Resource

↓

Generic Resource

AWS EC2

↓

Generic Resource

The Rule Engine never sees provider-specific objects.

---

# Generic Resource Model

Instead of writing rules against Kubernetes objects,

write rules against infrastructure resources.

Example:

```
Resource

├── Metadata

├── Labels

├── Annotations

├── Security Context

├── Runtime

├── Networking

├── Storage

├── Owner

├── Relationships
```

Every provider populates this model.

---

# Rule Engine

The Rule Engine should only evaluate generic resources.

Pipeline:

```
Generic Resource

↓

Evaluate Rules

↓

Findings
```

This allows one rule to work across multiple providers.

Example:

```
Running as Root

↓

Kubernetes Pod

Docker Container

Linux Process
```

without changing the rule.

---

# Discovery Pipeline

```
Provider

↓

Discover Native Objects

↓

Normalize

↓

Generic Resources

↓

Rule Engine

↓

Findings
```

Each provider owns only discovery and normalization.

---

# Reporting Layer

Responsible only for presentation.

Outputs:

* Table
* JSON
* SARIF
* Markdown
* HTML
* CI/CD Output

The reporting layer should never execute rules.

---

# Recommended Project Structure

```
internal/

    app/

    scan/

        orchestrator/

        pipeline/

        engine/

    provider/

        provider.go

        manager.go

        registry.go

    resource/

        resource.go

        metadata.go

        security.go

        networking.go

    rule/

        engine/

        evaluator/

        parser/

    report/

        table/

        json/

        sarif/

        markdown/

providers/

    kubernetes/

    docker/

    host/

    terraform/

    aws/

    azure/

    gcp/

cmd/

pkg/
```

---

# Scan Pipeline

```
CLI

↓

Dispatcher

↓

Scan Orchestrator

↓

Provider Manager

↓

Provider

↓

Discovery

↓

Normalization

↓

Generic Resources

↓

Rule Engine

↓

Findings

↓

Report Engine

↓

Output
```

---

# Provider Lifecycle

```
Load

↓

Initialize

↓

Connect

↓

Discover

↓

Normalize

↓

Return Resources

↓

Shutdown
```

Every provider follows the same lifecycle.

---

# Provider Metadata

Every provider should expose metadata.

Example:

```
Name

Version

Supported Resources

Capabilities

Author
```

Example command:

```
infrasight providers list
```

Output:

```
NAME          VERSION

kubernetes    v1.0

docker        v1.0

host          v1.0
```

---

# Future Configuration

Instead of hardcoding providers,

use configuration.

```yaml
providers:

  kubernetes:

    enabled: true

  docker:

    enabled: false

  host:

    enabled: true
```

Startup sequence:

```
Read Config

↓

Instantiate Providers

↓

Register

↓

Ready
```

---

# Future Plugin Evolution

## Phase 1

In-process providers.

```
Core

↓

Go Interface

↓

Provider
```

---

## Phase 2

Dynamic registration.

```
Register()

↓

Provider Manager
```

---

## Phase 3

Configuration-driven loading.

```
Config

↓

Provider Manager
```

---

## Phase 4

External provider processes.

```
InfraSight

↓

gRPC

↓

Provider Process
```

Examples:

```
Kubernetes Provider

Docker Provider

AWS Provider
```

This enables:

* Independent releases
* Crash isolation
* Third-party providers
* Stable plugin protocol

---

# Long-Term Vision

```
                           InfraSight

                                │

                          CLI Commands

                                │

                                ▼

                        Scan Orchestrator

                                │

                                ▼

                        Provider Manager

     ┌───────────────┬───────────────┬───────────────┐

     ▼               ▼               ▼

 Kubernetes      Docker         Linux Host

     ▼               ▼               ▼

 Discovery      Discovery      Discovery

     └───────────────┼───────────────┘

                     ▼

           Resource Normalization

                     ▼

             Generic Resources

                     ▼

               Rule Evaluation

                     ▼

                 Findings

                     ▼

              Reporting Engine

     ┌──────────────┼──────────────┬──────────────┐

     ▼              ▼              ▼

   Table          JSON           SARIF
```

---

# Recommended Development Roadmap

| Phase | Goal                                          | Priority |
| ----- | --------------------------------------------- | -------- |
| 1     | Introduce `Provider` interface                | High     |
| 2     | Build `ProviderManager`                       | High     |
| 3     | Move Kubernetes into a provider               | High     |
| 4     | Create Generic Resource Model                 | High     |
| 5     | Refactor Rule Engine to use Generic Resources | High     |
| 6     | Introduce Scan Orchestrator                   | Medium   |
| 7     | Refactor Reporting Layer                      | Medium   |
| 8     | Add Docker Provider                           | Medium   |
| 9     | Add Linux Host Provider                       | Medium   |
| 10    | Introduce Configuration-driven Providers      | Low      |
| 11    | Add Provider Metadata                         | Low      |
| 12    | Evolve to gRPC-based External Providers       | Future   |

---

# Final Recommendation

The most important architectural improvement is **introducing a Generic Resource Model between infrastructure providers and the Rule Engine**.

By making providers responsible only for **discovery** and **normalization**, the Rule Engine becomes completely independent of Kubernetes, Docker, Linux, or cloud-specific APIs. This allows new providers to be added with minimal changes to the core application, while existing rules, reporting formats, and scan workflows continue to work unchanged.

This architecture transforms InfraSight from a Kubernetes-focused scanner into a scalable infrastructure analysis platform capable of supporting multiple providers, richer integrations, and eventually an external provider ecosystem without requiring fundamental redesigns.
