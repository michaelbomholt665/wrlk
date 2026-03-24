# Router Package

A zero-dependency port registry and extension boot machinery for Go applications.

## Overview

The router implements an extension-based dependency injection system with:

- **Port-based registry**: Providers are registered and resolved by named ports
- **Extension lifecycle**: Required and optional extensions with async initialization support
- **Lock-free reads**: Atomic pointer-based registry for high-performance concurrent access
- **Structured errors**: Comprehensive error catalog with contextual error messages

## Architecture

```mermaid
---
config:
  layout: dagre
  theme: redux-dark
  look: classic
---
flowchart LR
 subgraph ExtLayer["Entry & Wiring (Ext)"]
        direction TB
        RouterExt["internal/router/ext"]
        ExtSubExt["internal/router/ext/extensions/telemetry"]
  end
        RouterExt -- contains --> ExtSubExt
 subgraph RouterCore["Top Layer (Core - Frozen & Blind)"]
        CoreRouter["internal/router"]
  end
 subgraph AdaptersLayer["Middle Layer (Adapters - Implementations)"]
    direction LR
        AdapterConfig[".../adapters/config"]
        AdapterScanners[".../adapters/scanners"]
        AdapterWalk[".../adapters/walk"]
  end
 subgraph PortsLayer["Bottom Layer (Ports - Interfaces)"]
        Ports["internal/ports"]
  end
    AdapterConfig -- implements --> Ports
    AdapterScanners -- implements --> Ports
    AdapterWalk -- implements --> Ports
    AdapterConfig -. references .-> CoreRouter
    AdapterScanners -. references .-> CoreRouter
    AdapterWalk -. references .-> CoreRouter
    RouterExt -- registers --> CoreRouter
    RouterExt -- instantiates --> AdapterConfig & AdapterScanners & AdapterWalk
    Ports == 🚫 ILLEGAL: Upward dependency forbidden! ==> CoreRouter
    AdapterConfig == 🚫 ILLEGAL: Adapters cannot call each other ==> AdapterScanners
    AdapterScanners == "🚫 ILLEGAL: Cross-adapter highway forbidden" ==> AdapterWalk
    CoreRouter == 🚫 ILLEGAL: Router must remain blind to impl ==> AdapterConfig
    CoreRouter == 🚫 ILLEGAL: Router does not know of ports ==> Ports
    Ports L_Ports_AdapterConfig_0@== 🚫 ILLEGAL: Ports cannot import implementations ==> AdapterConfig
    n1["Rectangle"]

    n1@{ shape: rect}
     RouterExt:::ext
     CoreRouter:::core
     AdapterConfig:::adapters
     AdapterScanners:::adapters
     AdapterWalk:::adapters
     Ports:::ports
    classDef ports fill:#fce4ec,stroke:#c2185b,stroke-width:2px,color:#880e4f
    classDef adapters fill:#e8f5e9,stroke:#388e3c,stroke-width:2px,color:#1b5e20
    classDef core fill:#e3f2fd,stroke:#1976d2,stroke-width:2px,color:#0d47a1
    classDef ext fill:#fff3e0,stroke:#f57c00,stroke-width:2px,color:#e65100
    linkStyle 10 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none
    linkStyle 11 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none
    linkStyle 12 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none
    linkStyle 13 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none
    linkStyle 14 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none
    linkStyle 15 stroke:#d32f2f,stroke-width:3px,stroke-dasharray: 5 5,fill:none

    L_Ports_AdapterConfig_0@{ animation: slow }
```

## Core Concepts

### Ports

Ports are typed identifiers for provider capabilities:

| Port            | Description                        |
| --------------- | ---------------------------------- |
| `PortPrimary`   | Primary provider port              |
| `PortSecondary` | Secondary provider port            |
| `PortTertiary`  | Tertiary provider port             |
| `PortOptional`  | Optional provider port (telemetry) |

### Extensions

Extensions register providers during boot:

- **Required**: Boot fails if registration fails
- **Optional**: Boot continues with warnings on failure
- **Async**: Support for asynchronous initialization
- **Topological sorting**: Extensions are sorted by dependency order

### Registry

