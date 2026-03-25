# [Component Name] Test Strategy

**Version:** 0.1.0
**Status:** Draft — test strategy in progress
**Scope:** `internal/[component]` — TDD test suite
**Source of truth:** `[component]-final.md`

---

## 1. Context

[Component description]. Its value is [value proposition].
Tests verify contracts, not implementation details.

All tests are **red first**. The [component] implementation does not exist yet.
Tests are written against `[component]-final.md` and resolved Q&A only.

This document is written to support a first TDD implementation. The goal is not
just to list tests. The goal is to define the order of work so each red test
forces one small [component] behavior into existence before the next one is added.

---

## 2. Pilot Project

**[Project Name]** (`internal/[project]/`) is the host project used to pilot the [component].

Natural ports/contracts identified:

| Contract constant | Contract string | Provided by                   |
| ----------------- | --------------- | ----------------------------- |
| `ContractX`       | `"x"`           | `internal/adapters/[adapter]` |
| `ContractY`       | `"y"`           | `internal/adapters/[adapter]` |
| `ContractZ`       | `"z"`           | `internal/adapters/[adapter]` |

Port interfaces (`XProvider`, `YProvider`, `ZProvider`) live in `internal/ports/`.
Each adapter package exposes a struct that implements `[component].Extension`.

---

## 3. Resolved Design Decisions

All open decisions from the design are now closed.

### 3.1 Provider type

```go
type Provider any
```

[Explain how the provider type is used in this component.]

### 3.2 Reset mechanism for tests

A build-tagged helper exposes reset capability for the [component] test suite only:

```go
//go:build test

func [Component]ResetForTest() {
    // Reset implementation
}
```

Called from `suite.SetupTest()`. This is the only test seam into frozen code.

### 3.3 Lock/Integrity mechanism

[Component].lock is a **development integrity guardrail**, not a runtime requirement.

