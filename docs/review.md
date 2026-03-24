# Codebase Review

## 1. Architectural Structure & Design
The codebase implements a custom dependency injection router with an extension-based boot mechanism (`internal/router`). This architectural structure heavily isolates the core router mechanism from external or internal implementation details.
* **Pros**:
  - Zero-dependency routing in the core layer (`internal/router`).
  - Separation of required and optional extensions.
  - O(1) concurrent atomic-pointer lock-free reads for fast port resolution.
  - Strict cycle detection and dependency ordering during boot.
* **Cons/Deviations**:
  - The rigid "lock file" anti-tampering pattern forces developers to rely exclusively on the custom `wrlk` tool to add ports. If the `wrlk` tool is broken (which it currently is), development grinds to a halt.

## 2. CLI Tooling Errors (`wrlk`)
The `wrlk` CLI tool built in `internal/router/tools/wrlk` is currently broken and prevents compilation and testing.
* **Compilation Error**: `internal/router/tools/wrlk/main.go:112:10: undefined: RouterRunExtCommand`
* **Root Cause**: `main.go` has a case for `"ext"` in `RouterDispatchCLICommand` mapping to `RouterRunExtCommand(options, args[1:], stdout, stderr)`, but the actual implementation of this command is missing from the directory (`internal/router/tools/wrlk/`).
* **Impact**: Running `go test ./internal/tests/...` entirely fails for the `wrlk` tests due to this compilation error. Building the tool also fails.
* **Recommendation**: Implement `RouterRunExtCommand` in a new file (e.g., `ext.go`) or temporarily remove the `"ext"` case and the reference to it from `main.go` to unblock the build.

## 3. Testing Structure
Testing files are mostly well separated in `internal/tests/router/` instead of polluting the source tree. This mirrors the production layout correctly and follows the `AGENTS.md` guidelines.
However, because of the aforementioned missing function `RouterRunExtCommand`, the tests fail out-of-the-box. Tests in `main_test.go` and `portgen_test.go` cannot run due to the compilation failure.

## 4. Adherence to `AGENTS.md` Conventions
* **Language & Setup**: The repository targets Go 1.24+ (currently 1.25.4 in `go.mod`), which complies.
* **CLI Frameworks**: The tooling strictly avoids third-party packages like `Cobra` or `Kong`, relying on the standard `flag` library, adhering closely to the "No frameworks" rule.
* **File Pointers / Missing Files**:
  - `AGENTS.md` points to `cmd/wrlk/main.go`, but this directory doesn't seem to exist. The actual `main.go` for the CLI is located at `internal/router/tools/wrlk/main.go`.
  - `docs/database/wrlk_schema.sql` and `docs/database/project_shared.sql` are referenced as key conventions but they don't seem to exist in the `docs` directory yet.
  - Adapter files `internal/adapters/` and `scripts/scanner.py` mentioned in the docs also do not exist.
  - `internal/db/schema.go:EnsureSchema` mentioned in `AGENTS.md` does not exist.

## 5. Security & Policies
* The application properly handles environment profiles (`WRLK_ENV`, `ROUTER_PROFILE`) and fails early if boot policies mismatched (e.g. `ROUTER_ALLOW_ANY=true` on `prod` is blocked).
* The anti-tamper `router.lock` forces procedural updates, which might act as an internal security control against accidental changes, albeit at a high operational cost.

## Conclusion
The repository has a solid and very strictly defined architectural pattern for internal dependency injection. The primary and immediate concern is that the repository is in an uncompilable state due to missing extension scaffolding logic (`RouterRunExtCommand`). Documentation in `AGENTS.md` mentions several directories and files that do not currently exist in the repository, possibly indicating that the project is in an early structural phase or the documentation is out of sync.
