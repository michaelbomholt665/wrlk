# Port Router — Design Document

**Version:** 0.9.0  
**Status:** v0.9 active — all planned features promoted to in scope  
**Scope:** Plug-and-play dependency broker for Hexagonal Architecture in Go

## Overview

The Port Router is a **zero-dependency, copy-paste dependency broker** that lives in `internal/router/`. Its single job is to centralize port declarations and adapter wiring so adapters do not depend on each other directly.

**Key properties:**
- No external dependencies (stdlib only)
- No runtime reflection or dynamic loading
- Lives entirely in `internal/` 
- Designed to constrain AI agent behavior during development
- Host project controls all policy, validation, timeout, and complexity rules

Copy this folder into any Go project. Touch only the mutable wiring files.

## Core Purpose

**Primary problem solved:** AI-driven coupling creep in Hexagonal Architecture.

Without guardrails, AI agents tend to create cross-adapter calls, hidden dependencies, and local shortcuts that bypass the intended port boundaries. The router solves the **structural** side by making dependency wiring a small, auditable declaration surface. Cross-adapter coupling now requires explicit changes to `ports.go` plus the layer wiring in `extensions.go` and `optional_extensions.go` when optional capabilities are involved.

**Secondary problem solved:** AI modification of shared infrastructure. Frozen/mutable split + `router.lock` make correct changes cheaper than wrong changes.

**Out of scope:** behavioral concerns such as complexity, style, policy, and business-specific validation belong in host project tooling.

***

## What It Is vs What It Is Not

| **Is**                                      | **Is Not**                         |
| ------------------------------------------- | ---------------------------------- |
| Centralized port whitelist (`ports.go`)     | Dependency injection framework     |
| Explicit extension wiring (`extensions.go`, `optional_extensions.go`) | Plugin system with dynamic loading |
| Compile-time port name safety               | Policy enforcement tool            |
| Boot orchestration + lifecycle guardrails   | Complexity/style linter            |
| AI development constraint system            | Runtime sandbox                    |

***

## Folder Structure

```text
internal/router/
│
├── MUTABLE — host project wiring (4 files)
│   ├── ports.go              # PortName constants (whitelist)
│   ├── registry_imports.go   # Imports + routerValidatePortName + atomic registry declaration
│   ├── extensions.go         # application extensions + thin RouterBootExtensions wrapper
│   └── optional_extensions.go # optional extensions wired ahead of application extensions
│
├── FROZEN — never edit directly
│   ├── registry.go           # Atomic publication + RouterResolveProvider
│   └── extension.go          # Extension interfaces + RouterLoadExtensions
│
├── router.lock               # NDJSON integrity checksums (git committed)
└── tools/
    └── wrlk/
        └── main.go           # Optional router-local CLI for live checks and lock workflows
```

## File Responsibilities

### `ports.go` — MUTABLE (Port Whitelist)

```go
package router

// PortName is a typed string that prevents raw string values from being
// passed to RouterRegisterProvider or RouterResolveProvider. The compiler enforces this.
type PortName string

// Port constants define every valid port in this project.
// To add a new port: add one line here, then register an implementation
// in the correct wiring layer. No frozen router files need to change.
const (
    PortConfig PortName = "config"
    PortAuth   PortName = "auth"
    PortDB     PortName = "db"
    // Add new ports here only. One line.
)
```
**Rules:**

- One constant per port, no exceptions
- Names are lowercase strings, PascalCase constants
- Removing a constant is a breaking change — deprecate first, remove in a later version
- Constants live here and nowhere else — adapters import this package to reference them


### `registry_imports.go` — MUTABLE

```go
package router

import "sync/atomic"

var registry atomic.Pointer[map[PortName]Provider]

func routerValidatePortName(port PortName) bool {
    switch port {
    case PortConfig, PortAuth, PortDB:
        return true
    }
    return false
}
```

### `optional_extensions.go` — MUTABLE (Optional Layer Wiring)

```go
package router

import (
    "your-project/internal/adapters/telemetry"
    // Add optional extension imports here
)

var optionalExtensions = []Extension{
    telemetry.Extension(),
    // Add one line per optional extension
}
```

This file owns the optional extension layer only. Optional extensions boot before
application extensions and may provide ports consumed during application boot.

### `extensions.go` — MUTABLE (Application Wiring + Thin Wrapper)

