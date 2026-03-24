# API Reference

This document provides complete API documentation for all public router functions and types.

## Core Functions

### RouterResolveProvider

Resolves a provider from the published registry by port name.

```go
func RouterResolveProvider(port PortName) (Provider, error)
```

**Parameters:**
- `port` - The port name to resolve (e.g., `router.PortPrimary`)

**Returns:**
- `Provider` - The registered provider, or `nil` if not found
- `error` - An error if resolution failed

**Example:**

```go
provider, err := router.RouterResolveProvider(router.PortPrimary)
if err != nil {
    // Handle error
    return err
}

// Cast to the expected interface
primary, ok := provider.(ports.PrimaryProvider)
if !ok {
    return &router.RouterError{
        Code: router.PortContractMismatch,
        Port: router.PortPrimary,
    }
}
```

**Error Codes:**
- `PortNotFound` - Port is not registered
- `RegistryNotBooted` - Router has not been booted yet

---

### RouterResolveRestrictedPort

Resolves a provider with consumer access control. The consumer ID must be in the allowed list for the port.

```go
func RouterResolveRestrictedPort(port PortName, consumerID string) (Provider, error)
```

**Parameters:**
- `port` - The port name to resolve
- `consumerID` - The consumer identity requesting access

**Returns:**
- `Provider` - The registered provider, or `nil` if not found
- `error` - An error if resolution or access check failed

**Example:**

```go
provider, err := router.RouterResolveRestrictedPort(router.PortConfig, "admin-service")
if err != nil {
    return err
}
```

**Error Codes:**
- `PortNotFound` - Port is not registered
- `RegistryNotBooted` - Router has not been booted yet
- `PortAccessDenied` - Consumer ID is not in the allowed list

---

### RouterBootExtensions

Boots the router by loading optional extensions first, then application extensions.

```go
func RouterBootExtensions(ctx context.Context) ([]error, error)
```

**Note:** This function is in the `ext` subpackage:

```go
import "your-project/internal/router/ext"

warnings, err := ext.RouterBootExtensions(ctx)
```

**Parameters:**
- `ctx` - Context for timeout/cancellation. Required - hardcoding context inside the router is forbidden.

**Returns:**
- `[]error` - Warnings from optional extensions that failed to load (boot continues)
- `error` - Fatal error if boot failed

**Example:**

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

warnings, err := ext.RouterBootExtensions(ctx)
if err != nil {
    log.Fatal(err)
}
for _, w := range warnings {
    log.Println("warning:", w)
}
```

---

### RouterLoadExtensions

Loads extension registrations and publishes the registry. This is the core boot function in the frozen router package.

```go
func RouterLoadExtensions(
    optionalExts []Extension,
    exts []Extension,
    ctx context.Context,
) ([]error, error)
```

**Parameters:**
- `optionalExts` - Optional extensions to load first
- `exts` - Application extensions to load second
- `ctx` - Context for async extension initialization

**Returns:**
- `[]error` - Warnings from extensions that failed (if optional)
- `error` - Fatal error if boot failed

---

## Registry Handle Methods

### RouterRegisterProvider

Registers a provider for a port in the local boot registry.

```go
func (r *Registry) RouterRegisterProvider(port PortName, provider Provider) error
```

**Parameters:**
- `port` - The port name to register
- `provider` - The provider implementation

**Error Codes:**
- `PortUnknown` - Port is not declared in ports.go
- `InvalidProvider` - Provider is nil
- `PortDuplicate` - Port already has a provider

---

### RouterRegisterPortRestriction

Adds an access restriction to a port during boot.

```go
func (r *Registry) RouterRegisterPortRestriction(port PortName, allowedConsumerIDs []string) error
```

**Parameters:**
- `port` - The port to restrict
- `allowedConsumerIDs` - List of consumer IDs that can access this port. Use "Any" to allow all.

**Example:**

```go
// Restrict config port to specific services
reg.RouterRegisterPortRestriction(router.PortConfig, []string{"admin-service", "config-service"})

// Allow any consumer
reg.RouterRegisterPortRestriction(router.PortPublic, []string{"Any"})
```

---

## Type Definitions

### PortName

```go
type PortName string
```

Typed string identifier for router ports. Use constants from `ports.go` instead of raw strings.

### Provider

```go
type Provider any
```

The registered implementation for a router port. Consumers must cast to the expected interface.

### RouterError

```go
type RouterError struct {
    Code       RouterErrorCode  // Error code
    Port       PortName          // Associated port (if applicable)
    ConsumerID string            // Consumer ID (if applicable)
    Err        error            // Underlying cause
}
```

Structured router error type.

### Extension Interface

```go
type Extension interface {
    Required() bool
    Consumes() []PortName
    Provides() []PortName
    RouterProvideRegistration(reg *Registry) error
}
```

---

## Error Codes

| Code                        | Description                        | When Raised                                    |
| --------------------------- | ---------------------------------- | ---------------------------------------------- |
| `PortUnknown`               | Port is not declared               | Registration with undeclared port              |
| `PortDuplicate`             | Port already registered            | Multiple registrations for same port           |
| `InvalidProvider`           | Provider is nil                    | Registration with nil provider                 |
| `PortNotFound`              | Port not in registry               | Resolution of unregistered port                |
| `RegistryNotBooted`         | Router not initialized             | Resolution before boot                         |
| `PortContractMismatch`      | Provider doesn't satisfy interface | Consumer type assertion fails                  |
| `RequiredExtensionFailed`   | Required extension failed          | Mandatory extension error during boot          |
| `OptionalExtensionFailed`   | Optional extension failed          | Optional extension error (warning, not fatal)  |
| `DependencyOrderViolation`  | Port dependency not satisfied      | Extension consumes port not yet available      |
| `AsyncInitTimeout`          | Async init timed out               | Context deadline exceeded during boot          |
| `MultipleInitializations`   | Router booted twice                | Boot attempted after successful initialization |
| `RouterCyclicDependency`    | Circular dependency detected       | Extension cycle in dependency graph            |
| `PortAccessDenied`          | Consumer not allowed               | Restricted port access denied                  |
| `RouterProfileInvalid`      | Invalid profile configuration      | Environment policy violation                   |
| `RouterEnvironmentMismatch` | Profile doesn't match environment  | Declared profile differs from WRLK_ENV         |

---

## Error Handling Example

```go
func resolveWithErrorHandling(port router.PortName) (interface{}, error) {
    provider, err := router.RouterResolveProvider(port)
    if err != nil {
        var routerErr *router.RouterError
        if errors.As(err, &routerErr) {
            switch routerErr.Code {
            case router.RegistryNotBooted:
                return nil, fmt.Errorf("router not initialized: call RouterBootExtensions first")
            case router.PortNotFound:
                return nil, fmt.Errorf("port %q not registered", port)
            case router.PortAccessDenied:
                return nil, fmt.Errorf("access denied for port %q", port)
            default:
                return nil, fmt.Errorf("router error [%s]: %w", routerErr.Code, err)
            }
        }
        return nil, fmt.Errorf("resolve provider: %w", err)
    }
    return provider, nil
}
```

---

## Related Documents

- [Architecture](architecture.md) - Bootstrap flow and publication model
- [Concepts](concepts.md) - Core concepts explained
- [Usage Guide](usage.md) - How to use the router
- [Troubleshooting](troubleshooting.md) - Common errors and solutions