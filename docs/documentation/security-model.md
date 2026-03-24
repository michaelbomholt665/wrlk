# Router Security Model

This document explains the router's security model and AI guardrails.

## What the Router Protects Against

The router is a **development integrity and structural constraint system**, not a runtime sandbox. It protects against:

| Concern                       | Router Contribution                      | Protection Type                            |
| ----------------------------- | ---------------------------------------- | ------------------------------------------ |
| External package reachability | `internal/` placement                    | Encapsulation boundary inside host project |
| Typo / wrong port literals    | Typed `PortName` + whitelist validation  | Compile-time correctness guard             |
| Late mutation after boot      | Immutable published snapshot             | Prevents post-boot registration            |
| Port shadowing                | Duplicate registration rejection         | First provider wins                        |
| Frozen file drift             | `router.lock` + host tooling             | Development integrity control              |
| Async boot deadlock           | Host-supplied `ctx` timeout/cancellation | Operational startup safeguard              |

## What the Router Does NOT Protect Against

The router does **NOT** protect against:

- Malicious code already inside the host project's allowed package tree
- Runtime security attacks (it's a development constraint system)
- Runtime `PATH` injection, command injection, or unsafe process execution
- Business logic errors
- Network security issues

## Compile-Time Boundary vs Runtime Execution

The router's `internal/` placement is a **compile-time trust boundary**.
It ensures that only code inside the owning module tree can import the router
internals directly. A third-party dependency outside that boundary cannot
silently import `internal/router` and start using ports without the host
application explicitly wiring them in.

That protection does **not** extend to runtime process execution.
If a host application or adapter shells out to external binaries, the router
does not defend against:

- Untrusted `PATH` resolution
- Executing a malicious binary found earlier in `PATH`
- Shell injection from concatenated command strings

Those risks belong to the host application and any adapter that launches
processes. The router stays intentionally minimal and opt-in, so runtime
execution policy must be chosen explicitly by the host.

Recommended host-side defaults for process execution:

- Prefer absolute executable paths for security-sensitive commands
- Use direct process execution APIs instead of invoking a shell
- Resolve and validate tool paths during trusted startup, then reuse them
- Treat `PATH` lookup as an explicit relaxed mode, not the default

## AI Guardrails (Development Constraints)

The router provides development workflow constraints specifically designed to guide AI agents and prevent common mistakes:

### Frozen / Mutable Split

**Files that should NEVER be edited directly:**

- `internal/router/extension.go` - Extension interfaces and loading logic
- `internal/router/registry.go` - Atomic publication and resolution

**Files designed for host project modification:**

- `internal/router/ports.go` - Port constants
- `internal/router/registry_imports.go` - Port validation
- `internal/router/ext/extensions.go` - Required application extensions
- `internal/router/ext/optional_extensions.go` - Optional capability extensions

**Why this matters:**
- Wrong path: Edit frozen files (harder, requires understanding consequences)
- Right path: Edit mutable files (easier, intended workflow)

### router.lock Checksums

The `router.lock` file tracks SHA256 checksums of frozen files:

```json
{"file":"internal/router/extension.go","checksum":"f2d4e4c7468cbff6a1d9a8cfbffc546a827f6c2643ddafee4123635594eac897"}
{"file":"internal/router/registry.go","checksum":"81b056144d678058f61840f46be36a279b011fde2c6893205e6be1e407a89712"}
```

**Verification flow:**

```bash
# Verify checksums
go run ./internal/router/tools/wrlk lock verify
```

If checksums don't match:
- Automatic CI/CD failure (if integrated)
- Clear signal that frozen files were modified
- Forces intentional decision to update lock

### Narrow Wiring Surface

The mutable wiring surface is intentionally narrow:

- `ports.go` should contain port declarations only
- `registry_imports.go` should contain the whitelist and registry declaration only
- `ext/optional_extensions.go` should remain wiring-focused
- `ext/extensions.go` is the required application boot composition layer and should contain only explicit app-owned wiring

The main wiring files are designed to stay simple and auditable, typically containing:
- Constant declarations
- Slice initialization
- Import statements

This makes it harder to hide business logic inside the router declaration surface.

### Explicit Error Catalog

All router errors include clear guidance on where to fix the problem:

```go
// From error_surface.go
const dependencyOrderViolationGuidance = "If this port is registered in extensions.go or " +
    "optional_extensions.go, the initialization order is wrong. " +
    "Move the providing extension higher up in the correct extensions slice."
```

### Typed PortName

The `PortName` type prevents runtime surprises from typos:

```go
// COMPILER ERROR - can't pass raw string
provider, _ := router.RouterResolveProvider("primary")

// CORRECT - use constant
provider, _ := router.RouterResolveProvider(router.PortPrimary)
```

## Host Tooling Integration

Host projects can add additional enforcement through:

1. **Pre-commit hooks** - Verify `router.lock` before committing
2. **CI/CD gates** - Block merges with checksum mismatches
3. **Editor integrations** - Warn when editing frozen files

### Recommended CI/CD Integration

```yaml
# .github/workflows/router.yml
- name: Verify router lock
  run: go run ./internal/router/tools/wrlk lock verify
```

## Security Properties Summary

| Property              | Mechanism                        | Guarantee               |
| --------------------- | -------------------------------- | ----------------------- |
| Port uniqueness       | Duplicate registration rejection | No accidental shadowing |
| Immutable state       | Atomic publication               | No post-boot mutation   |
| Compile-time safety   | Typed PortName                   | No runtime typos        |
| Frozen file integrity | router.lock checksums            | No silent modifications |
| Boot timeout          | Host context                     | No infinite waits       |

## Port Access Control

The router supports restricted ports that validate consumer identity:

```go
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
    // Register the provider
    reg.RouterRegisterProvider(router.PortConfig, &ConfigProvider{})

    // Restrict access to specific consumers
    reg.RouterRegisterPortRestriction(router.PortConfig, []string{
        "admin-service",
        "config-service",
    })

    // Or allow all
    // reg.RouterRegisterPortRestriction(router.PortConfig, []string{"Any"})

    return nil
}
```

Consumers must use `RouterResolveRestrictedPort` with their identity:

```go
provider, err := router.RouterResolveRestrictedPort(
    router.PortConfig,
    "admin-service",  // Consumer's identity
)
```

## Best Practices

1. **Always verify lock before commits:**
   ```bash
   go run ./internal/router/tools/wrlk lock verify
   ```

2. **Use CLI to add ports:**
   ```bash
   go run ./internal/router/tools/wrlk add --name PortFoo --value foo
   ```

3. **Don't edit frozen files** - If you think you need to, reconsider the approach

4. **Use context with timeout** - Never pass `context.Background()` to boot:
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ext.RouterBootExtensions(ctx)
   ```

5. **Cast to interfaces** - Don't use `any` in business logic:
   ```go
   // WRONG
   result := provider.(type)

   // CORRECT
   primary := provider.(ports.PrimaryProvider)
   ```

## Related Documents

- [Architecture](architecture.md) - Folder structure and frozen/mutable split
- [Concepts](concepts.md) - Core concepts and extension interfaces
- [CLI Tools](cli-tools.md) - Lock verification commands
- [Troubleshooting](troubleshooting.md) - Common error handling
