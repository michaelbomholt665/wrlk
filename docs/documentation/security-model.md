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
- Business logic errors
- Network security issues

## AI Guardrails (Development Constraints)

The router provides development workflow constraints specifically designed to guide AI agents and prevent common mistakes:

### Frozen / Mutable Split

**Files that should NEVER be edited directly:**

- `internal/router/extension.go` - Extension interfaces and loading logic
- `internal/router/registry.go` - Atomic publication and resolution

**Files designed for host project modification:**

- `internal/router/ports.go` - Port constants
- `internal/router/registry_imports.go` - Port validation
- `internal/router/extensions.go` - Application extensions
- `internal/router/optional_extensions.go` - Optional extensions

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

### Data-Only Wiring Files

The mutable wiring files (`ports.go`, `extensions.go`, `optional_extensions.go`) are designed to contain only:
- Constant declarations
- Slice initialization
- Import statements

This makes it hard to justify adding business logic to layer declaration files.

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