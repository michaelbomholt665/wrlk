# Router Documentation

The Router is a zero-dependency dependency broker for Hexagonal Architecture in Go. It lives in `internal/router/` and provides a centralized port registry with AI development guardrails.

## What Is the Router?

The Router is a compile-time dependency injection system that:

- **Centralizes port declarations** - All port names are defined in one place (`ports.go`)
- **Wires adapters explicitly** - Required adapters live in `ext/extensions.go`; optional capability extensions live in `ext/optional_extensions.go`
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
- `internal/router/ext/extensions.go` - required application extensions
- `internal/router/ext/optional_extensions.go` - optional capability extensions

### Secondary Problem Solved: Shared Infrastructure Modification

The frozen/mutable split plus `router.lock` checksums make correct changes cheaper than wrong changes.

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

2. **Resolve providers** by port name:

```go
import (
    "your-project/internal/router"
    "your-project/internal/ports"
)

// Get the primary provider
provider, err := router.RouterResolveProvider(router.PortPrimary)
if err != nil {
    return err
}

// Cast to the port interface
primary, ok := provider.(ports.PrimaryProvider)
if !ok {
    return &router.RouterError{
        Code: router.PortContractMismatch,
        Port: router.PortPrimary,
    }
}

// Use the provider
result := primary.DoSomething()
```

### Adding a New Port

Use the CLI tool to add a new port:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

This command:
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
| [CLI Tools](cli-tools.md)             | CLI commands: wrlk lock, wrlk add, wrlk ext add, wrlk ext app add, wrlk guide |
| [Extension Authoring](extensions.md)  | Detailed guide for building real optional and app-consumable extensions      |
| [Troubleshooting](troubleshooting.md) | Common errors, FAQ, error code reference                                     |
| [Security Model](security-model.md)   | AI guardrails, router.lock, frozen/mutable protection                        |

## Key Properties

- **Zero external dependencies** - stdlib only
- **No runtime reflection** - compile-time safe
- **Lives in `internal/`** - host project encapsulation
- **Host-controlled policy** - validation, timeout, complexity rules defined by host
- **Copy-paste bundle** - drop into any Go project

## Package Structure

```
internal/router/
├── MUTABLE (host project wiring)
│   ├── ports.go              # PortName constants
│   ├── registry_imports.go   # Port validation + atomic registry
│   ├── ext/
│   │   ├── extensions.go         # Required application extensions
│   │   └── optional_extensions.go # Optional capability extensions
│
├── FROZEN (never edit directly)
│   ├── extension.go          # Extension interfaces + RouterLoadExtensions
│   └── registry.go          # Atomic publication + RouterResolveProvider
│
├── router.lock               # Checksums for frozen files
└── tools/wrlk/              # CLI for port management
```

For detailed architecture information, see [Architecture](architecture.md).
