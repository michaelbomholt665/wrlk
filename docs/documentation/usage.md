# Router Usage

## Boot Once

Call `ext.RouterBootExtensions(ctx)` once during startup.

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

warnings, err := ext.RouterBootExtensions(ctx)
if err != nil {
	log.Fatal(err)
}

for _, warning := range warnings {
	log.Println("router warning:", warning)
}
```

Rules:
- Boot before workers or request handlers start.
- Pass a real context with timeout or cancellation.
- Treat returned warnings as optional-extension failures.
- Boot policy validation also runs here. In this repo, `ROUTER_PROFILE` must match `WRLK_ENV` when both are set, and `ROUTER_ALLOW_ANY=true` is rejected in `prod`.

## Resolve Providers

Resolve by port, then cast to the expected port contract.

```go
provider, err := router.RouterResolveProvider(router.PortPrimary)
if err != nil {
	return nil, fmt.Errorf("resolve primary provider: %w", err)
}

primary, ok := provider.(ports.PrimaryProvider)
if !ok {
	return nil, &router.RouterError{
		Code: router.PortContractMismatch,
		Port: router.PortPrimary,
	}
}
```

Use `RouterResolveRestrictedPort` when the port has access restrictions.

For router-native CLI capabilities, prefer the typed resolvers in `internal/router/capabilities/`:

```go
styler, err := capabilities.ResolveCLIOutputStyler()
chrome, err := capabilities.ResolveCLIChromeStyler()
interactor, err := capabilities.ResolveCLIInteractor()
```

## Copy Into a New Module

If you copied the router bundle from this repository into a different Go module, run:

```bash
go run ./internal/router/tools/wrlk module sync
```

This is a one-time bootstrap step. It rewrites bundled `internal/router` imports from the source module path to the module declared in the local `go.mod`.

## Register a Port

Use the CLI:

```bash
go run ./internal/router/tools/wrlk register --port --router --name PortFoo --value foo
```

That updates:
- `internal/router/router_manifest.go`
- `internal/router/ports.go`
- `internal/router/registry_imports.go`
- `internal/router/router.lock`

`router.lock` verifies the managed router files that are expected to stay in sync, not just the frozen core.

## Register an Extension

Use `wrlk register` against the manifest layer, then let the generated runtime wiring stay in sync.

Optional capability extension:

```bash
go run ./internal/router/tools/wrlk register --ext --router --name telemetry
```

This wires `internal/router/ext/extensions/telemetry/` into `internal/router/ext/optional_extensions.go` and records the declaration in `internal/router/router_manifest.go`.

Wire a required application extension:

```bash
go run ./internal/router/tools/wrlk register --ext --app --name billing
```

This wires `internal/adapters/billing/` into `internal/router/ext/extensions.go` and records the declaration in `internal/router/ext/app_manifest.go`.

`internal/router/ext/extensions.go` may legitimately remain empty when the application has no required adapters to boot there.

`register --ext --router` is for router-owned extensions under `internal/router/ext/extensions/<name>/`, which boot first. `register --ext --app` is for app-owned adapters such as `internal/adapters/<name>/`, which boot second and then rely on declared `Consumes()` edges for ordering within the application layer.

Use `--dry-run` with `wrlk register`, `wrlk ext remove`, or `wrlk ext app remove` to preview changes.

## Layout

- `internal/ports/`: port contracts
- `internal/router/capabilities/`: router-native capability contracts and typed resolvers
- `internal/router/ports.go`: port names
- `internal/router/ext/optional_extensions.go`: optional capability wiring
- `internal/router/ext/extensions.go`: generated required application wiring and boot policy wrapper; may be empty
- `internal/router/registry.go`: provider resolution and restricted resolution

## Rule

Do not import adapters directly from business code. Resolve through the router and cast to the port interface.
