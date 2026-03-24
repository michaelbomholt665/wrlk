# Router Architecture

This document provides a deep dive into the router's folder structure, the frozen/mutable split, and the bootstrap flow.

## Folder Structure

```
internal/router/
├── MUTABLE — host project wiring (4 files)
│   ├── ports.go              # PortName constants (whitelist)
│   ├── registry_imports.go   # Imports + RouterValidatePortName + atomic registry declaration
│   └── ext/
│       ├── extensions.go         # Required application extensions + thin RouterBootExtensions wrapper
│       └── optional_extensions.go # Optional capability extensions wired ahead of application extensions
│
├── FROZEN — never edit directly
│   ├── registry.go           # Atomic publication + RouterResolveProvider
│   └── extension.go          # Extension interfaces + RouterLoadExtensions
│
├── router.lock               # NDJSON integrity checksums (git committed)
└── tools/wrlk/
    └── main.go              # Optional router-local CLI for lock verification and port management
```

## What Is vs What Is Not

| **Is**                                                                | **Is Not**                         |
| --------------------------------------------------------------------- | ---------------------------------- |
| Centralized port whitelist (`ports.go`)                               | Dependency injection framework     |
| Explicit extension wiring (`ext/extensions.go`, `ext/optional_extensions.go`) | Plugin system with dynamic loading |
| Compile-time port name safety                                         | Policy enforcement tool            |
| Boot orchestration + lifecycle guardrails                             | Complexity/style linter            |
| AI development constraint system                                      | Runtime sandbox                    |

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
    PortPrimary   PortName = "primary"
    PortSecondary PortName = "secondary"
    PortTertiary  PortName = "tertiary"
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

type routerSnapshot struct {
    providers    map[PortName]Provider
    restrictions map[PortName][]string
}

var registry atomic.Pointer[routerSnapshot]

func RouterValidatePortName(port PortName) bool {
    switch port {
    case PortPrimary, PortSecondary, PortTertiary, PortOptional:
        return true
    default:
        return false
    }
}
```

### `ext/optional_extensions.go` — MUTABLE (Optional Layer Wiring)

```go
package ext

import (
    "your-project/internal/router"
    "your-project/internal/router/ext/extensions/telemetry"
    // Add optional extension imports here
)

var optionalExtensions = []router.Extension{
    &telemetry.Extension{},
    // Add one line per optional extension
}
```

This file owns the optional extension layer only. Optional extensions boot before
application extensions and may provide ports consumed during application boot.

### `ext/extensions.go` — MUTABLE (Required Application Wiring + Thin Wrapper)

```go
package ext

import (
    "context"

    "your-project/internal/router"
)

var extensions = []router.Extension{
    // App-owned required extensions only.
    // Generated with `wrlk ext app add` or maintained manually.
}

// RouterBootExtensions wires optional extensions first, then application
// extensions, and publishes the atomic registry on full success only.
func RouterBootExtensions(ctx context.Context) ([]error, error) {
    return router.RouterLoadExtensions(optionalExtensions, extensions, ctx)
}
```

### `extension.go` — FROZEN

Contains:
- [`Extension`](concepts.md#extension-interface) / [`AsyncExtension`](concepts.md#asyncextension-interface) / [`ErrorFormattingExtension`](concepts.md#errorformattingextension-interface) interfaces
- [`RouterError`](api-reference.md#routererror), [`RouterErrorFormatter`](concepts.md#error-formatting), [`Registry`](concepts.md#registry-handle) handle
- [`RouterLoadExtensions(optionalExts []Extension, exts []Extension, ctx context.Context) ([]error, error)`](api-reference.md#routerloadextensions)

`Registry` holds a private pointer to the local boot map. It is the **only** write surface extensions touch.

### `registry.go` — FROZEN

Contains atomic publication logic + [`RouterResolveProvider`](api-reference.md#routerresolveprovider).  
`RouterRegisterProvider` exists only as internal implementation used by the `Registry` handle (not public API).

## Bootstrap Flow

The router must be booted once before any goroutines attempt to resolve providers.

```mermaid
sequenceDiagram
    participant Main as main.go
    participant Ext as ext.RouterBootExtensions
    participant Load as router.RouterLoadExtensions
    participant Opt as Optional Extensions
    participant App as Application Extensions
    participant Reg as Atomic Registry

    Main->>Ext: RouterBootExtensions(ctx)
    Ext->>Load: RouterLoadExtensions(optional, application, ctx)
    
    rect rgb(240, 248, 255)
        Note over Load: Optional Layer (first)
        Load->>Opt: Load each optional extension
        Opt->>Reg: RouterRegisterProvider(port, provider)
        Opt-->>Load: warnings or error
    end
    
    rect rgb(255, 245, 238)
        Note over Load: Application Layer (second)
        Load->>App: Load each application extension
        App->>Reg: RouterRegisterProvider(port, provider)
        App-->>Load: warnings or error
    end
    
    alt All extensions succeeded
        Load->>Reg: registry.CompareAndSwap(nil, snapshot)
        Reg-->>Ext: warnings, nil
        Ext-->>Main: warnings, nil
    else Any extension failed
        Load-->>Ext: nil, error
        Ext-->>Main: fatal error (no partial state published)
    end
    
    Note over Reg: After boot: lock-free reads via atomic.Pointer
```

### Bootstrap Semantics

- `ctx` is required so async extension boot respects host timeout/cancellation policy
- Hardcoding `context.Background()` inside the router is forbidden
- A deadlocked or stalled async extension must be able to fail startup through host timeout policy
- Concurrent boot attempts are a host programming error
- Repeated boot after successful initialization returns `MultipleInitializations`
- Failed boot attempts roll back boot-only work for extensions that opt into `RollbackExtension`

## Atomic Publication Model (Model A)

**Model A is the correct design.** The atomic registry pointer is the only published runtime state.

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

## Extension Layering

The router supports two distinct extension layers:

- **Application extension path** - for required application adapters in `ext/extensions.go`
- **Optional extension path** - for router-extending capabilities in `ext/optional_extensions.go`

These layers must remain structurally separate in wiring even though both
ultimately register providers by port name.

### Layer Ordering

1. Optional extensions boot **first** - they may provide ports consumed by application extensions
2. Application extensions boot **second** - they depend on optional extensions' ports if needed

If a later extension depends on a port that was not made available by the
earlier phase, boot fails under the existing dependency/order semantics.

## Design Contracts

- Router depends only on `Extension` interface, never concrete adapters.
- Host supplies `ctx` for timeout/cancellation.
- Mutable files = `ports.go` + `registry_imports.go` + `ext/optional_extensions.go` + `ext/extensions.go`.
- Frozen files contain contracts + orchestration + publication logic only.
- Zero external dependencies.
- `Registry` handle is the only write surface for extensions.
- Single atomic publication is the only runtime source of truth.
- `PortContractMismatch` belongs in router for consistent structured errors.

## Related Documents

- [Concepts](concepts.md) - Core concepts explained in detail
- [API Reference](api-reference.md) - Complete API documentation
- [Usage Guide](usage.md) - How to use the router in your project
- [Security Model](security-model.md) - AI guardrails and protection mechanisms