The registry uses `atomic.Pointer` for lock-free reads after boot:

```go
provider, err := router.RouterResolveProvider(router.PortPrimary)
```

## CLI Tools

### wrlk

Manage port generation, router lock verification, and live sessions:

```bash
# Add a new port
go run ./internal/router/tools/wrlk add --name PortFoo --value foo

# Dry-run a new port addition
go run ./internal/router/tools/wrlk add --name PortFoo --value foo --dry-run

# Verify lock file
go run ./internal/router/tools/wrlk lock verify

# Update lock file
go run ./internal/router/tools/wrlk lock update

# Restore lock file
go run ./internal/router/tools/wrlk lock restore

# Run live verification
go run ./internal/router/tools/wrlk live run --expect participant1 --timeout 30s

# Scaffold a new extension
go run ./internal/router/tools/wrlk ext add --name ExtensionName

# Show operational guide
go run ./internal/router/tools/wrlk guide
```

## File Structure

```
internal/router/
├── doc.go                 # Package documentation
├── ports.go               # Port type definitions
├── registry.go            # Provider resolution
├── registry_imports.go    # Registry implementation
├── extension.go           # Extension interfaces & boot
├── error_surface.go       # Error handling
├── test_reset.go          # Test utilities
├── router.lock            # Lock file (anti-tampering)
├── ext/
│   ├── doc.go
│   ├── extensions.go      # Required/application extensions
│   ├── optional_extensions.go  # Optional extensions (capabilities that extend without adding dependencies to core)
│   └── extensions/
│       └── telemetry/    # Optional telemetry extension
└── tools/
    └── wrlk/              # CLI tools (portgen, lock, live, ext)

internal/tests/router/
├── boot_test.go
├── restricted_test.go
├── registration_test.go
├── resolution_test.go
├── helpers_test.go
├── benchmark_test.go
├── ext_boot_test.go
└── tools/
    └── wrlk/              # CLI tool tests
```

## Design Principles

1. **Zero dependencies in core**: `internal/router` imports nothing from internal packages
2. **Clean architecture**: Extensions implement ports defined in `internal/router`
3. **Type safety**: Port names are strongly typed
4. **Immutable after boot**: Registry is read-only after initialization
5. **Fail fast**: Required extension failures abort boot immediately
6. **Lock protection**: Core files are checksum-protected via `router.lock`

## Lock Protocol

The router core is protected by `router.lock`. Manual edits to core files will fail lock verification:

- Use `wrlk add` to add new ports (automates updates and lock recalculation)
- Use `wrlk lock verify` to detect drift
- Use `wrlk lock restore` to revert local changes

## Error Codes

| Code                        | Description                        |
| --------------------------- | ---------------------------------- |
| `PortUnknown`               | Port not declared in whitelist     |
| `PortDuplicate`             | Port already registered            |
| `InvalidProvider`           | Provider is nil                    |
| `PortNotFound`              | Port not registered                |
| `RegistryNotBooted`         | Resolution before boot             |
| `PortContractMismatch`      | Provider does not satisfy contract |
| `RequiredExtensionFailed`   | Required extension error           |
| `OptionalExtensionFailed`   | Optional extension error           |
| `DependencyOrderViolation`  | Dependency not satisfied           |
| `AsyncInitTimeout`          | Async initialization timeout       |
| `MultipleInitializations`   | Boot called twice                  |
| `RouterCyclicDependency`    | Circular dependency detected       |
| `PortAccessDenied`          | Consumer not allowed               |
| `RouterProfileInvalid`      | Invalid router profile             |
| `RouterEnvironmentMismatch` | Profile doesn't match environment  |

## Environment Variables

| Variable           | Description                              |
| ------------------ | ---------------------------------------- |
| `WRLK_ENV`         | Runtime environment (dev, staging, prod) |
| `ROUTER_PROFILE`   | Declared router profile                  |
| `ROUTER_ALLOW_ANY` | Allow unrestricted port access           |

## Testing

Run router tests:

```bash
go test ./internal/tests/router/... -v
```

Run benchmarks:

```bash
go test ./internal/tests/router/... -bench=. -benchtime=3s
