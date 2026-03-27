# Router AGENTS Snippet

Place this block at or near the top of the host repository's real `AGENTS.md`.
Keep it short so the router adds minimal context overhead for AI agents.

```md
## Router Rules

- If you edit router files, read `<insert_path_to_router_guide>` first.
- Treat the router as manifest-backed. Do not hand-edit generated router wiring unless explicitly required for consistency.
- Do not manually edit `internal/router/extension.go` or `internal/router/registry.go` unless explicitly asked.
- Use `go run ./internal/router/tools/wrlk module sync` once after copying the router bundle into a different Go module.
- Use `go run ./internal/router/tools/wrlk register --port --router --name <PortName> --value <string>` to add router ports.
- Use `go run ./internal/router/tools/wrlk register --ext --router --name <ExtensionName>` for router-owned extensions under `internal/router/ext/extensions/<name>/`.
- Use `go run ./internal/router/tools/wrlk register --ext --app --name <ExtensionName>` for app-owned adapters such as `internal/adapters/<name>/`.
- Router-owned extensions boot first; app-owned adapters boot second and then rely on declared `Consumes()` dependencies for ordering.
- Before finishing router-related changes, run `go run ./internal/router/tools/wrlk --help`.
- If router lock or scaffold output does not match the current file shape, stop and report drift instead of forcing edits.
```
