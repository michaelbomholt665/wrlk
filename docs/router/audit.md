# Router Codebase Audit

**Scope:** `internal/router/`, `internal/ext/`, `internal/router/tools/wrlk/`  
**Tests:** TDD/E2E suite not included but considered implicitly.

---

## CRITICAL

### 1. `RouterInjectValidationCase` generates non-compiling code — `portgen.go`

**File:** `portgen.go`  
**Function:** `RouterInjectValidationCase`

The injection regex matches `switch port {` and inserts `\n\tcase <Name>:` immediately after it, with no body:

```go
newCase := fmt.Sprintf("\n\tcase %s:", name)
```

The resulting code in `registry_imports.go` after `wrlk add --name PortFoo --value foo`:

```go
func RouterValidatePortName(port PortName) bool {
    switch port {
    case PortFoo:                                          // ← injected, empty body
    case PortConfig, PortWalk, PortScanner, PortTelemetry:
        return true
    default:
        return false
    }
}
```

In Go, `case PortFoo:` with an empty body does **not** fall through. If `port == PortFoo`, the switch exits cleanly, and the function ends with no `return` statement. This is a **compile error** — Go requires all code paths to return in a `bool`-returning function. Every call to `wrlk add` produces a module that does not build.

**Fix:** The injected case must carry a body, or the new name must be appended to the existing case list. The minimal correct injection is one of:

```go
// Option A — append to existing comma list (minimal diff):
newCase := fmt.Sprintf(", %s", name)
// inject at the position of the first `:` in the switch body

// Option B — standalone case with return:
newCase := fmt.Sprintf("\n\tcase %s:\n\t\treturn true", name)
```

Option A is safer because it doesn't change the switch structure. Option B is cleaner if multi-line cases are ever needed.

---

### 2. `RouterBuildDependencyGraph` invokes `RouterProvideRegistration` with a dummy registry to infer provided ports — `extension.go`

**File:** `extension.go`  
**Function:** `RouterBuildDependencyGraph`

```go
dummyPorts := make(map[PortName]Provider)
dummy := &Registry{ports: &dummyPorts}
_ = ext.RouterProvideRegistration(dummy)
```

This is called during topological sort, which is called from `routerLoadExtensionLayer`, which then calls `RouterProvideRegistration` **again** for real boot. Every extension's registration runs twice. The consequences:

- **Side effects fire twice.** `telemetryExample.RouterProvideRegistration` calls `log.Println("telemetry initialized")` — it will print twice in every boot. Any extension that allocates a resource, opens a connection, starts a goroutine, or logs during registration will do so twice.
- **Errors are silently swallowed.** The return value is `_ =` — if `RouterProvideRegistration` fails on the dummy for any reason, the graph misses that extension's ports, producing a silent incomplete dependency map.
- **The dummy cannot enforce `PortUnknown` cleanly.** The dummy registry does validate port names (via `RouterValidatePortName`), so registration of a bad port on the dummy returns an error that is discarded. The real boot will then catch it — but the graph construction has already silently treated that extension as providing zero ports.

**Root cause:** The `Extension` interface has `Consumes()` but no `Provides()`. The only way to discover what an extension registers is to run it.

**Fix:** Add `Provides() []PortName` to the `Extension` interface. This eliminates the dummy-registry hack entirely and makes the contract explicit. The dependency graph then becomes:

```go
for i, ext := range exts {
    for _, port := range ext.Provides() {
        if _, exists := provides[port]; exists {
            return nil, &RouterError{Code: PortDuplicate, Port: port}
        }
        provides[port] = i
    }
}
```

All existing extensions already know what they provide — the information just isn't surfaced through the interface.

---

## HIGH

### 3. `routerClassifyExtensionError` mutates a shared error in place — `extension.go`

```go
if formattedRouterErr, ok := formattedErr.(*RouterError); ok {
    switch formattedRouterErr.Code {
    case RequiredExtensionFailed, OptionalExtensionFailed:
        formattedRouterErr.Code = code   // ← mutates the error value
        return formattedRouterErr
    }
}
```

If an `ErrorFormattingExtension` returns a `*RouterError` from its formatter and holds a reference to it elsewhere (e.g., for inspection or logging), mutating `.Code` will corrupt the formatter's copy. Error values should be immutable once returned.

**Fix:**

```go
return &RouterError{
    Code: code,
    Port: formattedRouterErr.Port,
    Err:  formattedRouterErr.Err,
}
```

---

### 4. Dead code in `RouterRunLockCommand` — `lock.go`

```go
func RouterRunLockCommand(options globalOptions, args []string, stdout io.Writer) error {
    if len(args) == 0 || RouterIsHelpToken(args[0]) {
        return RouterWriteLockUsage(stdout)
    }

    if len(args) == 0 {  // ← unreachable: already handled above
        return &usageError{message: "missing lock subcommand"}
    }
    ...
```

