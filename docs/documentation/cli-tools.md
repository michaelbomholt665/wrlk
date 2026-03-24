# Router CLI Tools

The router provides a built-in CLI tool called `wrlk` for managing ports, extensions, and lock verification.

## Running the CLI

```bash
go run ./internal/router/tools/wrlk <command> [flags]
```

## Commands Overview

| Command             | Description                                     |
| ------------------- | ----------------------------------------------- |
| `wrlk lock verify`  | Verify router.lock checksums match frozen files |
| `wrlk lock update`  | Update router.lock after intentional changes    |
| `wrlk lock restore` | Restore previous local snapshot                 |
| `wrlk add`          | Add a new port to the router                    |
| `wrlk ext add`      | Scaffold a new router extension                 |
| `wrlk guide`        | Print operational guide                         |

---

## lock verify

Verifies that the frozen router files match the checksums in `router.lock`.

```bash
go run ./internal/router/tools/wrlk lock verify
```

**Exit Codes:**
- `0` - All checksums match
- `1` - Checksum mismatch detected

**Example:**

```bash
$ go run ./internal/router/tools/wrlk lock verify
router.lock: checksums verified
```

If there's a mismatch:

```bash
$ go run ./internal/router/tools/wrlk lock verify
error: router.lock: checksum mismatch for internal/router/extension.go
  expected: f2d4e4c7468cbff6a1d9a8cfbffc546a827f6c2643ddafee4123635594eac897
  actual:   a1b2c3d4e5f6...
```

**When to use:**
- Before committing changes
- After pulling updates
- In CI/CD pipelines

---

## lock update

Updates the router.lock checksums after you've intentionally modified frozen files.

```bash
go run ./internal/router/tools/wrlk lock update
```

**Warning:** Only run this command when you have intentionally changed the frozen router files and understand the implications.

**Example:**

```bash
$ go run ./internal/router/tools/wrlk lock update
router.lock: checksums updated
```

---

## lock restore

Restores the previous local router snapshot (before last `wrlk add` or `wrlk lock update`).

```bash
go run ./internal/router/tools/wrlk lock restore
```

**Example:**

```bash
$ go run ./internal/router/tools/wrlk lock restore
router.lock: snapshot restored
restored: internal/router/ports.go
restored: internal/router/registry_imports.go
```

---

## add (portgen)

Adds a new port to the router. This is the recommended way to add ports.

```bash
go run ./internal/router/tools/wrlk add --name <PortName> --value <port-value>
```

**Flags:**
- `--name` (required) - The port name constant (PascalCase, e.g., `PortFoo`)
- `--value` (required) - The port string value (lowercase, e.g., `foo`)
- `--dry-run` - Show what would change without making changes

**Example:**

```bash
# Add a new port
go run ./internal/router/tools/wrlk add --name PortFoo --value foo

# Dry run to see what would happen
go run ./internal/router/tools/wrlk add --name PortFoo --value foo --dry-run
```

**What it does:**
1. Adds the constant to `ports.go`
2. Adds the validation case to `registry_imports.go`
3. Writes a local restore snapshot
4. Updates `router.lock`

**Output:**

```bash
$ go run ./internal/router/tools/wrlk add --name PortFoo --value foo
added: PortFoo PortName = "foo" to internal/router/ports.go
added: case PortFoo: return true to internal/router/registry_imports.go
router.lock: checksums updated
```

---

## ext add

Scaffolds a new router extension.

```bash
go run ./internal/router/tools/wrlk ext add --name <ExtensionName>
```

**Flags:**
- `--name` (required) - The extension name (PascalCase, e.g., `Telemetry`)

**Example:**

```bash
go run ./internal/router/tools/wrlk ext add --name Telemetry
```

**What it creates:**

```
internal/router/ext/extensions/telemetry/
├── doc.go           # Package documentation
└── extension.go     # Extension scaffold
```

**What it updates:**
- Creates the extension directory
- Adds the extension to `optional_extensions.go`
- Writes a local restore snapshot

**Generated extension code:**

```go
// Package telemetry is a router capability extension.
package telemetry

import (
    "github.com/michaelbomholt665/wrlk/internal/router"
)

// Extension implements router.Extension.
type Extension struct{}

func (e *Extension) Required() bool {
    return false  // Optional extension
}

func (e *Extension) Consumes() []router.PortName {
    return nil
}

func (e *Extension) Provides() []router.PortName {
    // TODO: Add ports this extension provides
    return nil
}

func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
    // TODO: Register providers
    return nil
}

// Extension returns the extension instance.
func Extension() *Extension {
    return &Extension{}
}
```

---

## guide

Prints a concise operational guide.

```bash
go run ./internal/router/tools/wrlk guide
```

**Output:**

```
Router guide:
  - Use `wrlk add --name <PortName> --value <string>` to add a new router port.
  - `wrlk add` writes a local restore snapshot before mutating router files.
  - Use `wrlk ext add --name <ExtensionName>` to scaffold a new router capability extension.
  - `wrlk ext add` creates internal/router/ext/extensions/<name>/ with doc.go and extension.go.
  - `wrlk ext add` splices the new extension into optional_extensions.go and writes a restore snapshot.
  - Use `wrlk lock verify` to detect drift in checksum-tracked router core files.
  - Use `wrlk lock update` only when intentional router core changes are accepted.
  - Use `wrlk lock restore` to restore the previous local router snapshot.
  - The router core stays contract-blind; business logic must resolve and cast to typed port contracts.
  - `Any` is acceptable only in contract-blind infrastructure or explicit relaxed policy wiring, not business logic.
```

---

## Global Options

| Flag     | Description          | Default |
| -------- | -------------------- | ------- |
| `--root` | Repository root path | `.`     |

Example:

```bash
go run ./internal/router/tools/wrlk --root /path/to/project lock verify
```

---

## Common Workflows

### Add a New Port

```bash
# 1. Verify current state
go run ./internal/router/tools/wrlk lock verify

# 2. Add the port
go run ./internal/router/tools/wrlk add --name PortFoo --value foo

# 3. Verify checksums updated
go run ./internal/router/tools/wrlk lock verify

# 4. If something went wrong, restore
go run ./internal/router/tools/wrlk lock restore
```

### Add a New Extension

```bash
# 1. Scaffold the extension
go run ./internal/router/tools/wrlk ext add --name Logging

# 2. Implement the extension
# Edit internal/router/ext/extensions/logging/extension.go

# 3. Verify
go run ./internal/router/tools/wrlk lock verify
```

---

## Related Documents

- [Index](index.md) - Overview and quick start
- [Architecture](architecture.md) - Folder structure and bootstrap flow
- [Security Model](security-model.md) - router.lock and AI guardrails