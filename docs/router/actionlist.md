# Router Audit — Reconciled Action List

Findings reconciled against agent review. Severity labels adjusted where the agent
disagreed with the original audit. Three buckets: **Fix Now**, **Fix Next**, **Defer**.

---

## FIX NOW

### [P0-A] `RouterInjectValidationCase` generates non-compiling code
**File:** `portgen.go` — `RouterInjectValidationCase`  
**Risk:** Every `wrlk add` invocation produces a `registry_imports.go` that fails `go build`.  
**Agent:** Confirmed strongest finding.

Current injection:
```go
newCase := fmt.Sprintf("\n\tcase %s:", name)  // empty body — no return
```

Fix (Option A — append to existing case list, minimal diff):
```go
// Locate `case PortConfig,` and append the new name before the colon.
// Result: case PortConfig, PortWalk, PortScanner, PortTelemetry, PortFoo:
```

Fix (Option B — standalone case with explicit return):
```go
newCase := fmt.Sprintf("\n\tcase %s:\n\t\treturn true", name)
```
Option A is safer structurally. Option B is acceptable if the format is intended to grow
toward one-case-per-constant. Pick one and make the portgen test (`portgen_test.go`)
assert that the resulting file compiles and that `RouterValidatePortName(PortFoo)` returns true.

---

### [P0-B] `RouterBuildDependencyGraph` double-invokes `RouterProvideRegistration`
**File:** `extension.go` — `RouterBuildDependencyGraph`  
**Risk:** Extension side effects fire twice; dummy-run errors silently dropped; dependency
graph can be silently wrong if a registration fails on the dummy.  
**Agent:** Confirmed.

Root cause: `Extension` has `Consumes()` but no `Provides()`. The dummy-registry workaround
is the only way to discover what an extension registers without executing it.

Fix — add `Provides() []PortName` to the `Extension` interface:
```go
type Extension interface {
    Required()    bool
    Consumes()    []PortName
    Provides()    []PortName
    RouterProvideRegistration(reg *Registry) error
}
```

Then `RouterBuildDependencyGraph` becomes:
```go
for i, ext := range exts {
    if ext == nil { continue }
    for _, port := range ext.Provides() {
        if _, exists := provides[port]; exists {
            return nil, &RouterError{Code: PortDuplicate, Port: port}
        }
        provides[port] = i
    }
}
```

All existing extensions trivially implement `Provides()` by returning the port(s) they
pass to `RouterRegisterProvider`. Update `telemetryExample`, `configExtension`,
`walkExtension`, `scannerExtension` accordingly.

---

## FIX NEXT

### [P1-A] `routerClassifyExtensionError` mutates a returned `*RouterError` in place
**File:** `extension.go` — `routerClassifyExtensionError`  
**Agent:** Confirmed valid.

```go
// Current — mutates the formatter's returned error value
formattedRouterErr.Code = code
return formattedRouterErr

// Fix — construct a new value instead
return &RouterError{
    Code: code,
    Port: formattedRouterErr.Port,
    Err:  formattedRouterErr.Err,
}
```

---

### [P1-B] Duplicated checksum and lock-write logic across `lock.go` and `portgen.go`
**File:** both  
**Agent:** Confirmed, noted it has grown worse after snapshot work.

Concrete deduplication:
- Delete `portgenLockRecord` — use `lockRecord` throughout.
- Delete `RouterChecksumPortgenFile` — call `RouterChecksumForPath`.
- Delete `RouterComputePortgenLockRecords` — call `RouterComputeLockRecords`.
- Delete `RouterWriteAndCloseTempFile` — call `RouterWriteTempLockFile`.
- `RouterWriteLockAfterPortgen` can then be reduced to:
  ```go
  func RouterWriteLockAfterPortgen(root string) error {
      records, err := RouterComputeLockRecords(root)
      if err != nil { return fmt.Errorf("compute lock records after portgen: %w", err) }
      return RouterWriteLockRecords(root, records)
  }
  ```

---

### [P2-A] `routerHandleExtensionRegistration` missing nil guard
**File:** `extension.go`  
**Agent:** Confirmed panic path is real.

`routerCheckExtensionDependencies` guards nil; `routerHandleExtensionRegistration` does not.
Add at the top:
```go
if ext == nil {
    return nil, nil
}
```

---

### [P2-B] Empty `consumerID` silently accepted in `RouterResolveRestrictedPort`
**File:** `registry.go`  
**Agent:** Downgraded from HIGH to P2. Agrees it is real but notes that empty IDs are
denied unless a restriction explicitly lists `""` or `"Any"` is used.