The second `if len(args) == 0` can never be reached. The first guard covers both `len == 0` and the help token case. The dead branch also has a different error message ("missing lock subcommand") than what a user would actually see (the usage output). This is a minor correctness hazard if the first guard is ever refactored.

**Fix:** Remove the second check entirely.

---

### 5. Duplicate type definitions and duplicated logic between `lock.go` and `portgen.go`

Both files are in `package main`. They define parallel, structurally identical types and functions:

| `lock.go`                            | `portgen.go`                      | Purpose                                              |
| ------------------------------------ | --------------------------------- | ---------------------------------------------------- |
| `lockRecord`                         | `portgenLockRecord`               | `{File, Checksum string}` with identical JSON tags   |
| `RouterChecksumForPath`              | `RouterChecksumPortgenFile`       | `sha256` of a file at a relative path                |
| `RouterComputeLockRecords`           | `RouterComputePortgenLockRecords` | Build sorted checksum slice for `trackedRouterFiles` |
| `RouterWriteTempLockFile`            | `RouterWriteAndCloseTempFile`     | Write+sync+close a `*os.File`                        |
| (inline in `RouterWriteLockRecords`) | `RouterWriteLockAfterPortgen`     | JSON-encode records and atomically write lock file   |

This duplication means a change to the lock file format must be applied in two places. They will inevitably drift.

**Fix:** Consolidate into a single type and a shared set of functions. `portgen.go` should call `RouterComputeLockRecords` and `RouterWriteLockRecords` from `lock.go` instead of re-implementing them. The distinct type `portgenLockRecord` should be deleted and `lockRecord` used throughout.

---

### 6. `RouterResolveRestrictedPort` accepts an empty `consumerID` — `registry.go`

```go
func RouterResolveRestrictedPort(port PortName, consumerID string) (Provider, error) {
    ...
    if !RouterCheckPortConsumerAccess(port, consumerID) {
        return nil, &RouterError{Code: PortAccessDenied, Port: port, ConsumerID: consumerID}
    }
    return provider, nil
}
```

An empty string `consumerID` is silently accepted. If a restriction list ever contains an empty string (accidentally or not), it would match. An empty `consumerID` is almost certainly a call-site bug — no business logic component should have a blank identity.

**Fix:** Return `PortAccessDenied` immediately if `consumerID == ""`.

---

## MEDIUM

### 7. `Extension` interface missing `Provides() []PortName`

This is the structural root of issue #2. The `Consumes()` method declares what an extension needs. The absence of a symmetric `Provides()` method forces the dummy-registry workaround, creates the double-invoke hazard, and makes the extension contract asymmetric. A well-defined extension should be fully self-describing without being executed.

The wrlk `add` tooling also only touches `ports.go` and `registry_imports.go` — it doesn't scaffold the extension struct. A `Provides()` method would give a clear, verifiable answer to "what does this extension supply?" at both graph-build time and in future tooling.

---

### 8. `routerHandleExtensionRegistration` doesn't guard against nil extension — `extension.go`

`routerCheckExtensionDependencies` has an early nil guard:

```go
if ext == nil {
    return nil
}
```

But `routerHandleExtensionRegistration` does not:

```go
func routerHandleExtensionRegistration(registryHandle *Registry, ext Extension, ctx context.Context) ([]error, error) {
    if err := ext.RouterProvideRegistration(registryHandle); err != nil {  // ← panics if ext is nil
```

The nil guard in the dependency check function gives a false sense of safety. If a nil extension ever enters `routerLoadExtensionLayer` (e.g., from a `nil` element in an `[]Extension` slice), the sort step may handle it but the registration step panics.

**Fix:** Add a nil guard at the top of `routerHandleExtensionRegistration`, consistent with the dependency check.

---

### 9. `RouterRecordLiveReport` returns `nil` on participant failure — `live.go`

```go
case "failure":
    s.RouterMarkSessionFailure(...)
    return nil  // ← HTTP handler responds 202 Accepted
```

When a participant POSTs `status: "failure"`, the session is terminated as failed, but the HTTP response is `202 Accepted`. The participant cannot distinguish "my failure report was accepted" from "my success report was accepted." If a participant checks the response code to confirm delivery, this is ambiguous. A `200 OK` with a distinct response body, or a dedicated `status` field in the HTTP response, would make the protocol unambiguous.

This is also inconsistent with the unknown-participant path, which returns a non-nil error and gets a `400 Bad Request`.

---

### 10. Tick-based timeout polling in `RouterWaitForSessionCompletion` — `live.go`

```go
ticker := time.NewTicker(100 * time.Millisecond)
defer ticker.Stop()
for {
    select {
    ...
    case <-ticker.C:
        if s.RouterHasSessionTimedOut(timeout) {
```

