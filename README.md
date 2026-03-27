# Router Package

`internal/router` is a small port registry and extension boot layer for Go applications. It gives the application one explicit place to register providers behind typed port names, boot extensions in dependency order, and publish one immutable registry snapshot that consumers can resolve from without runtime wiring logic scattered across the codebase.

The package supports two extension categories. Optional capability extensions add router-native infrastructure such as CLI styling and interaction, while required application extensions wire the concrete adapters your application actually runs behind its declared ports. The package is intentionally split into a frozen core and manifest-backed wiring layers: `internal/router/router_manifest.go` owns router-native port and optional extension declarations, `internal/router/ext/app_manifest.go` owns required application extension declarations, and `wrlk register` regenerates the runtime files from those manifests. A local `wrlk` tool and `router.lock` keep the managed router surface explicit, reviewable, and hard to drift by accident.

## Architecture

```mermaid
---
config:
  layout: dagre
  look: classic
---
flowchart LR
    subgraph Business["Business Logic"]
        UseCase["internal/<domain>/usecase\nbusiness services"]
        CLI["cmd/... or internal/app\nentrypoints"]
    end

    subgraph Contracts["Contracts"]
        AppPorts["internal/ports\napplication port interfaces"]
        CapPkg["internal/router/capabilities\nrouter-native capability interfaces"]
    end

    subgraph RouterMutable["Router Wiring (mutable)"]
        Ports["internal/router/router_manifest.go -> ports.go\nport whitelist"]
        Imports["internal/router/router_manifest.go -> registry_imports.go\nport validation + snapshot state"]
        OptExt["internal/router/router_manifest.go -> optional_extensions.go\noptional capability wiring"]
        ReqExt["internal/router/ext/app_manifest.go -> extensions.go\nrequired app wiring + boot policy"]
    end

    subgraph RouterCore["Router Core (frozen)"]
        Extension["internal/router/extension.go\nboot orchestration"]
        Registry["internal/router/registry.go\nresolve + restricted resolve"]
        Errors["internal/router/error_surface.go\nstructured errors"]
        Manifest["internal/router/capabilities.go\ncapability manifest"]
    end

    subgraph OptionalExts["Optional Capability Extensions"]
        Pretty["prettystyle\nowns PortCLIStyle"]
        Charm["charmcli\nowns PortCLIChrome + PortCLIInteraction"]
    end

    subgraph Adapters["Application Adapters"]
        Adapter["internal/adapters/<name>\nrequired app adapter"]
    end

    CLI -->|"boots once"| ReqExt
    ReqExt -->|"calls"| Extension
    ReqExt -->|"reads env policy"| Extension
    OptExt -->|"supplies optional bundle"| Extension
    ReqExt -->|"supplies required bundle"| Extension
    Ports -->|"declares valid PortName values"| Extension
    Imports -->|"RouterValidatePortName + atomic snapshot"| Extension

    Extension -->|"publishes snapshot"| Registry
    Extension -->|"loads first"| Pretty
    Extension -->|"loads first"| Charm
    Extension -->|"loads second"| Adapter

    Pretty -->|"registers provider by port"| Extension
    Charm -->|"registers provider by port"| Extension
    Adapter -->|"registers provider by port"| Extension

    Pretty -->|"implements"| CapPkg
    Charm -->|"implements"| CapPkg
    Adapter -->|"implements"| AppPorts

    UseCase -->|"may import and use"| AppPorts
    UseCase -->|"may import typed capability resolvers"| CapPkg
    CLI -->|"may resolve providers"| Registry
    UseCase -->|"may resolve by port, then cast"| Registry

    UseCase -. "ACCEPTED: import internal/ports" .-> AppPorts
    UseCase -. "ACCEPTED: import internal/router/capabilities" .-> CapPkg
    UseCase -. "ACCEPTED: import internal/router for resolution only" .-> Registry

    UseCase ==> |"ILLEGAL: do not import concrete adapters"| Adapter
    UseCase ==> |"ILLEGAL: do not import extension packages"| Pretty
    UseCase ==> |"ILLEGAL: do not import extension packages"| Charm
    Adapter ==> |"ILLEGAL: adapters must not import adapters"| Adapter
    AppPorts ==> |"ILLEGAL: ports cannot import implementations"| Adapter
    CapPkg ==> |"ILLEGAL: capability contracts do not import extensions"| Pretty
    RouterCore ==> |"ILLEGAL: router core stays blind to adapters"| Adapter
    RouterCore ==> |"ILLEGAL: router core stays blind to business logic"| UseCase

    classDef business fill:#f4f1de,stroke:#7a5c3e,color:#3d2f1f,stroke-width:2px
    classDef contracts fill:#dff3e3,stroke:#2d6a4f,color:#1b4332,stroke-width:2px
    classDef mutable fill:#fff3bf,stroke:#b08900,color:#6b4f00,stroke-width:2px
    classDef core fill:#dceeff,stroke:#1d4ed8,color:#123b7a,stroke-width:2px
    classDef ext fill:#ffe5d9,stroke:#c2410c,color:#7c2d12,stroke-width:2px
    classDef adapters fill:#fce7f3,stroke:#be185d,color:#831843,stroke-width:2px

    class UseCase,CLI business
    class AppPorts,CapPkg contracts
    class Ports,Imports,OptExt,ReqExt mutable
    class Extension,Registry,Errors,Manifest core
    class Pretty,Charm ext
    class Adapter adapters

    linkStyle 17,18,19 stroke:#2d6a4f,stroke-width:2px,stroke-dasharray: 4 4
    linkStyle 20,21,22,23,24,25,26 stroke:#dc2626,stroke-width:3px,stroke-dasharray: 6 6

```

