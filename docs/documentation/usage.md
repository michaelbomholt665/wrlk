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

## Add a Port

Use the CLI:

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
```

That updates:
- `internal/router/ports.go`
- `internal/router/registry_imports.go`
- `internal/router/router.lock`

## Add an Extension

There are separate scaffold and wiring commands.

Optional capability extension:

```bash
go run ./internal/router/tools/wrlk ext add --name telemetry
```

This creates `internal/router/ext/extensions/telemetry/` and wires it into `internal/router/ext/optional_extensions.go`.

Wire an existing optional capability extension:

```bash
go run ./internal/router/tools/wrlk ext install --name telemetry
```

This wires the existing `internal/router/ext/extensions/telemetry/` package into `internal/router/ext/optional_extensions.go`.

Required application extension:

```bash
go run ./internal/router/tools/wrlk ext app add --name billing
```

This wires `internal/adapters/billing/` into `internal/router/ext/extensions.go`.

Use `--dry-run` with `ext add`, `ext install`, `ext remove`, `ext app add`, or `ext app remove` to preview changes.

## Layout

- `internal/ports/`: port contracts
- `internal/router/ports.go`: port names
- `internal/router/ext/optional_extensions.go`: optional capability wiring
- `internal/router/ext/extensions.go`: required application wiring
- `internal/router/registry.go`: provider resolution

## Rule

Do not import adapters directly from business code. Resolve through the router and cast to the port interface.
