# Router Concepts

This document explains the core concepts of the router: `PortName`, extension interfaces, the boot `Registry`, restrictions, and dependency ordering.

## PortName

`PortName` is a typed string identifier that prevents raw string values from being passed to provider registration or resolution functions. The compiler enforces correct usage.

```go
type PortName string
```

**Why use typed ports?**

- Compiler catches typos at compile time
- IDE autocomplete works for port constants
- No runtime surprises from misspelled port names
- Clear documentation of available ports in one place

**Example:**

```go
// Valid - using the constant
provider, err := router.RouterResolveProvider(router.PortPrimary)

// Invalid - compiler error: cannot use string literal
provider, err := router.RouterResolveProvider("primary")

// Invalid - compiler error: cannot use untyped string variable
var portName = "primary"
provider, err := router.RouterResolveProvider(portName)
```

## Extension Interface

The `Extension` interface is the core contract that router extensions implement to register with the router.

```go
type Extension interface {
    // Required reports whether this extension is mandatory for boot.
    // Required extensions cause fatal errors if they fail to load.
    // Optional extensions produce warnings but boot continues.
    Required() bool

    // Consumes reports the ports this extension depends on during boot.
    // The router uses this for dependency ordering and validation.
    Consumes() []PortName

    // Provides reports the ports this extension registers.
    Provides() []PortName

    // RouterProvideRegistration is called during boot to register
    // the extension's providers into the local boot registry.
    RouterProvideRegistration(reg *Registry) error
}
```

### Complete Example: Implementing an Extension

```go
package config

import (
    "fmt"

    "your-project/internal/router"
)

// Extension implements router.Extension for the config adapter.
type Extension struct{}

// Required returns true - config is required for the application to boot.
func (e *Extension) Required() bool {
    return true
}

// Consumes returns nil - this extension has no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName {
    return nil
}

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName {
    return []router.PortName{router.PortPrimary}
}

// RouterProvideRegistration registers the config provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
    provider := &ConfigProvider{ /* ... */ }
    return reg.RouterRegisterProvider(router.PortPrimary, provider)
}
```

Wire required application adapters with `wrlk ext app add`, scaffold new optional capability packages with `wrlk ext add`, and wire existing optional capability packages with `wrlk ext install`.

### RollbackExtension Interface

Extensions that start work during boot can opt into boot-only rollback:

```go
type RollbackExtension interface {
    Extension
    RouterRollbackBoot(ctx context.Context) error
}
```

Rollback runs only when boot aborts after startup work began. It is not a general runtime shutdown hook.

## AsyncExtension Interface

For extensions that need asynchronous initialization (e.g., establishing connections, loading resources), implement `AsyncExtension`:

```go
type AsyncExtension interface {
    Extension
    RouterProvideAsyncRegistration(reg *Registry, ctx context.Context) error
}
```

**Example:**

```go
type databaseExtension struct{}

func (e *databaseExtension) Required() bool { return true }
func (e *databaseExtension) Consumes() []router.PortName { return nil }
func (e *databaseExtension) Provides() []router.PortName {
    return []router.PortName{router.PortTertiary}
}
func (e *databaseExtension) RouterProvideRegistration(reg *router.Registry) error {
    // Synchronous registration (e.g., struct setup)
    return reg.RouterRegisterProvider(router.PortTertiary, &DBProvider{})
}

// RouterProvideAsyncRegistration handles async initialization.
func (e *databaseExtension) RouterProvideAsyncRegistration(reg *router.Registry, ctx context.Context) error {
    // Finish initialization that depends on the host context.
    if err := ConnectWithTimeout(ctx, "connection-string"); err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }

    return nil
}
```

`RouterProvideRegistration` runs first against a staged registry clone. `RouterProvideAsyncRegistration` then runs against the same staged clone before that staged state is committed.

## ErrorFormattingExtension Interface

Extensions can provide custom error formatting for more meaningful error messages:

