# Extension Authoring Guide

This guide shows how to build router extensions that are actually usable by the application, not just registered in the router.

The worked example is an optional capability extension that exposes `go-pretty` table rendering to the rest of the app.

## Choose the Right Extension Type

Use an **optional capability extension** when:

- the feature is cross-cutting infrastructure
- boot should continue if the feature fails
- application extensions or runtime code may consume it when present

For this category:

- the extension package lives in `internal/router/ext/extensions/<name>/`
- the extension is wired in `internal/router/ext/optional_extensions.go`
- `Required()` returns `false`

Use a **required application extension** only when the app cannot boot without it. Those belong in `internal/router/ext/extensions.go`.

## The Pattern That Actually Works

For an extension to be consumable by the app, you need three explicit pieces:

1. A router port in `internal/router/ports.go`
2. An app-facing interface in `internal/ports/`
3. A concrete provider registered by the extension

That split matters:

- the **port** is how the router discovers the capability
- the **interface** is what the app casts to
- the **provider** is the concrete implementation detail hidden behind the port

Consumers should not import the concrete extension package just to use the capability.

## Worked Example: `go-pretty` CLI Styling

Goal: provide a reusable table-rendering capability to the application through the router.

### 1. Add the Port

Use the router tool instead of editing files by hand:

```bash
go run ./internal/router/tools/wrlk add --name PortCLIStyle --value cli-style
```

That updates:

- `internal/router/ports.go`
- `internal/router/registry_imports.go`
- `internal/router/router.lock`

### 2. Add the App Contract

Create `internal/ports/cli_style.go`:

```go
package ports

// CLIStyleProvider renders CLI tables for consumers that want styled output.
type CLIStyleProvider interface {
	RenderTable(headers []string, rows [][]string) (string, error)
}
```

Why this matters:

- the app casts to `ports.CLIStyleProvider`
- the app does not need to know or care that `go-pretty` is the implementation
- you can swap implementations later without changing consumers
- canonical capability names belong to the router contract, so the extension translates them to renderer-native style names or options

If `internal/ports/` does not exist yet in the host project, create it with normal package docs and keep it focused on contracts only.

### 3. Scaffold the Optional Extension

```bash
go run ./internal/router/tools/wrlk ext add --name prettystyle
```

This creates:

- `internal/router/ext/extensions/prettystyle/doc.go`
- `internal/router/ext/extensions/prettystyle/extension.go`

and wires the extension into `internal/router/ext/optional_extensions.go`.

### 4. Add the Concrete Provider

Add the dependency to `go.mod`:

```bash
go get github.com/jedib0t/go-pretty/v6/table
```

Then implement the provider in the extension package, for example in `internal/router/ext/extensions/prettystyle/provider.go`:

```go
package prettystyle

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

// Provider renders styled CLI tables using go-pretty.
type Provider struct{}

// RenderTable formats headers and rows into a single table string.
func (p *Provider) RenderTable(headers []string, rows [][]string) (string, error) {
	writer := table.NewWriter()

	headerRow := make(table.Row, 0, len(headers))
	for _, header := range headers {
		headerRow = append(headerRow, header)
	}
	writer.AppendHeader(headerRow)

	for _, row := range rows {
		if len(row) != len(headers) {
			return "", fmt.Errorf("render table: row width %d does not match header width %d", len(row), len(headers))
		}

		renderRow := make(table.Row, 0, len(row))
		for _, cell := range row {
			renderRow = append(renderRow, cell)
		}
		writer.AppendRow(renderRow)
	}

	return strings.TrimRight(writer.Render(), "\n"), nil
}
```

The provider should satisfy `ports.CLIStyleProvider`.

Important boundary:

- semantic names in `internal/router/capabilities/` are canonical router roles
- extension code converts those roles into the chosen library's established styling patterns
- consumers should not need to know whether the provider uses ANSI, `go-pretty`, or plain text
- if a library uses different style headers or option names, adapt them inside the extension instead of pushing those names into the router contract

### 5. Register the Provider from the Extension

Update `internal/router/ext/extensions/prettystyle/extension.go` so the router advertises the correct port and registers the provider:

```go
package prettystyle

import (
	"fmt"

	"github.com/michaelbomholt665/wrlk/internal/router"
)

// Extension registers the optional CLI styling capability.
type Extension struct{}

func (e *Extension) Required() bool {
	return false
}

func (e *Extension) Consumes() []router.PortName {
	return nil
}

func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortCLIStyle}
}

func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	provider := &Provider{}

	if err := reg.RouterRegisterProvider(router.PortCLIStyle, provider); err != nil {
		return fmt.Errorf("prettystyle extension: %w", err)
	}

	return nil
}
```

Non-negotiable rule:

- `Provides()` must match the port you actually register

If `Provides()` says `PortCLIStyle` but registration writes to another port, the dependency graph becomes misleading and boot behavior will be wrong.

### 6. Document It in `doc.go`

The `wrlk guide current` command now reads extension docs. Use that.

Example:

```go
// Package prettystyle is a router capability extension that registers an optional
// CLI table-rendering provider for application consumers.
//
// Usage:
//   - Depend on this extension when the app wants styled CLI table output without
//     coupling consumers directly to go-pretty.
//   - Resolve router.PortCLIStyle and cast the provider to ports.CLIStyleProvider.
//
// Package Concerns:
//   - Required() must remain false because styled output is optional infrastructure.
//   - Provides() must stay aligned with router.PortCLIStyle.
package prettystyle
```

### 7. Consume It from the App

Where the app needs styled CLI output:

```go
provider, err := router.RouterResolveProvider(router.PortCLIStyle)
if err != nil {
	// Optional capability: degrade gracefully if unavailable.
	return nil
}

styler, ok := provider.(ports.CLIStyleProvider)
if !ok {
	return &router.RouterError{
		Code: router.PortContractMismatch,
		Port: router.PortCLIStyle,
	}
}

rendered, err := styler.RenderTable(
	[]string{"Name", "Status"},
	[][]string{{"scanner", "ready"}},
)
if err != nil {
	return err
}
```

This is the part that makes the extension usable by the app:

- the app resolves the capability by port
- the app depends on the contract, not the implementation
- failure handling matches the extension category
- canonical capability names stay in the router contract and the extension translates them into renderer-specific patterns

For an optional capability extension, consumers should degrade gracefully when the port is unavailable.

## Checklist

Before calling an extension “done”, verify all of this:

- the port was added with `wrlk add`
- the extension was scaffolded with `wrlk ext add`
- `Required()` matches the intended boot policy
- `Provides()` matches the registered port exactly
- `Consumes()` is truthful
- `doc.go` has a usable `Usage:` section
- the app-facing interface lives outside the concrete implementation
- at least one real consumer resolves and uses the port

## Useful Commands

Check what is wired right now:

```bash
go run ./internal/router/tools/wrlk guide current
```

Read the authoring guide from the CLI:

```bash
go run ./internal/router/tools/wrlk guide extension
```

Verify the protected router files:

```bash
go run ./internal/router/tools/wrlk lock verify
```