```go
package router

import (
    "context"
    "your-project/internal/adapters/config"
    "your-project/internal/adapters/auth"
    // Add new extension imports here
)

var extensions = []Extension{
    config.Extension(),
    auth.Extension(),
    // Add one line per new application extension
}

// RouterBootExtensions wires optional extensions first, then application
// extensions, and publishes the atomic registry on full success only.
func RouterBootExtensions(ctx context.Context) ([]error, error) {
    return RouterLoadExtensions(optionalExtensions, extensions, ctx)
}
```

### `extension.go` — FROZEN

Contains:
- `Extension` / `AsyncExtension` / `ErrorFormattingExtension` interfaces
- `RouterError`, `RouterErrorFormatter`, `Registry` handle
- `RouterLoadExtensions(optionalExts []Extension, exts []Extension, ctx context.Context) ([]error, error)`

`Registry` holds a private pointer to the local boot map. It is the **only** write surface extensions touch.

### `registry.go` — FROZEN

Contains atomic publication logic + `RouterResolveProvider`.  
`RouterRegisterProvider` exists only as internal implementation used by the `Registry` handle (not public API).

## Bootstrap Contract (main.go)

**Call this exactly once from the host startup layer before request handlers, workers, or other goroutines begin.**

```go
package main

import (
    "context"
    "log"
    "time"

    "your-project/internal/router"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    warnings, err := router.RouterBootExtensions(ctx)
    if err != nil {
        log.Fatal(err)
    }
    for _, w := range warnings {
        log.Println(w)
    }

    // Router is now booted. Start handlers/workers normally.
}
```

### Bootstrap Semantics

- `ctx` is required so async extension boot respects host timeout/cancellation policy
- hardcoding `context.Background()` inside the router is forbidden
- a deadlocked or stalled async extension must be able to fail startup through host timeout policy
- concurrent boot attempts are a **host programming error** and are unsupported
- repeated boot after successful initialization returns `MultipleInitializations`

***


## Atomic Publication Model (Model A)

**Model A is the correct design.**

The atomic registry pointer is the only published runtime state.

### State Model

- Boot builds registrations into a **local** temporary map.
- `Registry` handle writes only to this local map.
- On full success the map is published **exactly once** via `registry.Store(...)`.
- `RouterResolveProvider` checks only the atomic pointer: `nil` = not booted (`RegistryNotBooted`), non-nil = immutable snapshot (lock-free reads).
- Failed boot discards the local map. Nothing is published.
- `MultipleInitializations` is returned if boot is attempted again after successful publication.
- Boot publishes via `registry.CompareAndSwap(nil, &localMap)`. If two goroutines 
  race to boot, exactly one CAS succeeds. The loser receives `false` and returns 
  `MultipleInitializations`. No separate mutex or `sync.Once` required — the atomic 
  pointer is both the state and the concurrency primitive.

### Consequences

1. There is no split runtime state between a pointer and a separate package-level `sealed` flag
2. The visible state transition is one atomic publish event
3. After publication, readers observe a complete immutable snapshot
4. Before publication, the router is simply not booted

This preserves the zero-contention post-boot read path without introducing a second published state variable.

***

## Error Catalog

**Structured `RouterError` covers all router failures:**

**Registration Errors**
- `PortUnknown` - **Portname MUST be included**
- `PortDuplicate` - **Portname MUST be included**
- `InvalidProvider`

**Resolution Errors**
- `PortNotFound` - **Portname MUST be included**
- `RegistryNotBooted`

**Boot Errors**
- `RequiredExtensionFailed`
- `OptionalExtensionFailed`
- `DependencyOrderViolation` - **Portname MUST be included**
- `AsyncInitTimeout`

**Environment / Compatibility Errors**
- `MultipleInitializations`
- `PortContractMismatch` — **Defined by the router, raised by adapters** when a resolved `Provider` does not satisfy the expected port interface. Keeps type-assertion failures structured and consistent across all adapters rather than bare `fmt.Errorf` strings.

**Note on `PortNotFound` / `DependencyOrderViolation` during boot:** error message must contain: "If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong. Move the providing extension higher up in the correct extensions slice."

## Extension Error Override Contract

```go
type ErrorFormattingExtension interface {
    Extension
    ErrorFormatter() RouterErrorFormatter
}
```

Detected via type assertion during boot (same pattern as `AsyncExtension`). If present the router uses the formatter for that extension’s errors. Cannot downgrade fatal errors to warnings.

## Adding New Capabilities

