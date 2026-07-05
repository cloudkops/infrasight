# Host Provider — As-Built Reference

> Scope: `providers/host/` and the generic stages it depends on
> (`internal/scan/pipeline`, `internal/rule/evaluator`). This describes what is
> actually implemented and tested today, not a build plan — see `docs/docker.md`
> and `docs/k8s.md` for the sibling providers this one is modeled after, and
> `docs/dev_manual.md` for how to extend any of this.

---

## 1. Package layout

One package, four files, same split as `providers/kubernetes`/`providers/docker`:

| File | Owns |
|---|---|
| `providers/host/provider.go` | Satisfies `internal/provider.Provider`. Registers via `init()`. Gates `Discover` on `runtime.GOOS != "linux"` — this provider reads `/proc` and shells out to `systemctl`, so it only functions on Linux. |
| `providers/host/client.go` | Local environment setup: `defaultProcRoot` (`"/proc"`), the `unitInfo` type, and the `systemdLister` interface + its real `systemctl`-backed implementation. |
| `providers/host/discovery.go` | Reads `/proc` and calls `systemdLister`. Returns raw local types (`unitWithPorts`, `procInfo`), no `resource.Resource` knowledge. |
| `providers/host/normalize.go` | Maps raw discovery data → `[]resource.Resource`. Pure functions — this is what `normalize_test.go` exercises against hand-built fixtures. |

**This provider scans only the local machine InfraSight is running on** — there
is no remote-agent/SSH fan-out, matching the original stub comment in
`provider.go`.

---

## 2. End-to-end flow

```
infrasight scan host --ruleset ./rulesets/host-baseline --fail-on HIGH
        │
        ▼
internal/app/scan_cmd.go        → parses flags into scanFlags
        │
        ▼
internal/app/dispatcher.go      → dispatch(): builds orchestrator.Request, calls Run
        │
        ▼
internal/scan/orchestrator.Run  → manager.Get("host")  [the only lookup — never
        │                          a direct import of providers/host]
        ▼
providers/host.Provider.Discover(ctx, opts)
        │
        ├─ 0. runtime.GOOS != "linux" → immediate error, no partial scan
        │
        ├─ 1. discovery.go :: discoverAll(ctx, "/proc", newSystemdLister())
        │      a. systemdLister.ListUnits(): one `systemctl list-units
        │         --type=service --all --output=json` call, then one BATCHED
        │         `systemctl show <all units> -p ...` call for every unit's
        │         hardening-relevant properties — not one call per unit (see §4)
        │      b. walk /proc/<pid> for every PID; parse /proc/net/{tcp,tcp6}
        │         for listening sockets; correlate each listening socket's inode
        │         to its owning PID via /proc/<pid>/fd/*; resolve that PID's
        │         systemd unit (if any) via /proc/<pid>/cgroup
        │      Partial failures (systemctl unavailable, one PID's fd
        │      unreadable) are collected into rawObjects.warnings; only
        │      procRoot itself being unreadable aborts discovery — see §7.
        │
        └─ 2. normalize.go :: NormalizeAll() → []resource.Resource
                (field-by-field mapping in §5)
        │
        ▼
internal/scan/pipeline.Run       → resource.Validate() each result, then:
        │                            1. EnrichAttributes()   (§6 — generic, every provider)
        │                            2. ResolveRelationships() (generic; a no-op here —
        │                               every host Resource has Owner=nil)
        ▼
internal/scan/orchestrator.Run  → parser.LoadRuleset(dir, "host") or parser.Load(file)
        │                          → internal/scan/engine.Scan() → internal/rule/engine.Evaluate()
        ▼
internal/report                  → renderer resolved by --format (table/json/sarif/markdown),
                                    report.WriteSummary, report.ExitCode
```

---

## 3. `provider.go` — interface contract