Timeout detection has up to 100ms latency. For production use with `--timeout 15s` this is negligible. For tests using short timeouts (e.g., 50ms or 100ms), the poll interval interacts with the timeout duration and can cause flakiness — particularly since `startedAt` is only set on first participant report, meaning the full timeout check path requires a participant to have connected.

**Fix:** Replace the ticker with `time.AfterFunc` or a `time.NewTimer` that fires exactly when the deadline elapses, computed relative to `startedAt`. The timer should be started (or reset) when the first participant connects rather than running from the beginning.

---

### 11. `RouterBuildDependencyGraph` calls `RouterProvideRegistration` on nil-safe path but discards meaningful errors — `extension.go`

The `_ =` discard on the dummy call means that if an extension's `RouterProvideRegistration` returns `PortUnknown` (because it tries to register a port that was removed from `ports.go`), the error is silently lost during graph construction. The sort proceeds as if that extension provides nothing. The real boot then catches the error — but the dependency sort has already placed the extension in a potentially wrong position.

With a `Provides()` method (fix #7), this path is eliminated. Until then, the dummy call should at minimum check for `PortUnknown` and `PortDuplicate` errors and propagate them.

---

### 12. `RouterCheckPortConsumerAccess` relies on map-ok idiom for "is restricted" — `registry.go`

```go
allowed, restricted := published.restrictions[port]
if !restricted {
    return true   // open to all
}
```

The variable `restricted` is the map "ok" bool, repurposed as "is this port restricted." This is idiomatic Go but semantically confusing — a reader must understand that a missing map key means unrestricted, not that the restriction is disabled or default-deny. The variable name conflates "key exists" with "access is restricted." A rename to `isRestricted` and a comment clarifying the open-by-default policy would reduce maintenance risk.

---

## LOW / STYLE

### 13. `routerSnapshot` name collision across packages

`lock.go` (package `main`) defines `routerSnapshot` as a JSON-serializable file record. `registry_imports.go` (package `router`) defines `routerSnapshot` as an atomic pointer payload with providers and restrictions. These are different packages so Go does not complain, but the identical unexported name across the two closely related packages will confuse any developer or agent reading both files in the same context. The CLI type should be renamed to something like `routerFileSnapshot` or `routerLockSnapshot`.

---

### 14. `RouterInjectPortConstant` comment uses `value` as the descriptor

```go
newLine := fmt.Sprintf("\t// %s is the %s provider port.\n\t%s PortName = %q\n", name, value, name, value)
```

For `--name PortFoo --value foo`, this produces: `// PortFoo is the foo provider port.` The comment reads slightly awkwardly because `value` is a routing string, not a description. The existing ports in `ports.go` have manually written, human-readable comments ("PortWalk is the filesystem walk provider port"). `wrlk add` produces technically correct but templated comments. Consider adding an optional `--comment` flag for the human-readable description.

---

## Summary Table

| #   | Severity | File                     | Issue                                                                                    |
| --- | -------- | ------------------------ | ---------------------------------------------------------------------------------------- |
| 1   | CRITICAL | `portgen.go`             | `RouterInjectValidationCase` generates empty-body case — compile error                   |
| 2   | CRITICAL | `extension.go`           | `RouterBuildDependencyGraph` double-invokes `RouterProvideRegistration` via dummy        |
| 3   | HIGH     | `extension.go`           | `routerClassifyExtensionError` mutates `*RouterError` in place                           |
| 4   | HIGH     | `lock.go`                | Unreachable `if len(args) == 0` dead code after identical guard                          |
| 5   | HIGH     | `lock.go` / `portgen.go` | Duplicate type + logic: `lockRecord`, checksum, write functions                          |
| 6   | HIGH     | `registry.go`            | `RouterResolveRestrictedPort` silently accepts empty `consumerID`                        |
| 7   | MEDIUM   | `extension.go`           | `Extension` interface lacks `Provides() []PortName` — structural root of #2              |
| 8   | MEDIUM   | `extension.go`           | `routerHandleExtensionRegistration` missing nil guard present in sibling function        |
| 9   | MEDIUM   | `live.go`                | Participant failure report returns `nil` / 202 Accepted                                  |
| 10  | MEDIUM   | `live.go`                | Tick-based timeout polling has up to 100ms resolution; test-flake risk                   |
| 11  | MEDIUM   | `extension.go`           | Dummy `RouterProvideRegistration` errors discarded, silently corrupting dependency graph |
| 12  | MEDIUM   | `registry.go`            | `restricted` variable name conflates map-ok with access policy semantics                 |
| 13  | LOW      | `lock.go`                | `routerSnapshot` name collides with `registry_imports.go` type across packages           |
| 14  | LOW      | `portgen.go`             | Auto-generated port comment uses routing string, not human description                   |