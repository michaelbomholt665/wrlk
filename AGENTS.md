# AGENTS.md

Guidance for AI agents working in this repository.

## Read Order

Read these before making changes:

1. `AGENTS.md`
2. `.codex/rules/general.md`
3. The language-specific guide for the code you will edit:
   - Go: `.codex/rules/go.md`
   - Python: `.codex/rules/python.md`
   - TypeScript: `.codex/rules/typescript.md`
4. If using subagents: `.codex/subagent-summary-evaluation.md`

## Tooling

Required toolchain:

- Go: `1.24+`
- Python: `3.12+`
- Node: `20 LTS+`
- `golangci-lint`: latest
- `gofumpt`: latest
- `ruff`: latest

Formatting and checks:

- Go: `gofumpt -l -w .`
- Python: `ruff check scripts/` and `ruff format scripts/`
- TypeScript: `tsc --noEmit` and `npm run build:scanner`

Build and test:

```powershell
make build
make lint
go test ./internal/tests/... -v -count=1
python scripts/scanner_test.py -v
```

Policy:

```powershell
go run ./internal/router/tools/wrlk --help
go run ./internal/router/tools/wrlk
```

Run `go run ./internal/router/tools/wrlk` 1-3 times during implementation and always before completion.

## Non-Negotiables

- DON'T: swallow errors silently
       if err != nil { return nil }          ← banned
       if err != nil { _ = err }             ← banned
       _ = someFunc()                        ← banned

- DO:    always propagate or log, even minimally
       if err != nil { return err }          ← fine
       if err != nil { return nil, err }     ← fine
       if err != nil { return fmt.Errorf("checkGoVersion: %w", err) }  ← ideal

Do:

- Use `flag.NewFlagSet(name, flag.ContinueOnError)` for every subcommand.
- Add `--config` to every new `Run*` function with default `"test-config.toml"`.
- Wrap returned errors with context: `fmt.Errorf("context: %w", err)`.
- Defer cleanup for DB handles, rows, and statements.
- Keep package-level constants in `UPPER_CASE` and compile reused regexes once at package scope.
- Write Google-style doc comments for exported symbols.
- Write `doc.go` for production packages with `// Package <name> ...` and a `Package Concerns:` section.
- Use `log.Printf` inside goroutines.
- Keep function cognitive complexity at or below `15`.
- Name functions with at least 2 tokens such as `RunBackup`.
- Use `observationVersion` from `internal/adapters/types.go`, not a raw version string.

Do not:

- Use `gofmt`. Use `gofumpt`.
- Use CLI frameworks such as Cobra, Kong, or `urfave/cli`.
- Add global config state or singleton config caches.
- Hardcode config paths. Use `fs.String("config", "test-config.toml", ...)`.
- Create `schema_id` values at runtime without matching TOML declarations.
- Write tests under `internal/adapters/`. Go tests belong in `internal/tests/`.
- Write directly to `os.Stdout` in adapters. Use `fmt.Fprintf(os.Stdout, ...)`.
- Use `bash` as the executor for scanner scripts.
- Import third-party packages in `scripts/scanner.py`.
- Touch `docs/Initial/` unless explicitly instructed.

## Key Conventions

CLI:

- No frameworks.
- Every subcommand follows `func(args []string) error`.
- Command registration lives in `internal/app/run.go`.

Config:

- `test-config.toml` is loaded fresh on each command.
- `[registry.contracts]` is the only authoritative source for contract identities.
- `[registry.domains]` must be non-empty if any contracts are declared.

Observation records:

- All scanners emit one JSON object per line to stdout.
- Use the existing observation shape and reuse shared constants.

Important target reference shapes:

- Symbol: `<lang>:<file_path>:<qualified_name>`
- File: `file:<path>`
- Table: `table:<name>`
- Route: `route:<method>:<path>`

Example:

```text
go:internal/db/queries.go:db.GetUserByID
```

Database:

- SQLite only through Phase 2 using `modernc.org/sqlite`.
- Tests use `file::memory:?cache=shared`.
- Schema is applied by `internal/db/schema.go:EnsureSchema`.
- Do not add a third-party migration library.

Testing:

- Go tests mirror the production layout under `internal/tests/`.
- Prefer table-driven tests and `testify/assert` plus `require`.

Scanners:

- Go scanning is built in with `go/ast` and `go/parser`.
- Python scanning uses `scripts/scanner.py` with stdlib `ast` only.
- TypeScript scanning uses `scripts/scanner.ts` and embedded `dist/scanner.cjs`.

## File Pointers

- CLI entry: `cmd/wrlk/main.go`
- Dispatch table: `internal/app/run.go`
- Config structs: `internal/config/config.go`
- Schema DDL: `docs/database/wrlk_schema.sql`
- Shared schema DDL: `docs/database/project_shared.sql`
- All Go tests: `internal/tests/`
- Staged observations: `.wrlk/staging/staged.<lang>.ndjson`
- Exports: `.wrlk/exports/<filter>_<timestamp>.tsv`
- Embedded scripts: `.wrlk/scripts/`
- Design docs: `docs/Design/01-05_*.md`
- Codebase report: `docs/reports/codebase_report.md`
