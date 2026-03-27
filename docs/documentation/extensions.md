# Extension Authoring Guide

This guide shows how to build router extensions that are actually usable by the application, not just registered in the router.

The repository already includes two optional capability extensions:
- `prettystyle`, which owns `PortCLIStyle`
- `charmcli`, which owns `PortCLIChrome` and `PortCLIInteraction`

The worked example below uses `prettystyle`.

## Choose the Right Extension Type

Use an **optional capability extension** when:

- the feature is cross-cutting infrastructure
- boot should continue if the feature fails
- application extensions or runtime code may consume it when present

For this category:

- the extension package lives in `internal/router/ext/extensions/<name>/`
- the declaration lives in `internal/router/router_manifest.go`
- the generated runtime wiring lands in `internal/router/ext/optional_extensions.go`
- `Required()` returns `false`

Use a **required application extension** only when the app cannot boot without it. Those declarations live in `internal/router/ext/app_manifest.go` and generate into `internal/router/ext/extensions.go` with `wrlk register --ext --app`, while the package itself stays app-owned under `internal/adapters/<name>/`. Router-owned extensions boot first; required application adapters boot second and then rely on declared `Consumes()` edges for ordering within that second phase.

## The Pattern That Actually Works

For an extension to be consumable by the app, you need three explicit pieces:

1. A router port in `internal/router/ports.go`
2. A router-native capability interface in `internal/router/capabilities/`
3. A concrete provider registered by the extension

That split matters:

- the **port** is how the router discovers the capability
- the **capability interface** is what the app resolves and calls
- the **provider** is the concrete implementation detail hidden behind the port

Consumers should not import the concrete extension package just to use the capability.

## Worked Example: `go-pretty` CLI Styling

Goal: provide a reusable table-rendering capability to the application through the router.

### 1. Add the Port

Use the router tool instead of editing files by hand:

```bash
go run ./internal/router/tools/wrlk register --port --router --name PortCLIStyle --value cli-style
```

That updates:

- `internal/router/router_manifest.go`
- `internal/router/ports.go`
- `internal/router/registry_imports.go`
- `internal/router/router.lock`

### 2. Add the Capability Contract

Extend `internal/router/capabilities/cli.go`:

```go
package capabilities

const (
	TableKindNormal = "table.normal"
	LayoutKindPanel = "layout.panel"
)

// CLIOutputStyler renders semantic CLI output for consumers.
type CLIOutputStyler interface {
	StyleText(kind string, input string) (string, error)
	StyleTable(kind string, headers []string, rows [][]string) (string, error)
	StyleLayout(kind string, title string, content ...string) (string, error)
}
```

Why this matters:

- the app resolves `capabilities.ResolveCLIOutputStyler()`
- the app does not need to know or care that `go-pretty` is the implementation
- you can swap implementations later without changing consumers
- canonical capability names belong to the router contract, so the extension translates them to renderer-native style names or options

### 3. Scaffold the Optional Extension

```bash
go run ./internal/router/tools/wrlk register --ext --router --name prettystyle
```

This creates:

- `internal/router/ext/extensions/prettystyle/doc.go`
- `internal/router/ext/extensions/prettystyle/extension.go`

and wires the extension into `internal/router/ext/optional_extensions.go` while recording the declaration in `internal/router/router_manifest.go`.

### 4. Add the Concrete Provider

Add the dependency to `go.mod` if the extension needs one:

```bash
go get github.com/jedib0t/go-pretty/v6/table
```

Then implement the provider in the extension package, for example in `internal/router/ext/extensions/prettystyle/provider.go`:

