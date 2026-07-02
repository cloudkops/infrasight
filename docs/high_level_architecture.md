# InfraSight Provider Architecture Learning Roadmap

> **Goal:** Build InfraSight as a modular Infrastructure Analysis Platform while learning software architecture progressively.

This roadmap is intentionally incremental. Every phase introduces one major architectural concept without overwhelming the codebase.

---

# Phase 0 — Current State

```text
CLI

↓

Kubernetes Scanner

↓

Rule Engine

↓

Report
```

Characteristics

* Kubernetes-specific
* Simple
* Fast to develop
* Difficult to extend

Difficulty

⭐

---

# Phase 1 — Provider Abstraction

## Concept

Introduce the **Provider** abstraction.

Instead of:

```
Scan Kubernetes
```

Think:

```
Scan Infrastructure
```

Architecture

```text
CLI

↓

Scan Engine

↓

Provider

↓

Resources
```

Providers may include:

```
Kubernetes

Docker

Linux Host

Terraform
```

Learning Objectives

* Interfaces
* Dependency inversion
* Composition over coupling

Difficulty

⭐⭐

---

# Phase 2 — Provider Manager

## Concept

Introduce a central manager responsible for all providers.

Instead of:

```
CLI

↓

Kubernetes
```

Use

```text
CLI

↓

Provider Manager

↓

Provider
```

Responsibilities

* Register providers
* Lookup providers
* Initialize providers
* Shutdown providers

Architecture

```text
Provider Manager

        │

 ┌──────┼─────────┐

 ▼      ▼         ▼

K8s   Docker    Host
```

Learning Objectives

* Registry Pattern
* Factory Pattern
* Lifecycle Management

Difficulty

⭐⭐

---

# Phase 3 — Generic Resource Model

## Concept

The Rule Engine should never know Kubernetes exists.

Every provider converts native objects into a common model.

```text
Kubernetes Pod

↓

Resource
```

Docker Container

↓

Resource

Linux Process

↓

Resource

Architecture

```text
Provider

↓

Normalizer

↓

Generic Resource

↓

Rule Engine
```

Learning Objectives

* Domain Modeling
* Data Normalization
* Decoupling

Difficulty

⭐⭐⭐

---

# Phase 4 — Scan Orchestrator

## Concept

Separate workflow from implementation.

Current

```text
CLI

↓

Scanner
```

Future

```text
CLI

↓

Orchestrator

↓

Provider Manager

↓

Rule Engine

↓

Reporter
```

Responsibilities

* Load provider
* Execute discovery
* Run rules
* Generate report
* Cleanup

Learning Objectives

* Orchestration Pattern
* Application Services
* Workflow Management

Difficulty

⭐⭐⭐

---

# Phase 5 — Rule Engine Refactoring

## Concept

Rules should evaluate Resources, not Kubernetes objects.

Architecture

```text
Resource

↓

Rule

↓

Finding
```

Rules become reusable.

Example

```
Running As Root

↓

Kubernetes

Docker

Containerd

Podman
```

Learning Objectives

* Strategy Pattern
* Rule Evaluation
* Domain Separation

Difficulty

⭐⭐⭐

---

# Phase 6 — Dependency Injection

## Concept

Providers should receive dependencies instead of creating them.

Current

```text
Provider

↓

Creates Logger

Creates Config

Creates Clients
```

Future

```text
Application

↓

Container

↓

Provider
```

Dependencies

* Logger
* Config
* Kubernetes Client
* Docker Client
* HTTP Client

Learning Objectives

* Dependency Injection
* Inversion of Control
* Testability

Difficulty

⭐⭐⭐

---

# Phase 7 — Event Bus

## Concept

Components communicate through events.

Instead of

```text
Provider

↓

Reporter
```

Use

```text
Provider

↓

Event Bus

↓

Subscribers
```

Example Events

```
ScanStarted

ProviderLoaded

RuleMatched

ReportGenerated

ScanCompleted
```

Learning Objectives

* Publish / Subscribe
* Event-Driven Design
* Loose Coupling

Difficulty

⭐⭐⭐⭐

---

# Phase 8 — Pipeline Architecture

## Concept

Turn scanning into a processing pipeline.

```text
Provider

↓

Discovery

↓

Normalization

↓

Rule Evaluation

↓

Aggregation

↓

Reporting
```

Each stage has one responsibility.

Learning Objectives

* Pipeline Pattern
* Data Flow Architecture
* Single Responsibility

Difficulty

⭐⭐⭐⭐

---

# Phase 9 — Collector Architecture

## Concept

Discovery itself becomes modular.

Instead of

```
Kubernetes Provider
```

Internally

```text
Kubernetes Provider

        │

 ┌──────┼────────────┐

 ▼      ▼            ▼

Pods   Nodes    Deployments
```

