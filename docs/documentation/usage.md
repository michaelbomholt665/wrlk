# Router Usage Guide

This guide provides step-by-step instructions for using the router in your project.

## What Is the Router?

The Router is a zero-dependency dependency broker for Hexagonal Architecture in Go. It centralizes port declarations and adapter wiring so adapters do not depend on each other directly.

## Why Use the Router?

- **Prevents coupling creep** - AI agents and developers can't bypass port boundaries
- **Compile-time safety** - Typed `PortName` catches errors at build time
- **Zero dependencies** - stdlib only, no external imports required
- **Lock-free reads** - O(1) provider resolution after boot

## Boot the Router

### Step 1: Call RouterBootExtensions Once at Startup

In your `main.go`, boot the router exactly once before any request handlers or workers begin:

```go
package main

import (
    "context"
    "log"
    "time"

    "your-project/internal/router/ext"
)

func main() {
    // Set up timeout - router respects host cancellation policy
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Boot the router once
    warnings, err := ext.RouterBootExtensions(ctx)
    if err != nil {
        log.Fatal("router boot failed:", err)
    }

    // Log any warnings from optional extensions
    for _, w := range warnings {
        log.Println("router warning:", w)
    }

    // Router is now booted - start your application
    log.Println("Router initialized successfully")
    // ... continue with app startup
}
```

**Important:**
- Call `RouterBootExtensions` exactly once
- Pass a context with timeout - the router doesn't hardcode `context.Background()`
- Handle warnings from optional extensions (they don't halt boot)

## Resolve Providers

### Step 2: Resolve and Cast Providers

After boot, resolve providers by port name and cast to the expected interface:

```go
import (
    "fmt"

    "your-project/internal/router"
    "your-project/internal/ports"
)

func getPrimaryProvider() (ports.PrimaryProvider, error) {
    // Resolve the provider
    provider, err := router.RouterResolveProvider(router.PortPrimary)
    if err != nil {
        return nil, fmt.Errorf("resolve primary provider: %w", err)
    }

    // Cast to the port interface
    primary, ok := provider.(ports.PrimaryProvider)
    if !ok {
        return nil, &router.RouterError{
            Code: router.PortContractMismatch,
            Port: router.PortPrimary,
        }
    }

    return primary, nil
}
```

### Using Restricted Ports

For ports with access restrictions, use `RouterResolveRestrictedPort`:

```go
func getPrimaryForService(serviceName string) (ports.PrimaryProvider, error) {
    provider, err := router.RouterResolveRestrictedPort(
        router.PortPrimary,
        serviceName, // consumer ID
    )
    if err != nil {
        return nil, fmt.Errorf("resolve primary for %s: %w", serviceName, err)
    }

    primary, ok := provider.(ports.PrimaryProvider)
    if !ok {
        return nil, &router.RouterError{
            Code: router.PortContractMismatch,
            Port: router.PortPrimary,
        }
    }

    return primary, nil
}
```

## Add a New Port

### Using the CLI (Recommended)

The easiest way to add a new port is using the `wrlk add` command:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

This command:
1. Adds `PortFoo PortName = "foo"` to `ports.go`
2. Adds the validation case to `registry_imports.go`
3. Rewrites `router.lock` atomically

**Dry Run:**

To see what would change without making changes:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo --dry-run
```

### Manual Addition (Not Recommended)

If you must add manually, follow these steps:

1. **Add constant to `ports.go`:**
```go
const (
    // ... existing ports
    PortFoo PortName = "foo"
)
```

2. **Add validation case to `registry_imports.go`:**
```go
func RouterValidatePortName(port PortName) bool {
    switch port {
    case PortPrimary, PortSecondary, PortTertiary, PortOptional, PortFoo:
        return true
    default:
        return false
    }
}
```

3. **Define the port interface in `internal/ports/`:**
```go
// Package ports defines the port contracts for the router.
package ports

type FooProvider interface {
    DoSomething() error
}
```

4. **Implement the adapter:**
```go
// Package foo implements the foo port.
package foo

type FooAdapter struct{}

func (a *FooAdapter) DoSomething() error {
    // Implementation
    return nil
}
```

5. **Wire the extension in `internal/router/ext/extensions.go`:**
```go
import "your-project/internal/router"

var extensions = []router.Extension{
    // ... existing
    &fooExtension{},
}
```

## Create a New Extension

### Full Extension Example

Here's a complete example of implementing a new extension:

```go
package config

import (
    "fmt"

    "your-project/internal/router"
)

// Extension implements router.Extension for the config adapter.
type Extension struct{}

// Required returns true - config is mandatory for boot.
func (e *Extension) Required() bool {
    return true
}

// Consumes returns nil - no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName {
    return nil
}

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName {
    return []router.PortName{router.PortPrimary}
}

