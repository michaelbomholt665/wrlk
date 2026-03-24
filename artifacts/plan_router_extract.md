# Goal
Extract the router into a standalone repository without traces of the old policy-engine/pilot-project. 

## User Review Required
> [!IMPORTANT]
> - `internal/tests/router/tools/wrlk/main_test.go` - The `TestWrlkLockUpdateAndVerify_PilotRouterFixture` and `createPilotFixture` will be permanently deleted because they rely on `internal/adapters` and `internal/ports` that do not exist anymore.
> - `internal/router/ext/extensions.go` - The old `adapterconfig`, `adapterscanners`, and `adapterwalk` will be removed. Built-in extensions will be replaced with minimal structural examples that just wire string-named structs. 
> - `POLICYCHECK_ENV` will be renamed to `WRLK_ENV` in `extensions.go` and its boot tests.
> - ALL imports of `policycheck/...` will be rewritten to `github.com/michaelbomholt665/wrlk/...`.

## Proposed Changes

### Configuration and Tooling
#### [MODIFY] [AGENTS.md](file:///c:/Users/micha/.syntx/go/wire-lock/AGENTS.md)
- Replace references to `cmd/policycheck` and old testing targets with generic `wrlk` commands.

### Core Router & Extensions
#### [MODIFY] [internal/router/ext/extensions.go](file:///c:/Users/micha/.syntx/go/wire-lock/internal/router/ext/extensions.go)
- Remove imports to `policycheck/internal/adapters/...`
- Replace concrete adapter initializations (`adapterconfig.NewConfigProvider()`, etc.) with neutral mock implementations.
- Rename `POLICYCHECK_ENV` to `WRLK_ENV`.

#### [MODIFY] [internal/router/ext/telemetry_example.go](file:///c:/Users/micha/.syntx/go/wire-lock/internal/router/ext/telemetry_example.go)
- Change import `policycheck/internal/router` to `github.com/michaelbomholt665/wrlk/internal/router`.

#### [MODIFY] [internal/router/tools/wrlk/main.go](file:///c:/Users/micha/.syntx/go/wire-lock/internal/router/tools/wrlk/main.go)
- Ensure no traces of `policycheck` in the CLI help outputs.

### Router Tests
#### [MODIFY] internal/tests/router/... (Multiple files)
- For every file in `internal/tests/router` (e.g. `helpers_test.go`, `boot_test.go`, `restricted_test.go`, `benchmark_test.go`, `registration_test.go`, `resolution_test.go`, `ext_boot_test.go`):
  - Change import `policycheck/internal/router` to `github.com/michaelbomholt665/wrlk/internal/router`.

#### [MODIFY] [internal/tests/router/ext_boot_test.go](file:///c:/Users/micha/.syntx/go/wire-lock/internal/tests/router/ext_boot_test.go)
- Change `Setenv("POLICYCHECK_ENV")` to `Setenv("WRLK_ENV")`.

#### [MODIFY] [internal/tests/router/tools/wrlk/main_test.go](file:///c:/Users/micha/.syntx/go/wire-lock/internal/tests/router/tools/wrlk/main_test.go)
- Delete `TestWrlkLockUpdateAndVerify_PilotRouterFixture` since it explicitly requires `internal/adapters`.
- Delete `createPilotFixture` helper function.

## Verification Plan

### Automated Tests
```powershell
go test ./internal/tests/router/... -v -count=1
gofumpt -l -w .
```
We will verify that all tests pass without errors concerning missing packages. We will also run a grep to verify that no `policy` or `pilot` wording remains.