```go
type ErrorFormattingExtension interface {
    Extension
    ErrorFormatter() RouterErrorFormatter
}

// RouterErrorFormatter is a function that transforms errors.
type RouterErrorFormatter func(err error) error
```

**Example:**

```go
type customExtension struct{}

func (e *customExtension) Required() bool { return true }
func (e *customExtension) Consumes() []router.PortName { return nil }
func (e *customExtension) Provides() []router.PortName {
    return []router.PortName{router.PortCustom}
}
func (e *customExtension) RouterProvideRegistration(reg *router.Registry) error {
    return reg.RouterRegisterProvider(router.PortCustom, &CustomProvider{})
}

// ErrorFormatter returns a custom error formatter.
func (e *customExtension) ErrorFormatter() router.RouterErrorFormatter {
    return func(err error) error {
        if err == nil {
            return nil
        }
        return fmt.Errorf("[custom] original error: %w", err)
    }
}
```

## Registry Handle

The `Registry` handle is the **only** write surface that extensions use during boot. It provides methods to register providers and configure access restrictions.

```go
type Registry struct {
    ports        *map[PortName]Provider
    restrictions *map[PortName][]string
}
```

### Registering a Provider

```go
func (r *Registry) RouterRegisterProvider(port PortName, provider Provider) error
```

### Registering Port Restrictions

For restricted ports that require consumer identity validation:

```go
func (r *Registry) RouterRegisterPortRestriction(port PortName, allowedConsumerIDs []string) error
```

If no restriction is registered for a port, `RouterResolveRestrictedPort` allows any non-empty consumer ID. If a restriction exists, access is granted only when the consumer ID matches an allowed value or the allow list contains `"Any"`.

## Dependency Ordering

The router performs topological sorting on extensions based on their `Consumes()` declarations. This ensures ports are available before extensions that need them.

### How It Works

1. Build a dependency graph from `Consumes()` and `Provides()` declarations
2. Detect cycles (returns `RouterCyclicDependency` error)
3. Use Kahn's algorithm to sort extensions
4. Load extensions in sorted order

### Example: Dependency Chain

```go
// Extension A provides port "auth", no dependencies
type extensionA struct{}
func (e *extensionA) Required() bool { return true }
func (e *extensionA) Consumes() []router.PortName { return nil }
func (e *extensionA) Provides() []router.PortName { return []router.PortName{"auth"} }
func (e *extensionA) RouterProvideRegistration(reg *router.Registry) error {
    return reg.RouterRegisterProvider("auth", &AuthProvider{})
}

// Extension B consumes "auth", provides "users"
type extensionB struct{}
func (e *extensionB) Required() bool { return true }
func (e *extensionB) Consumes() []router.PortName { return []router.PortName{"auth"} }
func (e *extensionB) Provides() []router.PortName { return []router.PortName{"users"} }
func (e *extensionB) RouterProvideRegistration(reg *router.Registry) error {
    return reg.RouterRegisterProvider("users", &UsersProvider{})
}
```

The router automatically orders B after A because B consumes "auth" which A provides.

### Manual Ordering Still Works

The router sorts each extension layer before boot. Manual slice order is still the tie-breaker when there is no declared dependency edge.

## Key Terms Summary

| Term                     | Description                                                |
| ------------------------ | ---------------------------------------------------------- |
| PortName                 | Typed string identifier for router ports                   |
| Provider                 | The registered implementation for a port                   |
| Extension                | Interface that adapters implement to register with router  |
| AsyncExtension           | Extension with async initialization capability             |
| ErrorFormattingExtension | Extension with custom error formatting                     |
| Registry                 | Handle for registering providers during boot               |
| Frozen files             | Core router files that should never be edited directly     |
| Mutable files            | Host project wiring files that define ports and extensions |

## Related Documents

- [Architecture](architecture.md) - Folder structure and bootstrap flow
- [API Reference](api-reference.md) - Complete function documentation
- [Usage Guide](usage.md) - How to use the router
- [Security Model](security-model.md) - AI guardrails