**New port + adapter:**
1. Add constant to `ports.go`
2. Add case to `routerValidatePortName` in `registry_imports.go`
3. Define port interface in `internal/ports/`
4. Implement adapter + `Extension()` in adapter package
5. Add import + line to `extensions.go` or `optional_extensions.go`, whichever layer owns it

**Swap implementation:** One line in the owning wiring file.

## AI Guardrails (Development Constraints)

**Not runtime security. Development workflow constraints only.**

| **Mechanism**             | **Purpose**                    | **AI Effect**                                                   |
| ------------------------- | ------------------------------ | --------------------------------------------------------------- |
| Frozen/mutable split      | Protect core contracts         | Wrong path: edit frozen (hard). Right path: edit mutable (easy) |
| `router.lock` checksums   | Detect frozen drift            | Fatal halt before other changes                                 |
| Data-only wiring files    | Limit mutation surface         | Hard to justify logic changes in layer declaration files        |
| Explicit error catalog    | Guide agent diagnosis          | Clear "where to fix" without router internals                   |
| Typed `PortName`          | Compiler catches string errors | No runtime surprises from typos                                 |

**Host tooling or the router-local `wrlk` CLI may additionally enforce:**
- no edits to frozen files
- `router.lock` integrity before other checks
- explicit `wrlk lock update`
- no suggestions that propose editing frozen files to fix host wiring issues

***

## Security Model

This router is a **development integrity and structural constraint system**, not a runtime sandbox.

| **Concern**                   | **Router Contribution**                           | **Precise Claim**                                                    |
| ----------------------------- | ------------------------------------------------- | -------------------------------------------------------------------- |
| External package reachability | `internal/` placement                             | Encapsulation/import boundary inside the host project, not a sandbox |
| Typo / wrong port literals    | Typed `PortName` constants + whitelist validation | Correctness guard, not a full injection defense                      |
| Late mutation after boot      | Immutable published snapshot                      | Prevents normal post-boot registration in supported usage            |
| Port shadowing                | Duplicate registration rejection                  | First successful provider wins during supported boot flow            |
| Frozen file drift             | `router.lock` + host tooling                      | Development integrity control                                        |
| Async boot deadlock           | Host-supplied `ctx` timeout/cancellation          | Operational startup safeguard                                        |

The router does **not** claim to protect against malicious code already inside the host project's allowed package tree. That is outside scope.

**Host tooling or the router-local `wrlk` CLI may additionally enforce:**
- no edits to frozen files
- `router.lock` integrity before other checks
- explicit `wrlk lock update`
- no suggestions that propose editing frozen files to fix host wiring issues

***

## v0.9 Features (All In Scope)

### Router-local `wrlk` ✅
Optional router-local CLI entrypoint.

**Status:** Implemented. `wrlk lock verify`, `wrlk lock update`, and `wrlk live run` are green.

Command surface:
- `wrlk live run` — explicit live verification mode; never part of normal router boot
- `wrlk lock verify` — read-only lock verification
- `wrlk lock update` — explicit lock regeneration or rewrite

**Phase 11b** adds full `live run` session lifecycle test coverage.

Constraint (permanent):
- `wrlk` is optional and has **no bearing on regular operations**
- it must not change `RouterBootExtensions`, `RouterLoadExtensions`, or `RouterResolveProvider`
- it must remain embeddable in a larger host CLI structure without breaking downstream behavior

### Router-local `portgen` — Phase 11
Single-action port registration generation.

**Status:** In scope for v0.9.

Intent:
- eliminate the manual two-file coherence step between `ports.go` and `registry_imports.go`
- make adding a port one intentional action instead of two independent edits
- keep the generator inside the copy-paste router bundle, not coupled to any host project

Workflow:
- host project opts in explicitly via `go generate`
- `portgen add --name PortFoo --value foo` adds the constant to `ports.go`, adds the case to `routerValidatePortName`, and rewrites `router.lock` atomically
- `--dry-run` flag prints what would change without writing

Lives under: `internal/router/tools/portgen/main.go`

Key functions: `RouterRunPortgenProcess`, `RouterParsePortgenFlags`, `RouterAddPort`, `RouterWritePortsFile`, `RouterWriteValidationFile`, `RouterWriteLockAfterPortgen`

### `Extension.Consumes()` Topological Sort — Phase 12
Topological boot ordering.

**Status:** In scope for v0.9. Interface method declared; sort implementation not yet written.

Manual slice order is load-bearing until this is implemented. After Phase 12, the router automatically resolves boot order from declared `Consumes()` dependencies.

Key functions: `RouterSortExtensionsByDependency`, `RouterBuildDependencyGraph`, `RouterDetectCyclicDependency`