```go
package host

const Name = "host"

type Provider struct{}

func New() *Provider { return &Provider{} }

func init() {
	provider.Register(Name, New())
}

func (p *Provider) Name() string        { return Name }
func (p *Provider) Kind() provider.Kind { return provider.KindLive }

func (p *Provider) Discover(ctx context.Context, opts provider.Options) ([]resource.Resource, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("host: unsupported OS %q (this provider reads /proc and requires systemd)", runtime.GOOS)
	}
	raw, err := discoverAll(ctx, defaultProcRoot, newSystemdLister())
	if err != nil {
		return nil, err
	}
	return NormalizeAll(raw), nil
}
```

No new `provider.Options` field and no new CLI flag — this provider takes no
per-invocation configuration beyond what already exists (`--verbose`). The
`procRoot` seam exists only for tests (see §4); no real invocation needs a
different `/proc` path.

---

## 4. Normalize flow — the core design decision

**Both Resource `Kind`s this provider produces are deliberately containerless**
(`Runtime.Containers` stays `nil`) — a systemd unit or a bare process has no
nested-container concept the way a Kubernetes Pod or a Docker container does.
This matters because of a real bug fixed in the Docker provider: `internal/rule/
engine.Evaluate` skips a rule entirely for any resource with zero
`Runtime.Containers` entries if that rule references a `container.*`-prefixed
field (see `docs/docker.md` §4). Docker needed a fake "one nested Container" to
make its pre-existing `container.*`-prefixed ruleset resolve at all. **Host has
no such legacy** — `Security` is set directly and fully on the top-level
`Resource` (there is nothing to roll up from, unlike Docker/Kubernetes'
per-container detail), and `host-baseline.yaml` uses only
`resource.*`/`resource.security.*` plus two new attribute namespaces,
`systemd.*` and `process.*`. **`host-baseline` rules must never use
`container.*` fields** — `providers/host/normalize_test.go`'s
`TestHostResources_AreContainerless` guards this invariant directly.

### `Kind: "systemd_unit"` — one per systemd `.service` unit (`mapUnit`)

