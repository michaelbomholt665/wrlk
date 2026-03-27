# Router Documentation

The Router is a zero-internal-dependency port registry and extension boot layer for Go applications. It lives in `internal/router/` and provides centralized port registration, explicit extension wiring, and guardrails around router evolution.

## What Is the Router?

The Router is a compile-time dependency injection system that:

- **Centralizes port declarations** - All port names are defined in one place (`ports.go`)
- **Wires extensions explicitly** - Required application extensions generate into `ext/extensions.go` and may be empty; optional capability extensions generate into `ext/optional_extensions.go`
- **Uses manifests as the edit surface** - Router-owned declarations live in `router_manifest.go` and `ext/app_manifest.go`
- **Prevents coupling creep** - Frozen/mutable file split stops accidental modifications to core contracts
- **Provides lock-free reads** - Uses `atomic.Pointer` for O(1) provider resolution after boot

## Why Use the Router?

### Primary Problem Solved: AI-Driven Coupling Creep

Without guardrails, AI agents (and developers) tend to:
- Create cross-adapter direct calls
- Add hidden dependencies
- Take local shortcuts that bypass port boundaries

The Router solves this by making dependency wiring an auditable declaration surface. Cross-adapter coupling requires explicit changes to:
- [`ports.go`](architecture.md#portsgo--mutable-port-whitelist) - port whitelist
- `internal/router/ext/app_manifest.go` - required application extension declarations
- `internal/router/router_manifest.go` - router-native port and optional extension declarations

### Secondary Problem Solved: Shared Infrastructure Modification

The frozen/mutable split plus `router.lock` checksums over the managed router files make correct changes cheaper than wrong changes.

## How to Use the Router

### Quick Start

1. **Boot the router** once at startup:

```go
package main

import (
    "context"
    "log"
    "time"

    "your-project/internal/router/ext"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    warnings, err := ext.RouterBootExtensions(ctx)
    if err != nil {
        log.Fatal(err)
    }
    for _, w := range warnings {
        log.Println(w)
    }

    // Router is now booted. Use RouterResolveProvider to get providers.
}
```

2. **Resolve providers** through a port or a typed capability resolver:

```go
import "github.com/michaelbomholt665/wrlk/internal/router/capabilities"

styler, err := capabilities.ResolveCLIOutputStyler()
if err != nil {
    return err
}

rendered, err := styler.StyleText(capabilities.TextKindInfo, "router booted")
if err != nil {
    return err
}

_ = rendered
```

### Adding a New Port

Use the CLI tool to add a new port:

```bash
go run ./internal/router/tools/wrlk register --port --router --name PortFoo --value foo
```

This command:
- Adds the declaration to `internal/router/router_manifest.go`
- Adds the constant to [`ports.go`](architecture.md#portsgo--mutable-port-whitelist)
- Adds the case to [`registry_imports.go`](architecture.md#registry_importsgo--mutable)
- Rewrites [`router.lock`](architecture.md#routerlock--ndjson-checksums) atomically

See [CLI Tools](cli-tools.md) for more commands.

## Documentation Structure

| Document                              | Description                                                                  |
| ------------------------------------- | ---------------------------------------------------------------------------- |
| [Architecture](architecture.md)       | Deep dive into folder structure, frozen/mutable split, bootstrap flow        |
| [Concepts](concepts.md)               | Core concepts: PortName, Extension interfaces, Registry, dependency ordering |
| [API Reference](api-reference.md)     | API documentation for all public functions and error codes                   |
| [Usage Guide](usage.md)               | Step-by-step usage: boot, resolve, extend, add ports                         |
| [CLI Tools](cli-tools.md)             | CLI commands: `wrlk lock`, `wrlk register`, `wrlk ext`, `wrlk live`, `wrlk guide` |
| [Extension Authoring](extensions.md)  | Detailed guide for building real optional and app-consumable extensions      |
| [Troubleshooting](troubleshooting.md) | Common errors, FAQ, error code reference                                     |
| [Security Model](security-model.md)   | AI guardrails, router.lock, frozen/mutable protection                        |

## Key Properties

- **Router core uses the standard library only**
- **No runtime reflection** - compile-time safe
- **Lives in `internal/`** - host project encapsulation
- **Host-controlled boot policy** - environment/profile checks stay at the `ext` layer
- **Copy-paste bundle** - drop into any Go project

## Package Structure

```
internal/router/
├── MUTABLE (host project wiring)
│   ├── ports.go              # PortName constants
│   ├── registry_imports.go   # Port validation + atomic registry state
│   ├── router_manifest.go    # Source of truth for ports + router-owned extensions
│   ├── ext/
│   │   ├── app_manifest.go        # Source of truth for required app extensions
│   │   ├── extensions.go          # Generated required application extensions + boot policy wrapper
│   │   └── optional_extensions.go # Generated optional capability extensions
│
├── FROZEN (never edit directly)
│   ├── extension.go          # Extension interfaces + RouterLoadExtensions
│   ├── registry.go           # Provider resolution + restricted resolution
│   ├── error_surface.go      # Router error rendering
│   └── capabilities.go       # Declared capability manifest
│
├── router.lock               # Checksums for the managed router files
└── tools/wrlk/              # CLI for port management
```

For detailed architecture information, see [Architecture](architecture.md).
