# [Component] TDD Tasklist — v0.1

Use this file as a session checklist for [component] work. Adapt phases as needed
for your specific project. Complete one phase at a time before moving to the next.

## Session Start

- [ ] Read [AGENTS.md](AGENTS.md).
- [ ] Read [.codex/rules/general.md](.codex/rules/general.md).
- [ ] Read [.codex/rules/go.md](.codex/rules/go.md) (or your language-specific rules).
- [ ] Read your design document: `[component]-final.md`.
- [ ] Read your test strategy: `[component]-test-strategy.md`.
- [ ] Reconfirm the active phase before making changes.

## Standards Review

- [ ] Use only component-specific standards for this TDD track.
- [ ] Errors are never swallowed — always propagate or log.
- [ ] Returned errors are wrapped with context.
- [ ] No mutable global state beyond what the design requires.
- [ ] `context.Context` is passed into async paths only; not stored on structs.
- [ ] Any goroutines must honor cancellation and be covered by `-race` validation.
- [ ] Ignore broader standards that conflict with repo rules.

---

## Phase 1: Test Harness

**Goal:** Build the test infrastructure before writing any tests.

- [ ] Create `internal/tests/[component]/helpers_test.go`.
- [ ] Add test suite struct (e.g., `ComponentSuite` with `SetupTest`).
- [ ] Add mock types for your component's interfaces.
- [ ] Add factory helpers for common test scenarios.
- [ ] Add a test-only reset seam for global state (if needed).

Completion criteria:
- [ ] Test harness compiles (may fail on missing production symbols).
- [ ] No real component behavior implemented yet.

---

## Phase 2: First Test Phase (Red)

**Goal:** Write failing tests for the first set of behaviors.

- [ ] Create `internal/tests/[component]/[testarea]_test.go`.
- [ ] Write failing tests for [behavior 1].
- [ ] Write failing tests for [behavior 2].
- [ ] Write failing tests for [behavior 3].
- [ ] Run tests and confirm they fail for expected reasons.

Completion criteria:
- [ ] Tests fail for missing production behavior, not broken scaffolding.
- [ ] No implementation work started yet.

---

## Phase 3: First Test Phase (Green)

**Goal:** Implement the minimum code to make tests pass.

- [ ] Create `internal/[component]/[file1].go`.
- [ ] Create `internal/[component]/[file2].go`.
- [ ] Implement [specific functionality].
- [ ] Make tests pass.
- [ ] Run `go run ./cmd/[project]` to verify.

Completion criteria:
- [ ] All tests in this phase pass.
- [ ] Behavior is structured and deterministic.

---

## Phase 4: Second Test Phase (Red)

**Goal:** Write failing tests for the next set of behaviors.

- [ ] Create `internal/tests/[component]/[testarea2]_test.go`.
- [ ] Write failing tests for [behavior 4].
- [ ] Write failing tests for [behavior 5].
- [ ] Run tests and confirm they fail for expected reasons.

---

## Phase 5: Second Test Phase (Green)

**Goal:** Implement the next layer of functionality.

- [ ] Create `internal/[component]/[file3].go`.
- [ ] Implement [specific functionality].
- [ ] Make all tests from Phase 2 and 4 pass.
- [ ] Run concurrency tests with `-race`.
- [ ] Run `go run ./cmd/[project]`.

Completion criteria:
- [ ] Tests from both phases are green.
- [ ] Concurrent behavior is race-free.

---

## Phase 6: Error Surface Stabilization

**Goal:** Centralize error handling before adding more complexity.

- [ ] Centralize error message construction.
- [ ] Ensure all mandated error messages are defined centrally.
- [ ] Preserve the error catalog semantics.
- [ ] Make tests pass after refactor.
- [ ] Run `go run ./cmd/[project]`.

Completion criteria:
- [ ] Error messages not scattered across files.
- [ ] Ready for host-specific error shaping later.

---

## Phase 7: Third Test Phase (Red)

**Goal:** Write failing tests for [boot/lifecycle/advanced behavior].

- [ ] Create `internal/tests/[component]/[testarea3]_test.go`.
- [ ] Write happy-path tests.
- [ ] Write failure mode tests.
- [ ] Write async/timeout tests if applicable.
- [ ] Run tests and confirm they fail.

---

## Phase 8: Third Test Phase (Green)

**Goal:** Implement boot/lifecycle/advanced behavior.

- [ ] Create `internal/[component]/[file4].go`.
- [ ] Implement [boot/lifecycle logic].
- [ ] Make all tests pass.
- [ ] Run `go run ./cmd/[project]`.

Completion criteria:
- [ ] Full test suite is green.
- [ ] No partial state on failure.

---

## Phase 9: Benchmark (Optional)

**Goal:** Validate performance characteristics.

- [ ] Create `internal/tests/[component]/benchmark_test.go`.
- [ ] Benchmark the critical path.
- [ ] Keep benchmark free of test helpers in measured path.

Completion criteria:
- [ ] Benchmark runs successfully.
- [ ] No production logic distorted for numbers.

---

## Phase 10: Integration Wiring

**Goal:** Connect component to host project.

- [ ] Create port interfaces in `internal/ports/`.
- [ ] Create adapter extensions.
- [ ] Wire boot into startup path.
- [ ] Wire usage into consumer paths.
- [ ] Run component tests.
- [ ] Run `go run ./cmd/[project]`.

Completion criteria:
- [ ] Component tests remain green.
- [ ] No contracts changed by integration.

---

## Phase N: Additional Phases

*[Add phases as needed for your specific project. Examples:]*

- **Tooling Phase:** Add CLI tool for the component
- **Feature Phase:** Add planned feature X
- **Feature Phase:** Add planned feature Y

---

## End of Session

- [ ] Record the phase reached.
- [ ] Record what is green, what is red.
- [ ] Record what the next session should do first.
- [ ] Stop after the active phase is complete.

Session note:
- Completed through Phase [N]: [Description].
- Green: `go test -tags test ./internal/tests/[component]/... -v`.
- Green: `go run ./cmd/[project]`.
- [Any notes about state or blockers]
- Next session starts with: [What to do next]

---

## Template Notes

This template is intentionally lighter than the router's tasklist. Not every
component needs 14 phases. Adapt the phases to your project:

- **Smaller components** may only need 3-4 phases (harness → tests → implementation → integration).
- **Larger components** may need more phases for tooling, features, or complex lifecycle.
- **Skip phases** that don't apply (e.g., skip benchmark if not performance-critical).
- **Add phases** as you discover new work during implementation.

The key TDD principles remain:
1. Write one small failing test
2. Confirm it fails for the right reason
3. Implement minimum code to pass
4. Run all tests to check for regression
5. Repeat