[Describe the lock file purpose and how it's enforced.]

**Consequence for tests:** [component]_integrity_test.go does not exist in the [component] test suite.

### 3.4 Required vs optional extensions

`Required() bool` on the `Extension` interface drives the fatal/non-fatal split:

- `Required()` returns `true` → failure is `RequiredExtensionFailed`, boot aborts
- `Required()` returns `false` → failure is `OptionalExtensionFailed`, warning collected, boot continues

This also drives the assert/require split in tests directly.

### 3.5 Public API surface

Exactly three production public functions:

```go
// [file].go — MUTABLE
// One-line wrapper.
func [Component]Boot[Something](ctx context.Context) ([]error, error)

// [file].go — FROZEN
// Takes parameters so checksum stays stable.
func [Component]Load[Something](exts []Extension, ctx context.Context) ([]error, error)

// [file].go — FROZEN
// Lock-free read post-boot. nil registry = NotBooted.
func [Component]Resolve[Something](name TypeName) (Provider, error)
```

### 3.6 Extension layering

The [component] support two distinct extension layers:
- an optional extension path that boots first
- a primary application extension path that boots second

[Describe how the test strategy validates layered boot.]

### 3.7 Extension interface

```go
type Extension interface {
    Required() bool
    Consumes() []TypeName
    ProvideRegistration(reg *Registry) error
}

type AsyncExtension interface {
    Extension
    ProvideAsyncRegistration(reg *Registry, ctx context.Context) error
}

type ErrorFormattingExtension interface {
    Extension
    ErrorFormatter() ErrorFormatter
}
```

---

## 4. File Structure

```
internal/
├── [component]/
│   ├── [file1].go           (mutable — [purpose])
│   ├── [file2].go           (mutable — [purpose])
│   ├── [file3].go           (frozen — [purpose])
│   ├── [file4].go           (frozen — [purpose])
│   └── tools/
│       └── [tool]/
│           └── main.go      (tool for [purpose])
│
├── ports/
│   ├── [interface1].go     (XProvider interface)
│   ├── [interface2].go      (YProvider interface)
│   └── [interface3].go     (ZProvider interface)
│
├── adapters/
│   ├── [adapter1]/
│   │   └── extension.go     (implements Extension)
│   ├── [adapter2]/
│   │   └── extension.go
│   └── [adapter3]/
│       └── extension.go
│
└── tests/
    └── [component]/
        ├── helpers_test.go  (MockExtension + Suite)
        ├── [test1]_test.go  (phase 1)
        ├── [test2]_test.go  (phase 2)
        └── [test3]_test.go  (phase 3)
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

**require** — fatal. Test stops on failure. Use when the rest of the test is invalid
if this assertion fails.

**assert** — non-fatal. Test continues. Use when seeing all failures at once is useful.

### Decision rule

```
[Component]Boot fails unexpectedly         → require
[Component]Resolve fails before operation  → require
Required extension boot path               → require (Required() == true)
Optional extension boot path               → assert  (Required() == false)
Checking fields on an Error                → assert
Checking error message contains name      → assert
Checking warning list length + contents   → assert
```

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
    ConsumedPorts []TypeName

    // Controls what this extension registers.
    RegistersPort     TypeName
    RegistersProvider Provider
}

func (m *MockExtension) Required() bool              { return m.IsRequired }
func (m *MockExtension) Consumes() []TypeName         { return m.ConsumedPorts }
func (m *MockExtension) ProvideRegistration(reg *Registry) error {
    if m.BootError != nil {
        return m.BootError
    }
    if m.RegistersPort != "" {
        return reg.RegisterProvider(m.RegistersPort, m.RegistersProvider)
    }
    return nil
}
```

Optional interfaces:

```go
type MockAsyncExtension struct {
    MockExtension
}

func (m *MockAsyncExtension) ProvideAsyncRegistration(
    reg *Registry,
    ctx context.Context,
) error {
    // Implementation
}
```

### Factory helpers

```go
func requiredExtension(port TypeName, provider Provider) *MockExtension
func optionalExtension(port TypeName, provider Provider) *MockExtension
func failingRequiredExtension(err error) *MockExtension
func failingOptionalExtension(err error) *MockExtension
func asyncExtension(port TypeName, delay time.Duration) *MockAsyncExtension
func unknownPortExtension() *MockExtension
func duplicatePortExtension(port TypeName) *MockExtension
```

### Suite setup

```go
type ComponentSuite struct {
    suite.Suite
}

func (s *ComponentSuite) SetupTest() {
    [Component]ResetForTest()
}

func TestComponentSuite(t *testing.T) {
    suite.Run(t, new(ComponentSuite))
}
```

---

## 7. Error Catalog

| Code         | Category     | Notes   |
| ------------ | ------------ | ------- |
| `ErrorCode1` | Registration | [Notes] |
| `ErrorCode2` | Registration | [Notes] |
| `ErrorCode3` | Resolution   | [Notes] |
| `ErrorCode4` | Boot         | [Notes] |
| `ErrorCode5` | Boot         | [Notes] |

**Mandated message formats:**
- `ErrorCode1`: [Format requirement]
- `ErrorCode3`: [Format requirement]

---

## 8. Test Phases

### TDD Workflow

1. Write one small failing test for one contract.
2. Run that test and confirm it fails for the expected reason.
3. Implement the minimum production code needed to make that test pass.
4. Run the targeted test again until it passes.
5. Run the already-green tests to confirm nothing regressed.
6. Only then write the next failing test.

### Phase 1 — [Test Name] (`[test]_test.go`)

**What is under test:** [What is being tested]

**Why first:** [Why this comes first]

| Test                    | Setup   | require/assert   | Expected   |
| ----------------------- | ------- | ---------------- | ---------- |
| `TestCase1_Description` | [Setup] | [assert/require] | [Expected] |
| `TestCase2_Description` | [Setup] | [assert/require] | [Expected] |

---

### Phase 2 — [Test Name] (`[test]_test.go`)

**What is under test:** [What is being tested]

**Why second:** [Why this comes second]

| Test                    | Setup   | require/assert   | Expected   |
| ----------------------- | ------- | ---------------- | ---------- |
| `TestCase1_Description` | [Setup] | [assert/require] | [Expected] |
| `TestCase2_Description` | [Setup] | [assert/require] | [Expected] |

---

### Phase 3 — [Test Name] (`[test]_test.go`)

**What is under test:** [What is being tested]

**Why third:** [Why this comes third]

| Test                    | Setup   | require/assert   | Expected   |
| ----------------------- | ------- | ---------------- | ---------- |
| `TestCase1_Description` | [Setup] | [assert/require] | [Expected] |
| `TestCase2_Description` | [Setup] | [assert/require] | [Expected] |

---

### Phase 4 — Benchmark (`benchmark_test.go`)

**What is under test:** [Performance aspect]

**Why fourth:** This is not a correctness gate.

**File location:** `internal/tests/[component]/benchmark_test.go`

**Benchmark contract:** [Describe benchmark setup and contract]

```go
func Benchmark[Component](b *testing.B) {
    // Setup
    b.ResetTimer()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // Measured path
        }
    })
}
```

---

### Phase 5 — Planned Feature Tests (Not Active Yet)

[Describe tests for planned capabilities that aren't implemented yet.]

#### 5.1 [Feature Name]

[Description]

#### 5.2 [Feature Name]

[Description]

---

## 9. Running the Tests

```bash
# All [component] tests with test build tag and race detector
go test -tags test ./internal/tests/[component]/... -race -v

# Specific phase
go test -tags test ./internal/tests/[component]/... -run TestPhaseName -race -v

# Benchmark
go test -tags test -bench=. -benchtime=3s ./internal/tests/[component]/...
```

The `-tags test` flag is required on every run.

---

## 10. Implementation Order

```
1.  helpers_test.go          — MockExtension + Suite
2.  [test1]_test.go          — all tests written red
3.  Implement [feature 1]    — [files]
4.  [test1]_test.go          — all green
5.  [test2]_test.go          — all tests written red
6.  Implement [feature 2]    — [files]
7.  [test2]_test.go          — all green
8.  [test3]_test.go          — all tests written red
9.  Implement [feature 3]   — [files]
10. [test3]_test.go          — all green
11. Wire the pilot project   — [adapters]
12. Boot in [project].go    — [Component]Boot called once at startup
13. Use in [consumers]      — [Component]Resolve called per [consumer]
```

For a first TDD pass, stop after step 10.

---

## 11. What This Does Not Cover

- **Lock enforcement** — host project tooling or tool concern
- **Real adapter implementations** — adapter tests live in `internal/tests/adapters/`
- **Integration tests** — [component] + real adapters together