## What It Does

- Registers providers behind typed `PortName` values
- Boots optional extensions before required application extensions
- Orders extension startup from declared `Consumes()` and `Provides()` dependencies
- Publishes one immutable snapshot for lock-free provider resolution
- Supports router-native CLI output and interaction capabilities through semantic contracts
- Supports boot-time warnings, fatal failures, rollback hooks, restricted port access, and boot-policy validation
- Protects the router kernel with `router.lock` and the `wrlk` scaffolding/verification workflow

## Key Files

- `internal/router/extension.go`: core boot contracts and orchestration
- `internal/router/registry.go`: provider resolution and restricted resolution
- `internal/router/error_surface.go`: router error rendering
- `internal/router/capabilities.go`: declared capability manifest
- `internal/router/router_manifest.go`: router-native port and optional extension declarations
- `internal/router/ext/app_manifest.go`: required application extension declarations
- `internal/router/ext/optional_extensions.go`: generated optional capability wiring
- `internal/router/ext/extensions.go`: generated required application wiring and boot policy wrapper
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

For router-native CLI capabilities, prefer the typed resolvers in `internal/router/capabilities/`:

```go
styler, err := capabilities.ResolveCLIOutputStyler()
chrome, err := capabilities.ResolveCLIChromeStyler()
interactor, err := capabilities.ResolveCLIInteractor()
```

## CLI

```bash
go run ./internal/router/tools/wrlk register --port --router --name PortFoo --value foo
go run ./internal/router/tools/wrlk register --ext --router --name telemetry
go run ./internal/router/tools/wrlk register --ext --app --name billing
go run ./internal/router/tools/wrlk lock verify
go run ./internal/router/tools/wrlk live run --expect scanner-a --expect scanner-b
```

Use:
- `register --port --router` to add a router port in `router_manifest.go`
- `register --ext --router` to wire an optional capability extension in `router_manifest.go`
- `register --ext --app` to wire a required application extension in `app_manifest.go`
- `ext remove` to unwire an optional capability extension from `optional_extensions.go`
- `ext app remove` to unwire a required application extension from `extensions.go`
- `live run` to start a bounded live verification session for local or otherwise trusted-network use

For the CLI capability split:
- `PortCLIStyle` should stay owned by `prettystyle` for output concerns such as text, tables, and semantic layouts.
- `PortCLIChrome` should be owned by `charmcli` for themed text and layout chrome.
- `PortCLIInteraction` should stay owned by `charmcli` for interactive prompt flows.
- The app should resolve these capabilities separately instead of trying to stack multiple providers behind one router port.

## Important Rule

`internal/router/ext/extensions.go` is intentionally generated from `internal/router/ext/app_manifest.go`. It may legitimately be empty when the application has no required adapters to wire there. Do not leave sample or unused providers wired there, and do not treat the generated file as the edit surface.

`wrlk live run` is not suitable for internet exposure as-is. Before exposing it remotely, add authenticated session tokens, bounded request sizes, explicit server timeouts, and rate limiting.

Business logic should import `internal/ports` and, when needed, `internal/router/capabilities` or `internal/router` for resolution. It should not import concrete adapters or concrete extension packages.

## Optional Dependencies

`testify` is used by the repository test suite. Other third-party dependencies, such as renderer libraries used by optional extensions, are only needed when you choose to keep and build those extensions.

If you do not use an optional extension, its dependency is not part of the required application contract. The router core itself remains intentionally small and does not require extension-specific libraries unless you wire and ship that extension.

## Docs

- [Usage](docs/documentation/usage.md)
- [Extension Authoring](docs/documentation/extensions.md)
- [CLI Tools](docs/documentation/cli-tools.md)
- [Architecture](docs/documentation/architecture.md)
- [Troubleshooting](docs/documentation/troubleshooting.md)

## Example Consumer

- [policycheck](https://github.com/michaelbomholt665/policycheck) is an example repository that uses this router pattern in a real application.