A `RouterCyclicDependency` error is added to the error catalog for detected cycles.

### Restricted/Private Port Resolution — Phase 13
Consumer-identity-based access control for selected ports.

**Status:** In scope for v0.9.

Consumer identity becomes part of the resolution contract. Trust policy lives exclusively in mutable wiring files — never frozen router code.

A `PortAccessDenied` error code is added to the router error catalog.

Key functions: `RouterResolveRestrictedPort`, `RouterRegisterPortRestriction`, `RouterCheckPortConsumerAccess`

### Optional Extension Coverage — Phase 14
Actual test coverage and documented example for opt-in router-extending capabilities.

**Status:** In scope for v0.9. The `optionalExtensions` slice is currently empty (`[]router.Extension{}`). Structural wiring exists but is untested in the optional-layer-only path.

Goal: prove the optional extension layer with a real example and full test coverage for the opt-in capability lifecycle.

Key function: `RouterProvideOptionalCapability` (documented pattern, not a new API call).

***

## Extension Layering

The router must support two distinct extension layers in the initial design:

- a primary extension path for normal application adapters
- a separate optional extension path for router-extending capabilities

These layers must remain structurally separate in wiring even though both
ultimately register providers by port name.

### Structural Requirements

1. The router must support layered extension loading.
2. The layers must be configured separately.
3. The earlier layer may provide ports consumed by the later layer during boot.
4. Boot order between the layers is explicit and load-bearing: optional extensions
   boot first, application extensions boot second.
5. If a later extension depends on a port that was not made available by the
   earlier phase, boot fails under the existing dependency/order semantics.
6. Adding new optional capabilities later must not require changes to frozen
   router code.
7. The frozen router core defines only the contract for this layered boot model.
   Actual optional capabilities live outside the router and are introduced
   through mutable wiring only.

### Constraint

This is a structural capability, not a use-case-specific feature.

The router design must describe what this layered model needs to handle without
encoding why a host project might use it. The router core must not assign special
meaning to any particular optional capability. It only provides the boot and
wiring model that allows such capabilities to be added later without modifying
router internals.

Restricted or trusted-port enforcement is explicitly out of scope for this
initial layered design. If added later, it must extend the base resolution
contract deliberately rather than being smuggled into the first implementation.

***

## Design Contracts

- Router depends only on `Extension` interface, never concrete adapters.
- Host supplies `ctx` for timeout/cancellation.
- Mutable files = `ports.go` + `registry_imports.go` + `optional_extensions.go` + `extensions.go`.
- Frozen files contain contracts + orchestration + publication logic only.
- Zero external dependencies.
- `Registry` handle is the only write surface for extensions.
- Single atomic publication is the only runtime source of truth.
- `PortContractMismatch` belongs in router for consistent structured errors.
- `GoVersionMismatch` removed entirely.
- Layered boot is part of the base router contract; trusted-port enforcement is not.

**This router prevents coupling creep without becoming a framework. Copy. Wire. Ship.**




## Q&A

### #1 — Registry handle mechanics under Model A
**Q:** How does the `Registry` handle write to a local boot map?
**A:** `Registry` struct gains a private pointer to the local boot map created inside `RouterLoadExtensions`. The handle is the only write surface extensions touch. Extensions never see the atomic pointer or publication step.

### #2 — Package-level `RouterRegisterProvider` fate
**Q:** Does it still exist as a public function?
**A:** It either becomes the internal implementation the `Registry` handle delegates to, or is removed entirely with logic inlined into the handle method. It is not part of the public API. Extensions write exclusively through the `Registry` handle.

### #3 — Frozen function referencing mutable wiring state
**Q:** `RouterBootExtensions` in frozen code directly references mutable wiring state, poisoning the checksum. Intended?
**A:** No. Restore the separation. **Frozen:** `RouterLoadExtensions(optionalExts []Extension, exts []Extension, ctx context.Context)` takes both slices as parameters, checksums cleanly. **Mutable:** `RouterBootExtensions(ctx context.Context) ([]error, error)` is a thin wrapper in `extensions.go` passing `optionalExtensions`, `extensions`, and `ctx` through. Lock checksum stays stable across projects.

### #4 — `MultipleInitializations` behavior
**Q:** Return error or panic?
**A:** Return `MultipleInitializations` error and stop. No panic, no recovery attempt. Detection: `registry.Load() != nil` at boot start.