```go
func RouterResolveRestrictedPort(port PortName, consumerID string) (Provider, error) {
    if consumerID == "" {
        return nil, &RouterError{Code: PortAccessDenied, Port: port, ConsumerID: consumerID}
    }
    ...
}
```

Validate call sites in tests to ensure no legitimate consumer passes an empty ID.

---

### [P2-C] Live session: participant failure reports 202 Accepted
**File:** `live.go` — `RouterRecordLiveReport`  
**Agent:** Confirmed as legitimate protocol weakness.

```go
case "failure":
    s.RouterMarkSessionFailure(...)
    return nil  // handler writes 202
```

A participant reporting failure gets the same HTTP response as one reporting success.
Options in order of preference:
1. Return a non-nil error with a distinct message so the handler writes `400 Bad Request`
   with a body explaining the failure was recorded.
2. Keep `nil` but write an explicit JSON body `{"recorded":"failure"}` vs
   `{"recorded":"success"}` so participants can distinguish by body rather than status code.

Option 1 is simpler and consistent with the unknown-participant path.

---

### [P2-D] Tick-based timeout polling (100ms resolution)
**File:** `live.go` — `RouterWaitForSessionCompletion`  
**Agent:** Confirmed as real; not broken for production, but a test-flake risk.

Replace ticker-based detection with a one-shot timer started when the first participant
connects. `RouterRecordLiveReport` can reset the timer when `s.startedAt` is first set:

```go
// In RouterWaitForSessionCompletion, instead of a ticker:
timeoutCh := make(chan struct{})
go func() {
    // block until startedAt is set, then sleep for timeout duration
}()
```

Or: expose a `startedCh chan struct{}` from the session so the waiter can do:
```go
select {
case <-s.startedCh:
    timer := time.NewTimer(timeout)
    select {
    case <-timer.C:
        return &verificationBugError{...}
    case <-s.doneCh:
        return s.RouterBuildCompletionError()
    }
case <-s.doneCh:
    return s.RouterBuildCompletionError()
}
```

---

## DEFER

### [D-1] Dead code in `RouterRunLockCommand`
**File:** `lock.go`  
**Agent:** Correct finding, overrated severity. Just cleanup.

Remove the second `if len(args) == 0` block — it is unreachable after the first guard.
No correctness impact; the only risk is if the first guard is refactored without
noticing the second one.

---

### [D-2] `restricted` variable name conflates map-ok with access semantics
**File:** `registry.go`  
**Agent:** Reasonable style feedback, not a defect.

```go
allowed, restricted := published.restrictions[port]
// rename → isRestricted, add comment clarifying open-by-default policy
```

---

### [D-3] `routerSnapshot` name collides across packages
**Files:** `lock.go` (package `main`) / `registry_imports.go` (package `router`)  
**Agent:** Still mildly confusing; low priority given separate package contexts.

Rename the CLI type to `routerFileSnapshot` or `routerLockSnapshot` when touching
`lock.go` for any other reason.

---

### [D-4] Auto-generated port comment uses routing string, not human description
**File:** `portgen.go` — `RouterInjectPortConstant`  
**Agent:** Not raised; original audit finding stands at low priority.

Consider adding an optional `--comment` flag:
```
wrlk add --name PortFoo --value foo --comment "filesystem foo provider"
```
Falls back to `// PortFoo is the foo provider port.` if omitted.

---

## Test Coverage Notes

The agent correctly notes that the original audit understated test coverage.
`main_test.go` and `portgen_test.go` provide meaningful workflow coverage for
add/restore/help flows. Gaps to close as part of the fixes above:

- **P0-A fix:** assert generated `registry_imports.go` compiles and `RouterValidatePortName`
  returns `true` for the newly added port. The current test likely only checks file content,
  not compilation.
- **P0-B fix:** add a test asserting that a multi-extension boot with `Provides()`
  conflicts detects `PortDuplicate` without executing registration twice (verify via
  a counter or log capture).
- **P2-B fix:** add a test calling `RouterResolveRestrictedPort` with `consumerID == ""`.

---

## Execution Order

```
P0-A  →  portgen.go (unblocks wrlk add correctness)
P0-B  →  extension.go + all Extension implementors (structural, do together)
P1-A  →  extension.go (small, isolated)
P1-B  →  lock.go + portgen.go (consolidate types, run lock verify test after)
P2-A  →  extension.go (two lines)
P2-B  →  registry.go (two lines)
P2-C  →  live.go (small protocol fix)
P2-D  →  live.go (timer refactor, update live tests)
D-*   →  opportunistically when touching the relevant files
```