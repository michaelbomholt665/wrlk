# Router Test Strategy

**Version:** 0.8.0
**Status:** Ready for implementation — all open decisions resolved
**Scope:** `internal/router` — TDD test suite using policycheck as pilot project
**Source of truth:** `router-final.md` and Q&A addendum (#1–#16)

---

## 1. Context

The router is a zero-dependency dependency broker for Hexagonal Architecture in Go.
Its value is structural — it either enforces its invariants or it doesn't.
Tests verify contracts, not implementation details.

All tests are **red first**. The router implementation does not exist yet.
Tests are written against `router-final.md` and the resolved Q&A only.
The scrapped design document is not a source of truth. Individual rules or
explanations carried over from it are noted explicitly where used.

This document is written to support a first TDD implementation. The goal is not
just to list tests. The goal is to define the order of work so each red test
forces one small router behavior into existence before the next one is added.

---

## 2. Pilot Project

**policycheck** (`internal/policycheck/`) is the host project used to pilot the router.

Natural ports identified:

| Port constant | Port string | Provided by                  |
| ------------- | ----------- | ---------------------------- |
| `PortConfig`  | `"config"`  | `internal/adapters/config`   |
| `PortWalk`    | `"walk"`    | `internal/adapters/walk`     |
| `PortScanner` | `"scanner"` | `internal/adapters/scanners` |

Port interfaces (`ConfigProvider`, `WalkProvider`, `ScannerProvider`) live in `internal/ports/`.
Each adapter package exposes a struct that implements `router.Extension`.

---

## 3. Resolved Design Decisions

All open decisions from v1.0.0 are now closed.

### 3.1 Provider type

```go
type Provider any
```

The router never calls methods on a `Provider` — it only stores and returns it.
Type assertions happen in adapter code when resolving. `PortContractMismatch` is
defined by the router, raised by adapters when a resolved `Provider` does not
satisfy the expected port interface (Q&A #6).

### 3.2 Reset mechanism for tests

A build-tagged helper in `registry.go` exposes reset capability for the router
test suite only:

```go
//go:build test

func RouterResetForTest() {
    registry.Store(nil)
}
```

Called from `suite.SetupTest()`. This is the only test seam into frozen code.
It is acceptable — the alternative is subprocess isolation which is heavy and
unnecessary for unit tests.

This helper is test-only API, not production API. It exists because the suite
lives under `internal/tests/router/` and must be able to reset global router
state between tests without moving the suite into the production package.

### 3.3 router.lock

`router.lock` is a **development integrity guardrail**, not a runtime boot requirement.

Its sole purpose is to prevent an AI agent from editing frozen router files to
circumvent the port system — which would reintroduce exactly the adapter coupling
the router exists to prevent. The lock detects that drift. Host tooling or the
explicit router-local `wrlk` flow enforces it. `wrlk lock update` is an explicit
operation, never an accidental side effect.

**router.lock has no effect on router boot.** The router does not read or check
`router.lock` at runtime. Integrity verification is performed by host tooling
or the router-local `wrlk` CLI as a separate, out-of-band step.

Consequence for tests: `integrity_test.go` does not exist in the router test suite.
The lock tool is a separate concern and gets its own test suite under
`internal/tests/router/tools/wrlk/`.

`ChecksumMismatch`, `FrozenFileModified`, and `LockCorrupt` are **not** `RouterError`
codes. They are outputs of the tooling that enforces the lock — not the router.
These three codes are removed from the router error catalog.

### 3.3.1 `wrlk` CLI contract

`wrlk` is the router-local CLI shipped with the copy-paste router bundle.

Initial command surface:
- `wrlk live run`
- `wrlk lock verify`
- `wrlk lock update`

Rules:
- `wrlk` is optional and has **no bearing on regular operations**
- active behavior is explicit via subcommand; nothing accidental runs by default
- `live run` is an explicit live verification mode only
- `lock verify` is read-only
- `lock update` rewrites the lock only when explicitly invoked
- later `portgen` and router-extension maintenance commands may live under the same CLI
- `wrlk` must remain embeddable in a larger host CLI structure without breaking downstream behavior

`wrlk live run` semantics:
- start a bounded verification session
- wait for expected verification participants to report success or failure
- all expected success reports before timeout => exit `0`
- any reported failure => print the relevant error and exit non-zero
- timeout => non-zero and classified as a verification bug, not a normal router runtime condition

### 3.4 Required vs optional extensions

`Required() bool` on the `Extension` interface drives the fatal/non-fatal split:

- `Required()` returns `true` → failure is `RequiredExtensionFailed`, boot aborts
- `Required()` returns `false` → failure is `OptionalExtensionFailed`, warning collected, boot continues

This also drives the assert/require split in tests directly — see Section 5.

### 3.5 RouterSealRegistry

Not in `router-final.md`. Not in the Q&A. Does not exist.

Sealing the registry at a specific public call would be hard runtime enforcement,
which contradicts the opt-in philosophy of `router.lock`. Removed from the original
design for good reason.

### 3.6 Public API surface

Exactly three production public functions, as defined by `router-final.md`:

```go
// extensions.go — MUTABLE
// One-line wrapper. The only boot call main.go makes.
func RouterBootExtensions(ctx context.Context) ([]error, error)

// extension.go — FROZEN
// Takes both extension layers as parameters so its checksum stays stable.
func RouterLoadExtensions(optionalExts []Extension, exts []Extension, ctx context.Context) ([]error, error)

// registry.go — FROZEN
// Lock-free read post-boot. nil registry = RegistryNotBooted.
func RouterResolveProvider(port PortName) (Provider, error)
```

`RouterRegisterProvider` is not public API. It is either the internal implementation
the `Registry` handle delegates to, or inlined into the handle method (Q&A #2).

### 3.7 Extension layering

The current test strategy must account for the newer design requirement that the
router support two distinct extension layers:

- an optional extension path that boots first
- a primary application extension path that boots second

These are separate wiring surfaces, not one mixed list with comments. The test
strategy therefore has to validate:

- optional-layer boot succeeds independently
- application-layer boot can consume ports published by the optional layer
- boot order is load-bearing across the two layers
- failure to satisfy a cross-layer dependency returns the existing dependency/order
  error rather than entering runtime partially configured

This is a router boot concern, so coverage belongs primarily in `boot_test.go`.

Restricted or trusted-port enforcement is not part of the initial router
implementation. The TDD suite must not activate tests for that capability yet.

### 3.8 Extension interface

Confirmed from `router-final.md` and Q&A only:

```go
type Extension interface {
    // Required declares whether boot failure is fatal.
    Required() bool

    // Consumes declares ports this extension depends on during boot.
    // Topological sorting is NOT implemented. Return nil to skip.
    // Manual extensions slice order is load-bearing when dependencies exist.
    Consumes() []PortName

    // RouterProvideRegistration performs the actual provider binding.
    // Implementations call reg.RouterRegisterProvider for each port they provide.
    RouterProvideRegistration(reg *Registry) error
}

// AsyncExtension is detected via type assertion during boot.
type AsyncExtension interface {
    Extension
    RouterProvideAsyncRegistration(reg *Registry, ctx context.Context) error
}

// ErrorFormattingExtension is detected via type assertion during boot.
// Cannot downgrade fatal errors to warnings.
type ErrorFormattingExtension interface {
    Extension
    ErrorFormatter() RouterErrorFormatter
}
```

`Name()` and `Ports()` are not in the design document. Scrapped doc artifacts. Dropped.

### 3.9 Concurrent boot safety

Boot publishes via `registry.CompareAndSwap(nil, &localMap)`. If two goroutines
race to boot, exactly one CAS succeeds. The loser receives `false` and returns
`MultipleInitializations`. No separate mutex or `sync.Once` required — the atomic
pointer is both the state and the concurrency primitive (Q&A #15).

### 3.10 ports.go rules

Carried from scrapped doc — rules only, not contradicted by `router-final.md`:

- One constant per port, no exceptions
- Names are lowercase strings, PascalCase constants
- Removing a constant is a breaking change — deprecate first, remove later
- Constants live in `ports.go` only — adapters import this package to reference them

---

## 4. File Structure

```
internal/
├── router/
│   ├── ports.go                  (mutable — PortName constants)
│   ├── registry_imports.go       (mutable — atomic registry + routerValidatePortName)
│   ├── extensions.go             (mutable — application extensions + RouterBootExtensions wrapper)
│   ├── optional_extensions.go    (mutable — optional extension list and optional boot wiring)
│   ├── extension.go              (frozen — Extension interfaces + RouterLoadExtensions)
│   ├── registry.go               (frozen — atomic publication + RouterResolveProvider)
│   ├── router.lock               (NDJSON checksums — host tooling or `wrlk` enforces, router ignores at runtime)
│   └── tools/
│       └── wrlk/
│           └── main.go           (router-local CLI for live checks and lock workflows)
│
├── ports/
│   ├── config.go                 (ConfigProvider interface)
│   ├── walk.go                   (WalkProvider interface)
│   └── scanners.go               (ScannerProvider interface)
│
├── adapters/
│   ├── config/
│   │   └── extension.go          (implements router.Extension)
│   ├── walk/
│   │   └── extension.go
│   └── scanners/
│       └── extension.go
│
└── tests/
    └── router/
        ├── helpers_test.go       (MockExtension + RouterSuite — build first)
        ├── registration_test.go  (phase 1)
        ├── resolution_test.go    (phase 2)
        └── boot_test.go          (phase 3)
```

---

## 5. Test Dependencies and assert/require Split

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)
```

`testify` is for correctness tests in `internal/tests/router/`. Benchmarks use the
standard library `testing` package directly so the measured path does not include
assertion helper overhead.

**require** — fatal. Test stops on failure. Use when the rest of the test is invalid
if this assertion fails.

**assert** — non-fatal. Test continues. Use when seeing all failures at once is useful.

### Decision rule

```
RouterBootExtensions fails unexpectedly         → require
RouterResolveProvider fails before type assert  → require
Required extension boot path                    → require (Required() == true)
Optional extension boot path                    → assert  (Required() == false)
Checking fields on a RouterError                → assert (see all field failures)
Checking error message contains port name       → assert (message format, not fatal)
Checking warning list length + contents         → assert (both failures useful together)
```

The `Required() bool` return value on `Extension` maps directly to this split.
A required extension test uses `require`. An optional extension test uses `assert`.
This is not a coincidence — it is the design intent.

---

## 6. MockExtension Design (`helpers_test.go`)

```go
type MockExtension struct {
    mock.Mock

    // Controls boot behaviour.
    BootError  error
    AsyncDelay time.Duration

    // Controls Required() return value.
    IsRequired bool

    // Controls Consumes() return value.
    ConsumedPorts []router.PortName

    // Controls what this extension registers during RouterProvideRegistration.
    RegistersPort     router.PortName
    RegistersProvider router.Provider
}

func (m *MockExtension) Required() bool              { return m.IsRequired }
func (m *MockExtension) Consumes() []router.PortName { return m.ConsumedPorts }
func (m *MockExtension) RouterProvideRegistration(reg *router.Registry) error {
    if m.BootError != nil {
        return m.BootError
    }
    if m.RegistersPort != "" {
        return reg.RouterRegisterProvider(m.RegistersPort, m.RegistersProvider)
    }
    return nil
}
```

Optional interfaces are implemented via separate types that embed `MockExtension`
so the type assertion inside the router fires correctly:

```go
type MockAsyncExtension struct {
    MockExtension
}

func (m *MockAsyncExtension) RouterProvideAsyncRegistration(
    reg *router.Registry,
    ctx context.Context,
) error {
    if m.AsyncDelay > 0 {
        select {
        case <-time.After(m.AsyncDelay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return m.MockExtension.RouterProvideRegistration(reg)
}

type MockErrorFormattingExtension struct {
    MockExtension
    Formatter router.RouterErrorFormatter
}

func (m *MockErrorFormattingExtension) ErrorFormatter() router.RouterErrorFormatter {
    return m.Formatter
}
```

### Factory helpers

```go
func requiredExtension(port router.PortName, provider router.Provider) *MockExtension
func optionalExtension(port router.PortName, provider router.Provider) *MockExtension
func failingRequiredExtension(err error) *MockExtension
func failingOptionalExtension(err error) *MockExtension
func asyncExtension(port router.PortName, delay time.Duration) *MockAsyncExtension
func unknownPortExtension() *MockExtension
func duplicatePortExtension(port router.PortName) *MockExtension
```

### Suite setup

```go
type RouterSuite struct {
    suite.Suite
}

func (s *RouterSuite) SetupTest() {
    router.RouterResetForTest() // build-tagged helper in registry.go
}

func TestRouterSuite(t *testing.T) {
    suite.Run(t, new(RouterSuite))
}
```

---

## 7. Error Catalog

Cleaned of integrity codes — those belong to host tooling, not the router.

| Code                       | Category     | Port name in message                        |
| -------------------------- | ------------ | ------------------------------------------- |
| `PortUnknown`              | Registration | yes — port name that failed validation      |
| `PortDuplicate`            | Registration | yes — port name already registered          |
| `InvalidProvider`          | Registration | no                                          |
| `PortNotFound`             | Resolution   | yes — port name not found                   |
| `RegistryNotBooted`        | Resolution   | no                                          |
| `PortContractMismatch`     | Resolution   | yes — defined by router, raised by adapters |
| `RequiredExtensionFailed`  | Boot         | no                                          |
| `OptionalExtensionFailed`  | Boot         | no                                          |
| `DependencyOrderViolation` | Boot         | yes + mandated message text                 |
| `AsyncInitTimeout`         | Boot         | no                                          |
| `MultipleInitializations`  | Boot         | no                                          |

**Mandated message formats (Q&A #11, #13):**

`PortUnknown` and `PortNotFound` must include port name:
```
port "auth" not found
port "auth" is not a declared port
```

`DependencyOrderViolation` must include port name and this exact text:
```
If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong.
Move the providing extension higher up in the correct extensions slice.
```

---

## 8. Test Phases

### TDD Workflow

For this router, TDD means:

1. Write one small failing test for one contract.
2. Run that test and confirm it fails for the expected reason.
3. Implement the minimum production code needed to make that test pass.
4. Run the targeted test again until it passes.
5. Run the already-green router tests to confirm nothing regressed.
6. Only then write the next failing test.

Do not write the whole router first and “add tests after”. For this project,
the tests are the implementation checklist.

### Phase 1 — Registration (`registration_test.go`)

**What is under test:** The port whitelist contract. `routerValidatePortName` +
`Registry` handle behaviour.

**Why first:** If registration is broken, resolution and boot tests are meaningless.

| Test                                   | Setup                                    | require/assert                | Expected                                         |
| -------------------------------------- | ---------------------------------------- | ----------------------------- | ------------------------------------------------ |
| `TestPortUnknown_IncludesPortName`     | Extension registers `"unknown_port"`     | require error, assert fields  | `PortUnknown`, message contains `"unknown_port"` |
| `TestPortDuplicate_SecondFails`        | Two extensions register `PortConfig`     | require error on second       | `PortDuplicate`, message contains `"config"`     |
| `TestPortDuplicate_FirstWins`          | Two extensions register same port in one boot attempt | require error, assert fields | `PortDuplicate`; no partial publication          |
| `TestInvalidProvider_NilRejected`      | Extension registers nil provider         | require error                 | `InvalidProvider`                                |
| `TestValidRegistration_Passes`         | Extension registers `PortConfig` cleanly | require boot, require resolve | Provider returned, no error                      |
| `TestAllDeclaredPorts_RegisterCleanly` | One extension per declared port          | require boot                  | All declared ports resolve                       |

---

### Phase 2 — Resolution (`resolution_test.go`)

**What is under test:** The atomic publication model. Pre-boot behaviour, post-boot
behaviour, immutability, concurrent reads.

**Why second:** Registration confirms the write side. Resolution confirms the read side.

| Test                                       | Setup                                             | require/assert                   | Expected                                   |
| ------------------------------------------ | ------------------------------------------------- | -------------------------------- | ------------------------------------------ |
| `TestRegistryNotBooted_BeforeBoot`         | Resolve before boot                               | require error                    | `RegistryNotBooted`                        |
| `TestPortNotFound_IncludesPortName`        | Boot succeeds, resolve unregistered port          | require error, assert message    | `PortNotFound`, message contains port name |
| `TestResolve_ReturnsCorrectProvider`       | Boot with `PortConfig`, resolve                   | require boot, require resolve    | Exact provider instance returned           |
| `TestResolve_ImmutableAfterBoot`           | Boot succeeds, attempt second boot                | require error on second          | `MultipleInitializations`                  |
| `TestResolve_ConcurrentReads_NoRace`       | Boot succeeds, 100 goroutines resolve             | require all return same provider | No race detector errors                    |
| `TestPortContractMismatch_StructuredError` | Resolve succeeds, adapter asserts wrong interface | assert error type                | `RouterError{Code: PortContractMismatch}`  |

**`TestResolve_ConcurrentReads_NoRace`** must run with `-race`. This is the proof
of Model A — lock-free reads post-boot via immutable atomic snapshot.

---

### Phase 3 — Boot Path (`boot_test.go`)

**What is under test:** Full boot lifecycle. Happy path, failure modes, async
extensions, ordering violations, multiple initialization prevention.

**Why third:** Boot depends on registration working correctly.

| Test                                              | Setup                                                       | require/assert                      | Expected                                                               |
| ------------------------------------------------- | ----------------------------------------------------------- | ----------------------------------- | ---------------------------------------------------------------------- |
| `TestBoot_HappyPath`                              | Three required extensions, all clean                        | require                             | `(nil, nil)`, registry non-nil                                         |
| `TestBoot_MultipleInitializations`                | Call boot twice                                             | require first, require error second | `MultipleInitializations` on second call                               |
| `TestBoot_ConcurrentBoot_ExactlyOneSucceeds`      | 10 goroutines call boot simultaneously                      | assert                              | Exactly 1 success, 9 `MultipleInitializations`                         |
| `TestBoot_RequiredFails_AbortsAll`                | Required extension returns error                            | require error                       | `RequiredExtensionFailed`, registry nil                                |
| `TestBoot_OptionalFails_Continues`                | Optional extension returns error                            | assert warning, require boot        | Warning in slice, boot succeeds                                        |
| `TestBoot_AsyncCompletes_BeforeDeadline`          | Async extension, delay < ctx deadline                       | require boot                        | Boot succeeds                                                          |
| `TestBoot_AsyncTimeout`                           | Async extension, delay > ctx deadline                       | require error                       | `AsyncInitTimeout`                                                     |
| `TestBoot_ContextCancelled_StopsAsync`            | ctx cancelled mid-boot                                      | require error                       | Boot fails, registry nil                                               |
| `TestBoot_DependencyOrderViolation_MessageFormat` | Extension B needs port from A, B listed first               | require error, assert message       | `DependencyOrderViolation`, message contains port name + mandated text |
| `TestBoot_EmptyExtensionSlices`                   | `optionalExtensions = nil`, `extensions = nil`              | require boot                        | Succeeds, all resolves return `PortNotFound`                           |
| `TestBoot_OptionalLayer_BootsBeforeApplication`   | Optional list provides port, application list consumes it   | require boot                        | Boot succeeds, application sees optional-layer provider                |
| `TestBoot_OptionalLayer_Empty_ApplicationOnly`    | Optional list empty, application list self-contained        | require boot                        | Boot succeeds                                                          |
| `TestBoot_CrossLayer_DependencyOrderViolation`    | Application list consumes port absent from optional layer   | require error, assert message       | `DependencyOrderViolation`                                             |
| `TestBoot_OptionalLayer_RequiredFails_AbortsBoot` | Required optional extension returns error                   | require error                       | `RequiredExtensionFailed`, application layer never enters runtime      |
| `TestBoot_OptionalLayer_OptionalFails_Continues`  | Optional optional-layer extension fails, app layer valid    | assert warning, require boot        | Warning recorded, application layer still boots if dependencies permit |
| `TestBoot_ErrorFormatter_UsedForThatExtension`    | Extension implements `ErrorFormattingExtension`, Init fails | assert error                        | Error formatted via custom formatter                                   |
| `TestBoot_ErrorFormatter_CannotDowngradeFatal`    | Custom formatter returns non-fatal for fatal                | require error                       | Fatal classification preserved                                         |

**`TestBoot_ConcurrentBoot_ExactlyOneSucceeds`** — run with `-race`. Verifies CAS
is the mechanism, not Store. Exactly one goroutine wins `CompareAndSwap`. All others
return `MultipleInitializations`.

```go
func (s *RouterSuite) TestBoot_ConcurrentBoot_ExactlyOneSucceeds() {
    const goroutines = 10
    results := make([]error, goroutines)
    var wg sync.WaitGroup

    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            _, err := router.RouterBootExtensions(context.Background())
            results[idx] = err
        }(i)
    }
    wg.Wait()

    successCount := 0
    for _, err := range results {
        if err == nil {
            successCount++
        }
    }
    assert.Equal(s.T(), 1, successCount,
        "exactly one CAS winner expected, all others must return MultipleInitializations")
}
```

---

### Phase 4 — Resolve Benchmark (`benchmark_test.go`)

**What is under test:** Post-boot `RouterResolveProvider` throughput under parallel
load.

**Why fourth:** This is not a correctness gate. It validates the design claim that
Model A gives lock-free reads after boot via the immutable atomic snapshot.

**File location:** `internal/tests/router/benchmark_test.go`

**Benchmark contract:** Boot the router first, exclude boot time from measurement,
then hammer `RouterResolveProvider(PortConfig)` with `b.RunParallel`.

`testify` should not be used in the measured benchmark path. Use `b.Fatalf` for
setup failures and unexpected benchmark-time errors.

```go
package router_test

import (
    "context"
    "testing"

    "your-project/internal/router"
)

// Run with: go test -tags test -bench=. -benchtime=3s ./internal/tests/router/...
func BenchmarkRouterResolve(b *testing.B) {
    router.RouterResetForTest()

    if _, err := router.RouterBootExtensions(context.Background()); err != nil {
        b.Fatalf("RouterBootExtensions: %v", err)
    }

    b.ResetTimer()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            if _, err := router.RouterResolveProvider(router.PortConfig); err != nil {
                b.Fatalf("RouterResolveProvider: %v", err)
            }
        }
    })
}
```

**Notes:**
- benchmark the resolve path only, not boot or registration
- keep setup deterministic so comparisons stay meaningful across runs
- if a dummy provider or benchmark-only extension is needed, keep it local to router test code

---

### Phase 5 — Planned Feature Tests (Not Active Yet)

These are design-tracking tests for planned capabilities. They should not become
active red tests until the corresponding router contracts are accepted for
implementation.

#### 5.1 `wrlk` tool tests

Coverage belongs in a separate CLI/tool suite, not in router runtime tests.

Active or immediate cases:
- `wrlk lock verify` succeeds for a matching lock and fails on drift
- `wrlk lock verify` does not rewrite the lock
- `wrlk lock update` writes or rewrites `router.lock` atomically
- `wrlk` works against a real pilot-project router fixture, not only synthetic files

Later cases:
- `wrlk live run` succeeds when all expected participants report success
- `wrlk live run` prints the relevant failure and exits non-zero when any participant reports failure
- `wrlk live run` times out as a bug when the verification session does not complete
- `wrlk` remains embeddable under a larger host CLI command tree

#### 5.2 `portgen` tool tests

Coverage belongs in a separate generator/tool test suite, not in router runtime
tests.

Planned cases:
- adding a port updates `ports.go`
- adding a port updates `routerValidatePortName` in `registry_imports.go`
- lock file rewrite is atomic
- generated output is idempotent when rerun with the same port
- duplicate port addition fails with actionable output

#### 5.3 Restricted/private port resolution

Coverage belongs in router runtime tests only after consumer identity becomes part
of the resolution contract.

Planned cases:
- trusted consumer resolves restricted port successfully
- untrusted consumer receives access-denied error
- unrestricted ports remain globally resolvable
- trust policy is defined in mutable wiring, not frozen router code

---

## 9. Running the Tests

```bash
# All router tests with test build tag and race detector
go test -tags test ./internal/tests/router/... -race -v

# Specific phase
go test -tags test ./internal/tests/router/... -run TestRegistration -race -v
go test -tags test ./internal/tests/router/... -run TestResolution -race -v
go test -tags test ./internal/tests/router/... -run TestBoot -race -v

# Resolve benchmark
go test -tags test -bench=. -benchtime=3s ./internal/tests/router/...
```

The `-tags test` flag is required on every run — it exposes `RouterResetForTest()`
from `registry.go`. Without it the suite will not compile.

---

## 10. Implementation Order

```
1.  helpers_test.go          — MockExtension + RouterSuite (nothing passes, nothing exists)
2.  registration_test.go     — all tests written red
3.  Implement registration   — ports.go, registry_imports.go, Registry handle in registry.go
4.  registration_test.go     — all green
5.  resolution_test.go       — all tests written red
6.  Implement resolution     — atomic publication + RouterResolveProvider in registry.go
7.  resolution_test.go       — all green
8.  boot_test.go             — all tests written red
9.  Implement boot           — RouterLoadExtensions in extension.go + optional/application layer wrapper in extensions.go
10. boot_test.go             — all green
11. Wire the pilot project   — PortConfig, PortWalk, PortScanner adapters
12. Boot in policy_manager.go — RouterBootExtensions called once at startup
13. Resolve in each group    — RouterResolveProvider called per policy group
```

For a first TDD pass, stop after step 10. Do not mix pilot-project integration
into the first router red-green cycle unless the router tests are already green.

---

## 11. What This Does Not Cover

- **router.lock enforcement** — host project tooling or `wrlk` concern, separate test suite from router runtime
- **Real adapter implementations** — adapter tests live in `internal/tests/adapters/`
- **Pilot-project policy tests** — live in the host project's policy test suite
- **Integration tests** — router + real adapters booted together, separate suite