```go
package prettystyle

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/michaelbomholt665/wrlk/internal/router/capabilities"
)

// Provider renders styled CLI text and tables using go-pretty.
type Provider struct{}

// StyleText applies router-owned semantic text roles.
func (p *Provider) StyleText(kind string, input string) (string, error) {
	switch kind {
	case "", capabilities.TextKindPlain:
		return input, nil
	case capabilities.TextKindHeader:
		return text.Colors{text.FgHiMagenta, text.Bold}.Sprint(input), nil
	case capabilities.TextKindInfo:
		return text.Colors{text.FgHiCyan, text.Bold}.Sprint("[ INFO ] ")+text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindSuccess:
		return text.Colors{text.FgHiGreen, text.Bold}.Sprint("[  OK  ] ")+text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindWarning:
		return text.Colors{text.FgHiYellow, text.Bold}.Sprint("[ WARN ] ")+text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindError:
		return text.Colors{text.FgHiRed, text.Bold}.Sprint("[ ERR  ] ")+text.Colors{text.FgHiWhite}.Sprint(input), nil
	default:
		return "", fmt.Errorf("style text: unsupported kind %q", kind)
	}
}

// StyleTable formats headers and rows into a single table string.
func (p *Provider) StyleTable(kind string, headers []string, rows [][]string) (string, error) {
	writer := table.NewWriter()

	headerRow := make(table.Row, 0, len(headers))
	for index, header := range headers {
		styledHeader, err := p.StyleText(capabilities.TextKindHeader, header)
		if err != nil {
			return "", fmt.Errorf("style table header %d: %w", index, err)
		}
		headerRow = append(headerRow, styledHeader)
	}
	writer.AppendHeader(headerRow)
	writer.Style().Format.Header = text.FormatDefault

	switch kind {
	case capabilities.TableKindCompact:
		writer.SetStyle(table.StyleLight)
	case capabilities.TableKindMerged:
		writer.SetStyle(table.StyleRounded)
		writer.SetColumnConfigs([]table.ColumnConfig{{Number: 1, AutoMerge: true}})
	default:
		writer.SetStyle(table.StyleRounded)
	}

	for rowIndex, row := range rows {
		if len(row) != len(headers) {
			return "", fmt.Errorf("style table: row %d width %d does not match header width %d", rowIndex, len(row), len(headers))
		}

		renderRow := make(table.Row, 0, len(row))
		for _, cell := range row {
			renderRow = append(renderRow, cell)
		}
		writer.AppendRow(renderRow)
	}

	return writer.Render(), nil
}
```

The provider should satisfy `capabilities.CLIOutputStyler`.

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
// CLI output provider for router consumers.
//
// Usage:
//   - Depend on this extension when callers want styled CLI text, tables, or
//     simple semantic layouts.
//   - Resolve router.PortCLIStyle through internal/router/capabilities.
//
// Package Concerns:
//   - Required() must remain false because styled output is optional infrastructure.
//   - Provides() must stay aligned with router.PortCLIStyle.
package prettystyle
```

### 7. Consume It from the App

Where the app needs styled CLI output:

```go
styler, err := capabilities.ResolveCLIOutputStyler()
if err != nil {
	// Optional capability: degrade gracefully if unavailable.
	return nil
}

rendered, err := styler.StyleTable(
	capabilities.TableKindNormal,
	[]string{"Name", "Status"},
	[][]string{{"scanner", "ready"}},
)
if err != nil {
	return err
}
```

This is the part that makes the extension usable by the app:

- the app resolves the capability by port
- the app depends on the router capability contract, not the concrete implementation
- failure handling matches the extension category
- canonical capability names stay in the router contract and the extension translates them into renderer-specific patterns

## Prompt Capabilities Follow the Same Pattern

Use a separate router port when the capability is interactive instead of purely visual. In this repository:

- `PortCLIStyle` resolves `capabilities.CLIOutputStyler`
- `PortCLIChrome` resolves `capabilities.CLIChromeStyler`
- `PortCLIInteraction` resolves `capabilities.CLIInteractor`

That split lets one optional extension own output rendering, such as `prettystyle`, while another optional extension owns text/layout chrome and prompts, such as `charmcli` with `huh` and `lipgloss`.

Recommended ownership in this repository:

- keep `PortCLIStyle` owned by `prettystyle`
- keep `PortCLIChrome` owned by `charmcli`
- keep `PortCLIInteraction` owned by `charmcli`
- resolve all three capabilities in the app when you want the full CLI UX

That keeps the app API simple and avoids trying to multiplex two concrete providers behind the same router port.

## Testing the Capability

Check that the extension is wired:

```bash
go run ./internal/router/tools/wrlk guide current
```

Run the router tests:

```bash
go test ./internal/tests/router/... -count=1
go test ./internal/tests/router/tools/wrlk/... -count=1
```

What to expect:

- `guide current` shows `prettystyle (optional)` under optional capability extensions
- router tests verify the extension boots, resolves through `capabilities.ResolveCLIOutputStyler()`, and formats text, tables, and layouts
- if the capability is unavailable at runtime, app code should degrade gracefully because it is optional

For an optional capability extension, consumers should degrade gracefully when the port is unavailable.

## Checklist

Before calling an extension “done”, verify all of this:

- the port was added with `wrlk register --port --router`
- the optional extension package was wired with `wrlk register --ext --router`
- the required application extension was wired with `wrlk register --ext --app`
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
