# Router AGENTS Snippet

Place this block at or near the top of the host repository's real `AGENTS.md`.
Keep it short so the router adds minimal context overhead for AI agents.

```md
## Router Rules

- If you edit `internal/router/ports.go`, `internal/router/registry_imports.go`, or `internal/router/ext/*`, read `docs/documentation/` first.
- Do not manually edit `internal/router/extension.go` or `internal/router/registry.go` unless explicitly asked.
- Use `go run ./internal/router/tools/wrlk add --name <PortName> --value <string>` to add router ports.
- Use `go run ./internal/router/tools/wrlk ext add --name <ExtensionName>` to scaffold router extensions.
- Before finishing router-related changes, run `go run ./internal/router/tools/wrlk --help`.
- If router lock or scaffold output does not match the current file shape, stop and report drift instead of forcing edits.
```
