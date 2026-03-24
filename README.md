# Router Package

Zero-dependency port registry and boot wiring for Go applications.

## What It Does

- Registers providers by typed port name
- Boots optional extensions before required application extensions
- Publishes one immutable snapshot for lock-free reads
- Keeps router core changes explicit with `router.lock`

## Key Files

- `internal/router/extension.go`: core boot contracts and orchestration
- `internal/router/registry.go`: provider resolution
- `internal/router/ext/optional_extensions.go`: optional capability wiring
- `internal/router/ext/extensions.go`: required application wiring
- `internal/router/tools/wrlk`: port and extension scaffolding, lock commands

## Basic Use

Boot once:

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

Resolve later:

```go
provider, err := router.RouterResolveProvider(router.PortPrimary)
if err != nil {
	return err
}

primary, ok := provider.(ports.PrimaryProvider)
if !ok {
	return &router.RouterError{
		Code: router.PortContractMismatch,
		Port: router.PortPrimary,
	}
}
```

## CLI

```bash
go run ./internal/router/tools/wrlk add --name PortFoo --value foo
go run ./internal/router/tools/wrlk ext add --name telemetry
go run ./internal/router/tools/wrlk ext app add --name billing
go run ./internal/router/tools/wrlk lock verify
```

Use:
- `ext add` for optional capability extensions wired into `optional_extensions.go`
- `ext app add` for required application extensions wired into `extensions.go`

## Important Rule

`internal/router/ext/extensions.go` is intentionally app-owned and starts empty. Do not leave sample or unused providers wired there.

## Docs

- [Usage](docs/documentation/usage.md)
- [CLI Tools](docs/documentation/cli-tools.md)
- [Architecture](docs/documentation/architecture.md)
- [Troubleshooting](docs/documentation/troubleshooting.md)
