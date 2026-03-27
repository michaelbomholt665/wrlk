# Router CLI Tools

Run commands with:

```bash
go run ./internal/router/tools/wrlk <command>
```

## Commands

| Command | Purpose |
| --- | --- |
| `wrlk register --port --router` | Add a router port via `router_manifest.go` |
| `wrlk register --ext --router` | Wire an optional capability extension via `router_manifest.go` |
| `wrlk register --ext --app` | Wire a required application extension via `app_manifest.go` |
| `wrlk module sync` | Rewrite copied router imports to the current `go.mod` module path |
| `wrlk ext remove` | Remove an optional capability extension wiring |
| `wrlk ext app remove` | Remove an application adapter wiring |
| `wrlk lock verify` | Verify `router.lock` |
| `wrlk lock update` | Update `router.lock` after intentional core changes |
| `wrlk lock restore` | Restore the previous local snapshot |
| `wrlk live run` | Start a bounded live verification session |
| `wrlk guide` | Print the short operational guide |
| `wrlk guide current` | Print the currently wired ports and extension inventory |
| `wrlk guide extension` | Print the detailed extension authoring guide |

## Common Commands

Sync copied router imports to the current module path:

```bash
go run ./internal/router/tools/wrlk module sync
```

Use this once after copying the router bundle into a different repository or module. It rewrites bundled `internal/router` import paths from the source module to the module declared in `go.mod`.

Add a port:

```bash
go run ./internal/router/tools/wrlk register --port --router --name PortFoo --value foo
```

Add an optional capability extension:

```bash
go run ./internal/router/tools/wrlk register --ext --router --name telemetry
```

This wires `internal/router/ext/extensions/telemetry/` into `internal/router/ext/optional_extensions.go`.

Wire a required application extension:

```bash
go run ./internal/router/tools/wrlk register --ext --app --name billing
```

This wires `internal/adapters/billing/` into `internal/router/ext/extensions.go`.

Historical migration note: earlier router refactors used `wrlk ext add`, `wrlk ext install`, and `wrlk ext app add`. The preferred edit surface is now `wrlk register`, with manifests as the source of truth.

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

- `wrlk register`, `wrlk ext remove`, and `wrlk ext app remove` support `--dry-run`.
- `wrlk module sync` is a one-time bootstrap step for copied router bundles, not part of normal day-to-day router edits.
- `internal/router/ext/app_manifest.go` is the source of truth for required application extension wiring; `extensions.go` remains generated runtime output.
- `optional_extensions.go` is for non-fatal capability extensions only.
- `lock verify` tracks only `internal/router/extension.go` and `internal/router/registry.go`.
