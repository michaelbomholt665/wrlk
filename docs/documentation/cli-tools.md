# Router CLI Tools

Run commands with:

```bash
go run ./internal/router/tools/wrlk <command>
```

## Commands

| Command | Purpose |
| --- | --- |
| `wrlk add` | Add a router port |
| `wrlk ext add` | Scaffold an optional capability extension |
| `wrlk ext install` | Wire an existing optional capability extension |
| `wrlk ext remove` | Remove an optional capability extension wiring |
| `wrlk ext app add` | Wire an existing required application adapter |
| `wrlk ext app remove` | Remove an application adapter wiring |
| `wrlk lock verify` | Verify `router.lock` |
| `wrlk lock update` | Update `router.lock` after intentional core changes |
| `wrlk lock restore` | Restore the previous local snapshot |
| `wrlk live run` | Start a bounded live verification session |
| `wrlk guide` | Print the short operational guide |
| `wrlk guide current` | Print the currently wired ports and extension inventory |
| `wrlk guide extension` | Print the detailed extension authoring guide |

## Common Commands

Add a port:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

Add an optional capability extension:

```bash
go run ./internal/router/tools/wrlk ext add --name telemetry
```

This creates `internal/router/ext/extensions/telemetry/` and wires it into `internal/router/ext/optional_extensions.go`.

Wire an existing optional capability extension:

```bash
go run ./internal/router/tools/wrlk ext install --name telemetry
```

This wires the existing `internal/router/ext/extensions/telemetry/` package into `internal/router/ext/optional_extensions.go`.

Wire an existing required application adapter:

```bash
go run ./internal/router/tools/wrlk ext app add --name billing
```

This wires `internal/adapters/billing` into `internal/router/ext/extensions.go`.

Verify router core integrity:

```bash
go run ./internal/router/tools/wrlk lock verify
```

Restore the last snapshot:

```bash
go run ./internal/router/tools/wrlk lock restore
```

Print the current router inventory:

```bash
go run ./internal/router/tools/wrlk guide current
```

Print the extension authoring guide:

```bash
go run ./internal/router/tools/wrlk guide extension
```

Start a live verification session:

```bash
go run ./internal/router/tools/wrlk live run --expect scanner-a --expect scanner-b
```

## Notes

- `ext add`, `ext install`, `ext remove`, `ext app add`, and `ext app remove` support `--dry-run`.
- `wrlk add` also supports `--dry-run`.
- `extensions.go` is app-owned and should contain only the required extensions you actually want booted.
- `optional_extensions.go` is for non-fatal capability extensions only.
- `lock verify` tracks only `internal/router/extension.go` and `internal/router/registry.go`.