| Resource field | Source |
|---|---|
| `ID`, `Name` | unit name, e.g. `"nginx.service"` — stable across scans, unlike a PID |
| `Security.RunAsRoot` | `User=` property empty or `"root"` |
| `Security.Privileged` | always `false` — no direct systemd equivalent; not invented |
| `Security.AllowPrivilegeEscalation` | `*bool`, `!NoNewPrivileges` (always set) |
| `Security.ReadOnlyRootFilesystem` | `*bool`, `true` only if `ProtectSystem == "strict"` — a documented approximation: `full`/`yes` are partial protections, not full read-only. The raw value is *also* kept as `systemd.protect_system` for rules wanting that finer distinction — same "typed approximation + raw attribute alongside it" pattern as Kubernetes' `service_account_token_automount` caveat |
| `Security.CapabilitiesAdd` | `AmbientCapabilities`, normalized from systemd's `cap_sys_admin` form to the `SYS_ADMIN` form Docker/Kubernetes already use (see §4a) |
| `Security.CapabilitiesDrop` | left `nil` — `CapabilityBoundingSet` is a restriction *list* (what's still allowed), not an additive "drop" list, so forcing it into this field would misrepresent it; exposed instead as `systemd.capability_bounding_set` |
| `Networking.Exposures` | listening TCP ports whose owning PID's cgroup resolves to this unit (§4b) |
| `Attributes` | `systemd.active_state`, `systemd.sub_state`, `systemd.description`, `systemd.fragment_path`, `systemd.private_tmp` (bool), `systemd.protect_home` (string), `systemd.protect_system` (string, raw), `systemd.restrict_namespaces` (string), `systemd.memory_deny_write_execute` (bool), `systemd.capability_bounding_set` ([]string), `systemd.main_pid` (int) |

**4a. Capability name normalization.** `systemctl show`'s `AmbientCapabilities`/
`CapabilityBoundingSet` return space-separated, lower-case `cap_xxx` names (e.g.
`cap_sys_admin`). `normalizeCapabilities` (`client.go`) strips the `cap_` prefix
and upper-cases the rest (`SYS_ADMIN`) — so a rule author writes one capability
naming convention project-wide, whether the resource came from Kubernetes,
Docker, or Host.

**4b. Batched `systemctl show`, not one call per unit.** Unlike Docker's
`ContainerInspect`/`ImageInspect` (no bulk API — bounded to 8 concurrent calls,
see `docs/docker.md` §6), `systemctl show` accepts every unit name in a single
invocation, one properties block per unit separated by a blank line. This
provider fetches **all units' properties in one subprocess call**, avoiding the
N+1-subprocess problem entirely — no worker pool needed. Each returned block is
matched back to its unit by its own `Id=` property, never by block position —
systemd does not promise block order follows argument order (verified against a
real `systemctl show a.service b.service -p Id` — order didn't match input
order).

### `Kind: "process"` — only for a listening process not attributable to any unit (`mapProcess`)

By construction, a `"process"` Resource only ever exists because it has a
listening TCP socket and discovery could not resolve any systemd unit owning
it — so a rule can flag "unmanaged exposed process" with nothing more than
`field: resource.kind, equals: "process"` (the `resource.kind` typed field
already exists in `evaluator.resolveField`).

| Resource field | Source |
|---|---|
| `ID` | `"pid-<pid>"` — explicitly a per-scan snapshot ID, not stable across scans/reboots (documented, same honesty as Docker's short-ID caveat) |
| `Name` | `/proc/<pid>/status`'s `Name:` (comm), falling back to the basename of the first `cmdline` argument, falling back to `"pid-<pid>"` |
| `Security.RunAsRoot` | effective UID `== 0`, from `/proc/<pid>/status`'s `Uid:` line (2nd field) |
| `Networking.Exposures` | its listening TCP ports |
| `Attributes` | `process.pid`, `process.exe_path` (`readlink /proc/<pid>/exe`, may be empty), `process.cmdline`, `process.uid` |

**Note on rule field consistency:** `resource.security.run_as_root` works
identically for both Kinds (same field, same prefix), so `ISG-HOST-001` ("running
as root") covers systemd units and unmanaged processes with one rule — no
per-Kind duplication in the ruleset.

### Socket → PID → unit correlation algorithm (`discovery.go`)

1. Parse `/proc/net/tcp` and `/proc/net/tcp6` for rows in the `TCP_LISTEN`
   (`0A`) state, capturing `{inode, port}`. **UDP is out of scope** — it has no
   equivalent unambiguous "listening" state, and forcing one in would risk
   misclassifying an ordinary bound-but-not-connected UDP socket as an exposure.
2. For every PID, scan `/proc/<pid>/fd/*` symlinks for targets shaped
   `socket:[<inode>]`, to map each listening inode to its owning PID.
3. For each owning PID, read `/proc/<pid>/cgroup` and extract the innermost
   path segment ending in `.service` — the same technique `systemd-cgls`/
   `ps -o unit` use. This is **robust to forking/multi-process units**, unlike
   matching solely against a unit's `MainPID` (a worker process's PID never
   equals it).
4. If that unit name matches a discovered unit → the port is attached to that
   `systemd_unit` Resource. Otherwise → the owning PID becomes its own
   standalone `"process"` Resource.

---

## 5. `EnrichAttributes` — how Security/Networking become rule-queryable

Identical generic code to the Kubernetes/Docker path
(`internal/scan/pipeline/enrich.go` — see `docs/k8s.md` §6):

```go
r.Attributes = mergeAttributes(r.Attributes, r.Security.Flatten("resource.security"))
r.Attributes = mergeAttributes(r.Attributes, r.Networking.Flatten("networking"))
```

Because both host Resource kinds populate `Resource.Security` directly (§4) and
`Runtime.Containers` stays empty, this stage produces
`resource.security.run_as_root`, `resource.security.allow_privilege_escalation`,
`resource.security.read_only_root_filesystem`, and
`resource.security.capabilities_add` for Host exactly the way it already does
for Kubernetes/Docker's resource-level fields — no Host-specific case anywhere
in `internal/rule/evaluator`.

---

## 6. Error handling summary

| Failure | Behavior |
|---|---|
| Non-Linux OS | `Discover()` returns an error immediately, no partial scan |
| `/proc` itself unreadable | `discoverAll()` returns an error immediately — without it, nothing can be discovered |
| `systemctl` not on `PATH`, or `list-units`/`show` fails | Recorded as a warning; unit discovery is skipped entirely, but listening-process discovery still runs and standalone `"process"` resources are still reported |
| A single PID's `/proc/<pid>/status` is unreadable (process exited mid-scan) | That PID is skipped, recorded as a warning |
| A single PID's `/proc/<pid>/fd` is unreadable (owned by a different, non-root user) | Socket-ownership correlation for that PID is skipped, recorded as a warning |

**Run as root for complete results.** `/proc/<pid>/status` (uid, comm) is
world-readable for any process regardless of owner, so root-detection always
works. `/proc/<pid>/fd/*` for a process owned by a *different* UID requires
root — running InfraSight as a non-root user means listening-socket
attribution for other users' processes will be incomplete (some sockets may be
silently invisible to correlation, recorded as warnings, not silently wrong).

---

## 7. Bundled ruleset — `rulesets/host-baseline`

11 rules, provider-scoped to `"host"`. See `docs/rulesets_manual.md` for the
full operator reference; Host-specific field usage:

| Rule | Field(s) | Severity |
|---|---|---|
| Running as root (unit or unmanaged process) | `resource.security.run_as_root` | HIGH |
| Privilege escalation not restricted | `resource.security.allow_privilege_escalation` | MEDIUM |
| Root filesystem not strictly protected | `resource.security.read_only_root_filesystem` | MEDIUM |
| Dangerous ambient capability granted | `resource.security.capabilities_add` (any_of contains) | CRITICAL |
| Dangerous capability still in bounding set | `systemd.capability_bounding_set` contains | HIGH |
| `PrivateTmp` not enabled | `systemd.private_tmp` | LOW |
| Home directories not protected | `systemd.protect_home` | LOW |
| Namespaces not restricted | `systemd.restrict_namespaces` | MEDIUM |
| `MemoryDenyWriteExecute` not set | `systemd.memory_deny_write_execute` | MEDIUM |
| Unit in a failed state | `systemd.active_state` | LOW |
| Unmanaged process listening on a network port | `resource.kind` equals `"process"` | MEDIUM |

---

## 8. Tests (all passing, including `-race`)

| File | Covers |
|---|---|
| `providers/host/normalize_test.go` | Root/hardening-flag mapping (`TestMapUnit_RootAndHardeningFlags`, `TestMapUnit_NonRootNoNewPrivilegesNotStrict`), the containerless invariant (`TestHostResources_AreContainerless`), process name fallback chain, and an evaluation-level regression test (`TestHostBaseline_KeyRulesFireOnUnhardenedUnit`) that loads the real `host-baseline` ruleset and asserts every rule fires against a realistically unhardened unit/process — the same class of test that caught the Docker provider's container-scoped field bug |
| `providers/host/discovery_test.go` | Socket→unit attribution, unmanaged-listening-process fallback, a non-listening process producing no resource, graceful degradation when systemd is unavailable, and `procRoot` itself being unreadable being the one fatal case — all against a fake `/proc` tree built under `t.TempDir()` and a fake `systemdLister`. This closes the "no discovery-level test" gap `docs/docker.md` §10 still documents for Docker (which has no fake-client seam) |
| `internal/rule/parser/loader_test.go` | `host-baseline` (11 rules) parses and validates via `TestLoadRuleset_HostBaseline` |

No live-machine integration test exists beyond the fake-`/proc` fixtures above —
this was manually verified by running `infrasight scan host` against the
developer's real machine during implementation (see the PR/commit this shipped
in for that verification).
