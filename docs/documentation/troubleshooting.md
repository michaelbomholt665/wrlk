# Router Troubleshooting Guide

This guide covers common errors, their causes, and how to fix them.

## Common Errors

### RegistryNotBooted

**Error:** `router registry not booted`

**Cause:** You're calling `RouterResolveProvider` before calling `RouterBootExtensions`.

**Fix:**

```go
// WRONG - resolves before boot
provider, _ := router.RouterResolveProvider(router.PortPrimary)

// CORRECT - boot first, then resolve
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if _, err := ext.RouterBootExtensions(ctx); err != nil {
    log.Fatal(err)
}

// Now safe to resolve
provider, _ := router.RouterResolveProvider(router.PortPrimary)
```

---

### PortNotFound

**Error:** `port "foo" not found`

**Cause:** The port is not registered in any extension.

**Fix:**

1. Verify the port exists in `ports.go`:
```go
const (
    PortFoo PortName = "foo"  // Make sure this exists
)
```

2. Verify the required extension is in `internal/router/ext/extensions.go`:
```go
var extensions = []router.Extension{
    &foo.Extension{},  // Or the concrete extension you wired
}
```

3. If the extension is optional, verify it's in `internal/router/ext/optional_extensions.go`:
```go
var optionalExtensions = []router.Extension{
    &foo.Extension{},  // For optional capabilities
}
```

---

### PortUnknown

**Error:** `port "foo" is not a declared port`

**Cause:** The extension tried to register a port that is not in the whitelist enforced by `RouterValidatePortName`.

**Fix:**

Add the case to `RouterValidatePortName` in `registry_imports.go`:

```go
func RouterValidatePortName(port PortName) bool {
    switch port {
    case PortPrimary, PortSecondary, PortTertiary, PortOptional, PortFoo:  // Add PortFoo here
        return true
    default:
        return false
    }
}
```

Or use the CLI to add the port correctly:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

---

### PortDuplicate

**Error:** `port "foo" already registered`

**Cause:** The same port is being registered by multiple extensions.

**Fix:**

Check your extension registrations:

```go
// In internal/router/ext/extensions.go
var extensions = []router.Extension{
    foo.Extension(),
    // foo.Extension() is registered twice?
}
```

Only one extension should register each port.

This can fail because:
- two different extensions both return the same port from `Provides()`
- an extension registers a port that was already staged by an earlier extension

---

### PortContractMismatch

**Error:** `provider for port "primary" does not satisfy the expected contract`

**Cause:** The provider doesn't implement the interface the consumer expects.

**Fix:**

1. Verify your provider implements the port interface:

```go
// In internal/ports/primary.go
type PrimaryProvider interface {
    DoSomething() error
}

// In internal/adapters/primary/extension.go
type PrimaryAdapter struct{}

func (p *PrimaryAdapter) DoSomething() error {  // Must match interface
    return nil
}
```

2. Verify the cast is correct:

```go
// WRONG - wrong interface
provider, _ := router.RouterResolveProvider(router.PortPrimary)
result := provider.(ports.WrongInterface)  // Error here!

// CORRECT - use the right interface
provider, _ := router.RouterResolveProvider(router.PortPrimary)
result := provider.(ports.PrimaryProvider)  // Correct
```

---

### DependencyOrderViolation

**Error:** `port "auth" dependency order violation. If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong. Move the providing extension higher up in the correct extensions slice.`

**Cause:** An extension declares a `Consumes()` port that isn't provided by an extension that loaded earlier.

**Fix:**

Check ordering in the correct slice:
- `internal/router/ext/extensions.go` for required application extensions
- `internal/router/ext/optional_extensions.go` for optional capability extensions

Example:

```go
// WRONG - consumer comes before provider
var extensions = []router.Extension{
    &authExtension{},     // Requires "auth" - too early!
    &authProvider{},      // Provides "auth"
}

// CORRECT - provider comes before consumer
var extensions = []router.Extension{
    &authProvider{},      // Provides "auth" - first
    &authExtension{},    // Requires "auth" - second
}
```

Or use the `Consumes()` declaration to let the router auto-sort:

```go
func (e *authExtension) Consumes() []router.PortName {
    return []router.PortName{router.PortPrimary}  // Router will order automatically
}
```

---

### AsyncInitTimeout

**Error:** `async extension initialization timed out: context deadline exceeded`

**Cause:** An async extension didn't complete before the context timeout.

**Fix:**

1. Increase the timeout in main.go:

```go
// Longer timeout for slower startup
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
```

2. Check the async extension for blocking operations:

```go
// WRONG - blocking operation without context
func (e *ext) RouterProvideAsyncRegistration(reg *router.Registry, ctx context.Context) error {
    result := blockingOperation()  // Ignores context!
    return reg.RouterRegisterProvider(router.PortCustom, result)
}

// CORRECT - use context
func (e *ext) RouterProvideAsyncRegistration(reg *router.Registry, ctx context.Context) error {
    result, err := operationWithContext(ctx, "args")
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return reg.RouterRegisterProvider(router.PortCustom, result)
}
```