Each collector discovers one resource type.

Learning Objectives

* Composition
* Modular Design
* Internal Plugin Pattern

Difficulty

⭐⭐⭐⭐

---

# Phase 10 — Provider Metadata

## Concept

Every provider describes itself.

Example

```text
Name

Version

Capabilities

Supported Resources

Description
```

Commands

```bash
infrasight providers list

infrasight providers info kubernetes
```

Learning Objectives

* Self-Describing Systems
* Metadata Design

Difficulty

⭐⭐

---

# Phase 11 — Configuration-Driven Providers

## Concept

Core no longer hardcodes providers.

Configuration

```yaml
providers:

  kubernetes:

    enabled: true

  docker:

    enabled: true

  host:

    enabled: false
```

Startup

```text
Read Config

↓

Instantiate Providers

↓

Register
```

Learning Objectives

* Configuration Management
* Bootstrapping
* Application Initialization

Difficulty

⭐⭐⭐

---

# Phase 12 — Multi-Provider Scan

## Concept

Scan multiple infrastructures in one run.

```text
Scan

↓

Kubernetes

Docker

Linux

↓

Merge Resources

↓

Rule Engine

↓

Unified Findings
```

Learning Objectives

* Aggregation
* Concurrency
* Coordination

Difficulty

⭐⭐⭐⭐

---

# Phase 13 — Parallel Execution

## Concept

Providers execute concurrently.

```text
Provider Manager

        │

 ┌──────┼────────────┐

 ▼      ▼            ▼

K8s   Docker      Host

        │

 Concurrent Discovery

        │

 Merge Results
```

Learning Objectives

* Goroutines
* WaitGroups
* Context Cancellation
* Worker Coordination

Difficulty

⭐⭐⭐⭐

---

# Phase 14 — Report Pipeline

## Concept

Separate findings from output formats.

```text
Findings

↓

Report Engine

↓

Table

JSON

SARIF

HTML

Markdown
```

Learning Objectives

* Adapter Pattern
* Output Abstraction

Difficulty

⭐⭐⭐

---

# Phase 15 — Future Evolution (Not Now)

If InfraSight ever needs third-party providers, only this layer changes.

Current

```text
Provider Interface

↓

Go Package
```

Future

```text
Provider Interface

↓

gRPC Client

↓

External Provider Process
```

Nothing else changes.

Learning Objectives

* RPC
* Process Isolation
* Distributed Systems
* Stable Plugin Contracts

Difficulty

⭐⭐⭐⭐⭐

---

# Complete Architecture

```text
                             CLI

                              │

                       Command Dispatcher

                              │

                       Scan Orchestrator

                              │

                       Provider Manager

      ┌───────────────┼────────────────┬────────────────┐

      ▼               ▼                ▼

 Kubernetes       Docker          Linux Host

      │               │                │

   Collectors     Collectors      Collectors

      │               │                │

      └───────────────┼────────────────┘

                      ▼

             Resource Normalizer

                      ▼

              Generic Resources

                      ▼

                Rule Engine

                      ▼

                  Findings

                      ▼

               Report Engine

      ┌───────────────┼───────────────┬───────────────┐

      ▼               ▼               ▼

    Table           JSON            SARIF
```

---

# Learning Progression

| Phase | Concept                              | Difficulty |
| ----- | ------------------------------------ | ---------- |
| 1     | Provider Interface                   | ⭐⭐         |
| 2     | Provider Manager                     | ⭐⭐         |
| 3     | Generic Resource Model               | ⭐⭐⭐        |
| 4     | Scan Orchestrator                    | ⭐⭐⭐        |
| 5     | Rule Engine                          | ⭐⭐⭐        |
| 6     | Dependency Injection                 | ⭐⭐⭐        |
| 7     | Event Bus                            | ⭐⭐⭐⭐       |
| 8     | Pipeline Architecture                | ⭐⭐⭐⭐       |
| 9     | Collector Architecture               | ⭐⭐⭐⭐       |
| 10    | Provider Metadata                    | ⭐⭐         |
| 11    | Configuration-Driven Providers       | ⭐⭐⭐        |
| 12    | Multi-Provider Scan                  | ⭐⭐⭐⭐       |
| 13    | Parallel Discovery                   | ⭐⭐⭐⭐       |
| 14    | Report Pipeline                      | ⭐⭐⭐        |
| 15    | External Provider Processes (Future) | ⭐⭐⭐⭐⭐      |

---

# Final Goal

By completing these phases, InfraSight evolves from a Kubernetes scanner into a modular infrastructure analysis platform. More importantly, each phase teaches a reusable architectural concept—interfaces, orchestration, pipelines, dependency injection, event-driven design, and concurrency—that applies well beyond this project and prepares you for building larger distributed systems in Go or Rust.