// RouterProvideRegistration registers the config provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
    provider := &ConfigProvider{
        // Initialize your provider
    }
    return reg.RouterRegisterProvider(router.PortPrimary, provider)
}

// Extension returns the extension instance.
func Extension() *Extension {
    return &Extension{}
}

// ConfigProvider is the provider for the config port.
type ConfigProvider struct {
    // Your provider fields
}

func (p *ConfigProvider) Get(key string) (string, error) {
    // Implementation
    return "", nil
}
```

### Optional Extension

For optional capabilities (boot continues on failure):

```go
type telemetryExtension struct{}

func (e *telemetryExtension) Required() bool {
    return false  // Optional - failures become warnings
}

func (e *telemetryExtension) Consumes() []router.PortName {
    return nil
}

func (e *telemetryExtension) Provides() []router.PortName {
    return []router.PortName{router.PortOptional}
}

func (e *telemetryExtension) RouterProvideRegistration(reg *router.Registry) error {
    provider := &TelemetryProvider{}
    return reg.RouterRegisterProvider(router.PortOptional, provider)
}

func Extension() router.Extension {
    return &telemetryExtension{}
}
```

Add to `internal/router/ext/optional_extensions.go`:

```go
var optionalExtensions = []router.Extension{
    &telemetry.Extension{},  // Added here, not in ext/extensions.go
}
```

## Extension with Async Initialization

For extensions that need async setup:

```go
type databaseExtension struct{}

func (e *databaseExtension) Required() bool { return true }
func (e *databaseExtension) Consumes() []router.PortName { return nil }
func (e *databaseExtension) Provides() []router.PortName {
    return []router.PortName{router.PortTertiary}
}

func (e *databaseExtension) RouterProvideRegistration(reg *router.Registry) error {
    // Synchronous setup (struct initialization)
    reg.RouterRegisterProvider(router.PortTertiary, &DBProvider{})
    return nil
}

// RouterProvideAsyncRegistration handles async initialization.
// This is called after RouterProvideRegistration.
func (e *databaseExtension) RouterProvideAsyncRegistration(
    reg *router.Registry,
    ctx context.Context,
) error {
    // Connect to database with context for timeout/cancellation
    db, err := connectWithContext(ctx, "connection-string")
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }

    // Update provider with connected instance
    provider := &DBProvider{conn: db}
    return reg.RouterRegisterProvider(router.PortTertiary, provider)
}

// Embed the async interface
var _ router.AsyncExtension = (*databaseExtension)(nil)
```

## Import Rule

Always route capability imports through these layers:

```text
consumer -> internal/ports + internal/router
host boot -> internal/router/ext
internal/router/ext -> internal/adapters/*
```

**Don't do this:**

```go
// BAD: Direct adapter import
import "your-project/internal/adapters/walk"

func walk(root string) error {
    walker := &walk.Walker{}  // Bypasses router!
    return walker.Walk(root)
```

**Do this:**

```go
// GOOD: Resolve from router
import (
    "your-project/internal/router"
    "your-project/internal/ports"
)

func runPrimary() error {
    provider, _ := router.RouterResolveProvider(router.PortPrimary)
    primary := provider.(ports.PrimaryProvider)
    return primary.DoSomething()
}
```

## What Lives Where

| Layer                  | Location                            | Purpose                    |
| ---------------------- | ----------------------------------- | -------------------------- |
| Port contract          | `internal/ports/*.go`               | Define interface contracts |
| Adapter implementation | `internal/adapters/*/extension.go`  | Implement ports            |
| Router wiring          | `internal/router/ext/extensions.go` | Register adapters          |
| Port constants         | `internal/router/ports.go`          | Declare port names         |
| Provider resolution    | `internal/router/registry.go`       | Resolve by port name       |

## Related Documents

- [Architecture](architecture.md) - Deep dive on folder structure
- [Concepts](concepts.md) - Core concepts explained
- [API Reference](api-reference.md) - Complete API documentation
- [CLI Tools](cli-tools.md) - CLI commands reference
