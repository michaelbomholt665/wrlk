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
| `wrlk ext app add` | Scaffold a required application extension |
| `wrlk lock verify` | Verify `router.lock` |
| `wrlk lock update` | Update `router.lock` after intentional core changes |
| `wrlk lock restore` | Restore the previous local snapshot |
| `wrlk guide` | Print the short operational guide |

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

Add a required application extension:

```bash
go run ./internal/router/tools/wrlk ext app add --name billing
```

This creates `internal/router/ext/extensions/billing/` and wires it into `internal/router/ext/extensions.go`.

Verify router core integrity:

```bash
go run ./internal/router/tools/wrlk lock verify
```

Restore the last snapshot:

```bash
go run ./internal/router/tools/wrlk lock restore
```

## Notes

- `ext add` and `ext app add` both support `--dry-run`.
- `extensions.go` is app-owned and should contain only the required extensions you actually want booted.
- `optional_extensions.go` is for non-fatal capability extensions only.