### #5 — `GoVersionMismatch` ownership
**Q:** Is this the router's job?
**A:** No. Removed from the error catalog entirely. Not the router's problem.

### #6 — `PortContractMismatch` ownership
**Q:** Type assertions happen in adapter code, not inside the router. Should this stay?
**A:** Stays. Defined by the router, raised by adapters when a resolved `Provider` does not satisfy the expected port interface. Keeps type-assertion failures structured and consistent across all adapters rather than bare `fmt.Errorf` strings in every adapter.

### #7 — Extension Error Override registration mechanism
**Q:** When and how does registration happen?
**A:** Optional interface detected via type assertion during boot, same pattern as `AsyncExtension`:
```go
type ErrorFormattingExtension interface {
    Extension
    ErrorFormatter() RouterErrorFormatter
}
```
No separate registration step, no post-boot calls. Part of what an extension optionally declares about itself.

### #8 — `extensions.go` trailing comment
**Q:** Permanent guidance or scaffolding?
**A:** Permanent guidance comment. Same purpose as AI guardrail annotations elsewhere in the codebase.

### #9 — `RouterBootExtensions` now requires `ctx`
**Q:** Why the change from the original?
**A:** Original hardcoded `context.Background()` inside `RouterLoadExtensions` for async extensions. Host must control timeout and cancellation policy. Hardcoding context inside the router is forbidden. `ctx` flows from `main.go` through the mutable wrapper into the frozen boot function.

### #10 — `sync.RWMutex` + `sealed` replaced by `atomic.Pointer`
**Q:** What happened to the mutex-based design?
**A:** Replaced by Model A. Single `atomic.Pointer[map[PortName]Provider]` is the only published runtime state. `nil` = not booted, non-`nil` = immutable snapshot. Eliminates split state between two published variables. Lock-free reads post-boot. Boot-time writes happen to a local temporary map, never to a published variable.

### #11 — `Consumes()` implementation status
**Q:** Is topological sorting implemented?
**A:** No. Interface method exists, topological sorting does not. Manual slice order is load-bearing when one extension depends on a port provided by another during boot. `DependencyOrderViolation` error message must include: *"If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong. Move the providing extension higher up in the correct extensions slice."*

## Resolved Questions — Addendum

### #12 — Consumes() declared but unused
**Q:** If `Consumes() []PortName` is declared on the `Extension` interface but 
topological sorting is not implemented, does every extension author have to write 
dead boilerplate forever?
**A:** No. In hexagonal architecture, if a port is not declared in `ports.go` and
wired in `extensions.go` or `optional_extensions.go`, it does not exist at boot.
The `Consumes()` declaration on an extension that is never registered is never
evaluated.
The method is forward-compatible scaffolding, not imposed boilerplate.

### #13 — PortUnknown and PortNotFound must include port name
**Q:** The `DependencyOrderViolation` message format is mandated. Are `PortUnknown` 
and `PortNotFound` held to the same standard?
**A:** Yes. Both messages must include the port name. Example format: 
`port "auth" not found`. This applies equally to `PortUnknown` — the port constant 
that failed validation must be named in the error message.

### #14 — router.lock integrity is opt-in
**Q:** If the host project skips the lock check, the checksum is decoration. Is 
this a design flaw?
**A:** No. The router.lock is an AI development guardrail, not a runtime 
enforcement mechanism. The core purpose explicitly states: frozen/mutable split + 
`router.lock` make correct changes cheaper than wrong changes. Enforcement belongs 
in host tooling or the explicit router-local `wrlk` flow, by design.

### #15 — Concurrent boot safety mechanism
**Q:** How does the atomic pointer model actually prevent two goroutines from both 
passing the nil check and racing to publish?
**A:** Boot publishes via `registry.CompareAndSwap(nil, &localMap)`. If two 
goroutines race to boot, exactly one CAS succeeds. The loser receives `false` and 
returns `MultipleInitializations`. No separate mutex or `sync.Once` required — the 
atomic pointer is both the state and the concurrency primitive.

### #16 — Planned generator for port registration coherence
**Q:** Should port registration remain a manual edit across `ports.go` and 
`registry_imports.go`?
**A:** No, not long-term. Planned design is a router-local generator under 
`internal/router/tools/portgen/`, shipped as part of the copy-paste router and 
invoked explicitly via `go generate`. It will add the new port constant, update 
`routerValidatePortName`, and rewrite `router.lock` atomically. The goal is to 
eliminate the two-file coherence problem structurally rather than relying on 
documentation or review discipline.