---

### MultipleInitializations

**Error:** `router already initialized`

**Cause:** `RouterBootExtensions` was called more than once.

**Fix:**

Ensure boot is called exactly once:

```go
// WRONG - called twice in different places
func initRouter() {
    ext.RouterBootExtensions(ctx)  // First call
}

func main() {
    initRouter()
    initRouter()  // Error! Second call
}

// CORRECT - called once
func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    ext.RouterBootExtensions(ctx)  // Exactly once
}
```

---

### PortAccessDenied

**Error:** `consumer "my-service" access denied to restricted port "config"`

**Cause:** The consumer ID is not in the allowed list for a restricted port.

**Fix:**

Register the consumer during boot in your extension:

```go
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
    // Register provider
    if err := reg.RouterRegisterProvider(router.PortConfig, &ConfigProvider{}); err != nil {
        return fmt.Errorf("register config provider: %w", err)
    }

    // Allow specific consumers
    if err := reg.RouterRegisterPortRestriction(router.PortConfig, []string{
        "admin-service",
        "config-service",
        "my-service",  // Add your consumer here
    }); err != nil {
        return fmt.Errorf("restrict config port: %w", err)
    }

    // Or allow all consumers
    // reg.RouterRegisterPortRestriction(router.PortConfig, []string{"Any"})

    return nil
}
```

---

## Router Lock Issues

### Checksum Mismatch

**Error:** `router lock verify failed: checksum mismatch in internal/router/extension.go`

**Cause:** The frozen router files have been modified.

**Fix:**

```bash
# If the change was intentional, update the lock
go run ./internal/router/tools/wrlk lock update

# If the change was accidental, restore the snapshot
go run ./internal/router/tools/wrlk lock restore
```

---

## FAQ

### When should I use optional extensions vs application extensions?

**Application extensions** are required for the application to boot. They belong in `extensions.go`. If they fail, boot fails with a fatal error.

**Optional extensions** are capabilities that extend the router. They belong in `optional_extensions.go`. If they fail, boot continues with warnings.

Use optional extensions for:
- Telemetry and monitoring
- Logging enhancements
- Metrics collection
- CLI capabilities such as styling, chrome, or interactive prompts
- Cross-cutting concerns that shouldn't block startup

Use application extensions for:
- Core services (database, config, auth)
- Services required for the application to function

Wire them with:

```bash
go run ./internal/router/tools/wrlk ext app add --name billing
```

---

### How do I add a new port?

Use the CLI (recommended):

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

This handles:
- Adding the constant to `ports.go`
- Adding validation in `registry_imports.go`
- Updating `router.lock`

---

### RouterEnvironmentMismatch / RouterProfileInvalid

**Errors:**
- `router profile does not match the runtime environment: ROUTER_PROFILE="..." does not match WRLK_ENV="..."`
- `router profile is invalid: ROUTER_ALLOW_ANY=true is not allowed when WRLK_ENV="prod"`

**Cause:** `ext.RouterBootExtensions` validates boot policy before calling `router.RouterLoadExtensions`.

**Fix:**
- Align `ROUTER_PROFILE` with `WRLK_ENV`, or unset one of them.
- Do not set `ROUTER_ALLOW_ANY=true` when `WRLK_ENV=prod`.

---

### Can I register the same port in multiple extensions?

No. Each port can only have one provider. If you need multiple implementations, use a router that delegates to the appropriate one:

```go
type MultiProvider struct {
    primary Provider
    fallback Provider
}

func (m *MultiProvider) DoSomething() error {
    if m.primary != nil {
        return m.primary.DoSomething()
    }
    return m.fallback.DoSomething()
}
```

---

### Why is the router in internal/?

The router lives in `internal/` to ensure it's only accessible within the host project. This provides encapsulation and prevents external packages from depending on router internals.

---

### Can I use the router in library code?

The router is designed for application-level wiring. Library code should:
- Define port interfaces in `internal/ports/`
- Accept dependencies through constructors
- Not depend on the router directly

Consumers of the library use the router to provide the implementations.

---

### How do I test extensions?

Test extensions in isolation:

```go
func TestMyExtension(t *testing.T) {
    ext := &myExtension{}

    assert.True(t, ext.Required())
    assert.Equal(t, []router.PortName{router.PortMy}, ext.Provides())
    assert.Empty(t, ext.Consumes())
}
```

Also test the real integration path through `router.RouterLoadExtensions` or `ext.RouterBootExtensions`; direct construction of `router.Registry` is not available outside the `router` package.

---

## Related Documents

- [API Reference](api-reference.md) - Error codes and functions
- [Usage Guide](usage.md) - How to use the router correctly
- [CLI Tools](cli-tools.md) - CLI commands for troubleshooting
