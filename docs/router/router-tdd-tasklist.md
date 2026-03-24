# Router TDD Tasklist — v0.9

Use this file as the stable session checklist for router work. Complete one
phase at a time. Do not work ahead into later phases in the same session unless
the current phase is fully green and its completion criteria are satisfied.

## Session Start

- [x] Read [AGENTS.md](C:/Users/micha/.syntx/go/policyengine/AGENTS.md).
- [x] Read [.codex/rules/general.md](C:/Users/micha/.syntx/go/policyengine/.codex/rules/general.md).
- [x] Read [.codex/rules/go.md](C:/Users/micha/.syntx/go/policyengine/.codex/rules/go.md).
- [x] Read [router-final.md](C:/Users/micha/.syntx/go/policyengine/docs/router/router-final.md).
- [x] Read [router-test-strategy.md](C:/Users/micha/.syntx/go/policyengine/docs/router/router-test-strategy.md).
- [x] Reconfirm the active phase before making changes.
- [x] Confirm that trusted/restricted port enforcement is out of scope for the initial router implementation.

## Standards Review

- [x] Use only these router-specific standards for this TDD track:
- [x] Errors are never swallowed and returned errors are wrapped with context.
- [x] No mutable global router state beyond the single atomic published registry snapshot.
- [x] `context.Context` is passed into boot/async paths only; it is not stored on structs or used as a parameter bag.
- [x] Any goroutine started by router boot must honor cancellation, finish deterministically, and be covered by `-race` validation.
- [x] Concurrency primitives stay minimal and idiomatic; do not add channels or coordination layers the router design does not require.
- [x] Ignore broader standards examples that conflict with repo rules or add structure the router does not need.

## Phase 1: Test Harness

- [x] Create `internal/tests/router/helpers_test.go`.
- [x] Add `MockExtension`, `MockAsyncExtension`, and `MockErrorFormattingExtension`.
- [x] Add factory helpers for required, optional, duplicate, unknown-port, and async cases.
- [x] Add `RouterSuite`.
- [x] Add a test-only reset seam for router global state.

Completion criteria:
- [x] The test harness compiles or is blocked only by intentionally missing production router symbols.
- [x] No real router behavior is implemented yet beyond what the harness strictly needs.

## Phase 2: Registration Tests Red

- [x] Create `internal/tests/router/registration_test.go`.
- [x] Write `TestPortUnknown_IncludesPortName`.
- [x] Write `TestPortDuplicate_SecondFails`.
- [x] Write `TestInvalidProvider_NilRejected`.
- [x] Write `TestValidRegistration_Passes`.
- [x] Write `TestAllDeclaredPorts_RegisterCleanly`.
- [x] Run only registration tests and confirm they fail for the expected reasons.

Completion criteria:
- [x] Registration tests are red for missing or incomplete production behavior, not for broken test scaffolding.
- [x] No resolution or boot implementation work has started.

## Phase 3: Registration Green

- [x] Create `internal/router/ports.go`.
- [x] Create `internal/router/registry_imports.go`.
- [x] Add `PortName`, `Provider`, and declared port constants.
- [x] Add whitelist validation.
- [x] Add registration behavior on the `Registry` handle.
- [x] Make the registration tests pass.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] All registration tests pass.
- [x] Unknown port, duplicate port, and invalid provider behavior are structured and deterministic.

## Phase 4: Resolution Tests Red

- [x] Create `internal/tests/router/resolution_test.go`.
- [x] Write `TestRegistryNotBooted_BeforeBoot`.
- [x] Write `TestPortNotFound_IncludesPortName`.
- [x] Write `TestResolve_ReturnsCorrectProvider`.
- [x] Write `TestResolve_ImmutableAfterBoot`.
- [x] Write `TestResolve_ConcurrentReads_NoRace`.
- [x] Write `TestPortContractMismatch_StructuredError`.
- [x] Run only resolution tests and confirm they fail for the expected reasons.

Completion criteria:
- [x] Resolution tests are red for missing read/publication behavior, not for harness failures.

## Phase 5: Resolution Green

- [x] Create `internal/router/registry.go`.
- [x] Implement the atomic published snapshot model.
- [x] Implement `RouterResolveProvider`.
- [x] Make registration and resolution tests pass.
- [x] Run targeted concurrency tests with `-race`.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] Registration and resolution tests are green.
- [x] Concurrent read behavior is race-free.
- [x] Second boot attempt is rejected once publication has happened.

## Phase 5.5: Error Surface Stabilization

- [x] Centralize router error message construction in one place.
- [x] Ensure all mandated port-bearing error messages are defined centrally.
- [x] Preserve the canonical router error catalog and semantic meanings.
- [x] Add a small internal seam that allows future host-specific error shaping without changing router semantics.
- [x] Make registration and resolution tests pass after the error-surface refactor.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] Router error messages are not scattered across multiple files.
- [x] `PortUnknown`, `PortDuplicate`, `PortNotFound`, `DependencyOrderViolation`, and `PortContractMismatch` message rendering is centralized.
- [x] The router still owns semantic meanings even if outer error representation changes later.
- [x] The code is ready for later host-side error-shape mapping without rewriting the catalog.

## Phase 6: Boot Tests Red

- [x] Create `internal/tests/router/boot_test.go`.
- [x] Write happy-path boot tests.
- [x] Write required-vs-optional failure tests.
- [x] Write async completion, timeout, and cancellation tests.
- [x] Write dependency-order violation tests.
- [x] Write layered optional-before-application tests.
- [x] Write error formatter tests.
- [x] Run only boot tests and confirm they fail for expected reasons.

Completion criteria:
- [x] Boot tests are red for missing boot/orchestration behavior.
- [x] No trusted/restricted-port tests are introduced.

## Phase 7: Boot Green

- [x] Create `internal/router/extension.go`.
- [x] Create `internal/router/optional_extensions.go`.
- [x] Create `internal/router/extensions.go`.
- [x] Implement `Extension`, `AsyncExtension`, and `ErrorFormattingExtension`.
- [x] Implement `RouterError`, `RouterErrorFormatter`, and boot-time warning/error classification.
- [x] Implement `RouterLoadExtensions(optionalExts []Extension, exts []Extension, ctx context.Context)`.
- [x] Implement `RouterBootExtensions(ctx context.Context)` as optional layer first, application layer second.
- [x] Make registration, resolution, and boot tests pass.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] The full router core test suite is green.
- [x] Layered boot works without partial publication on failure.
- [x] Optional extension failure handling behaves as designed.

## Phase 8: Benchmark

- [x] Create `internal/tests/router/benchmark_test.go`.
- [x] Benchmark the resolve path only.
- [x] Keep the benchmark free of `testify` in the measured path.
- [x] Run the benchmark from `internal/tests/router`.

Completion criteria:
- [x] Benchmark runs successfully.
- [x] No production logic is distorted just to improve the benchmark.

## Phase 9: Policycheck Pilot Wiring

- [x] Create `internal/ports/config.go`.
- [x] Create `internal/ports/walk.go`.
- [x] Create `internal/ports/scanners.go`.
- [x] Create adapter extension files for `config`, `walk`, and `scanners`.
- [x] Wire `RouterBootExtensions` into the pilot startup path.
- [x] Wire `RouterResolveProvider` into the pilot consumer paths.
- [x] Run router tests.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] Router core remains green after host integration.
- [x] Pilot wiring does not change router contracts.

## Phase 10: Router Lock Integrity Tooling

- [x] Confirm that lock verification remains out-of-band host or router-local tooling, not router runtime behavior.
- [x] Decide that the user-facing entrypoint lives under `internal/router/tools/` so it stays copy-pasteable with the router bundle.
- [x] Add red TDD tests for lock verification against synthetic router fixtures.
- [x] Add red TDD tests for explicit lock update or regeneration workflow.
- [x] Add a real-project TDD case that exercises the tool against the policy engine router bundle and adapter-backed wiring.
- [x] Create the router-local tool entrypoint under `internal/router/tools/wrlk/`.
- [x] Implement read-only `verify` mode for `internal/router/router.lock`.
- [x] Implement explicit `update` mode that rewrites `internal/router/router.lock` atomically.
- [x] Keep the tool scoped to integrity only: no general host command surface, no router boot coupling, no business logic awareness.
- [x] Decide and document exactly which router files are lock-tracked in v1.
- [x] Add or update docs for tool usage and expected lock workflow.
- [x] Run the `wrlk` test package and make it green.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] `go test ./internal/tests/router/tools/wrlk -count=1` is green.
- [x] `verify` fails clearly on missing, corrupt, or drifted lock state.
- [x] `update` only rewrites the lock when explicitly requested.
- [x] The resulting tool remains router-generic and copy-pasteable with `internal/router/`.
- [x] No router runtime contracts or host coupling levels are weakened.

## Phase 11 — portgen Tool (v0.9)

**Goal:** Eliminate the two-file coherence problem between `ports.go` and `registry_imports.go`.

- [x] Create `internal/router/tools/portgen/main.go`.
- [x] Expose `RouterRunPortgenProcess(args []string, stdout io.Writer, stderr io.Writer) int`.
- [x] Implement `RouterParsePortgenFlags(args []string) (portgenOptions, []string, error)` with `--name`, `--value`, `--root`, `--dry-run` flags.
- [x] Implement `RouterAddPort(root, name, value string, dryRun bool) error` as the top-level action.
- [x] Implement `RouterWritePortsFile(path, name, value string) error` — injects constant atomically.
- [x] Implement `RouterWriteValidationFile(path, name string) error` — injects switch case atomically.
- [x] Implement `RouterWriteLockAfterPortgen(root string) error` — rewrites `router.lock` using the same hash logic as `wrlk lock update`.
- [x] Create `internal/tests/router/tools/portgen/portgen_test.go`.
- [x] Write tests red first:
  - [x] `TestPortgen_Add_UpdatesPortsFile`.
  - [x] `TestPortgen_Add_UpdatesValidation`.
  - [x] `TestPortgen_Add_UpdatesLock`.
  - [x] `TestPortgen_Add_Idempotent`.
  - [x] `TestPortgen_Add_DuplicateName_Fails`.
  - [x] `TestPortgen_Add_DryRun_NoWrite`.
- [x] Make portgen tests green.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] `go test ./internal/tests/router/tools/portgen/... -v -count=1` green.
- [x] Dry-run and idempotency cases confirmed.
- [x] `router.lock` rewritten correctly after a portgen add.

## Phase 11b — wrlk live run Tests (v0.9)

**Goal:** Full lifecycle test coverage for the existing `live.go` session implementation.

- [x] Create `internal/tests/router/tools/wrlk/live_test.go`.
- [x] Write tests red first:
  - [x] `TestLive_Run_AllParticipantsSucceed_ExitsZero`.
  - [x] `TestLive_Run_OneParticipantFails_ExitsNonZero`.
  - [x] `TestLive_Run_UnknownParticipant_Rejected`.
  - [x] `TestLive_Run_DuplicateParticipant_Rejected`.
  - [x] `TestLive_Run_Timeout_IsBug`.
  - [x] `TestLive_ReportPath_WrongMethod_NotFound`.
  - [x] `TestLive_ParseOptions_RequiresExpect`.
- [x] Confirm all live tests green (no new production code expected).
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] `go test ./internal/tests/router/tools/wrlk/... -v -count=1` green including live cases.
- [x] No changes to `live.go` production logic required (if changes needed, fix the implementation).

## Phase 12 — Topological Boot Ordering (v0.9)

**Goal:** Implement `Consumes()` topological sort so manual slice order is no longer load-bearing.

- [x] Add red tests in `internal/tests/router/boot_test.go`:
  - [x] `TestBoot_TopologicalSort_ResolvesOutOfOrderSlice`.
  - [x] `TestBoot_TopologicalSort_MultiLayerChain`.
  - [x] `TestBoot_TopologicalSort_CyclicDependency_Fails`.
- [x] Add `RouterCyclicDependency RouterErrorCode` to the error catalog in `extension.go`.
- [x] Implement `RouterSortExtensionsByDependency(exts []Extension) ([]Extension, error)` in `extension.go`.
- [x] Implement `RouterBuildDependencyGraph(exts []Extension) (map[PortName]int, error)` as an internal helper.
- [x] Implement `RouterDetectCyclicDependency(graph map[PortName][]PortName) error` as an internal helper.
- [x] Wire `RouterSortExtensionsByDependency` at the start of `routerLoadExtensionLayer`.
- [x] Run all boot tests green with `-race`.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] Extensions declared out of slice order but with correct `Consumes()` declarations boot successfully.
- [x] Cyclic dependency is caught at boot time with `RouterCyclicDependency` error.
- [x] All existing boot tests remain green.

## Phase 13 — Restricted/Private Port Resolution (v0.9)

**Goal:** Consumer-identity-based access control for selected ports. Trust policy in mutable wiring only.

- [x] Add `PortAccessDenied RouterErrorCode` to the error catalog in `extension.go`.
- [x] Create `internal/tests/router/restricted_test.go`.
- [x] Write tests red first:
  - [x] `TestRestricted_TrustedConsumer_Resolves`.
  - [x] `TestRestricted_UntrustedConsumer_AccessDenied`.
  - [x] `TestRestricted_UnrestrictedPort_AlwaysResolvable`.
  - [x] `TestRestricted_TrustPolicy_InMutableWiringOnly`.
- [x] Implement `RouterResolveRestrictedPort(port PortName, consumerID string) (Provider, error)` in `registry.go`.
- [x] Implement `RouterRegisterPortRestriction(port PortName, allowedConsumerIDs []string) error` — called from mutable wiring, not from frozen router code.
- [x] Implement `RouterCheckPortConsumerAccess(port PortName, consumerID string) bool` as an internal helper.
- [x] Expose restriction registration surface through the `Registry` handle to keep it boot-time only.
- [x] Make restricted port tests green.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] `TestRestricted_*` tests green.
- [x] Unrestricted ports remain globally resolvable via `RouterResolveProvider`.
- [x] `PortAccessDenied` is a `RouterError` with the consumer ID and port name included in message.
- [x] Trust policy wiring is exclusively in mutable files.

## Phase 14 — Optional Extension Coverage (v0.9)

**Goal:** Prove the optional extension layer with a real documented pattern and full test coverage. The `optionalExtensions` slice in `ext/optional_extensions.go` is currently empty.

- [x] Add red tests in `internal/tests/router/boot_test.go` (already partially covered by cross-layer tests — add the missing optional-only lifecycle cases):
  - [x] `TestBoot_OptionalExtension_RegistersCapability`.
  - [x] `TestBoot_OptionalExtension_CapabilityConsumedByApplication`.
  - [x] `TestBoot_OptionalLayer_NoExtensions_BootStillSucceeds`.
- [x] Add a documented example optional extension under `internal/router/ext/` that expands router capabilities (e.g., a telemetry stub that registers a `PortTelemetry` provider before application extensions boot).
  - [x] Add `PortTelemetry PortName = "telemetry"` to `ports.go`.
  - [x] Add `PortTelemetry` case to `RouterValidatePortName`.
  - [x] Create `internal/router/ext/telemetry_example.go` — implements `router.Extension`, returns `Required() = false`, exposes `RouterProvideOptionalCapability` as function-level doc.
  - [x] Wire the example extension into `optionalExtensions` in `ext/optional_extensions.go`.
- [x] Make all optional-layer tests green.
- [x] Run `go run ./cmd/policycheck`.

Completion criteria:
- [x] Optional extension lifecycle fully exercised in test suite.
- [x] Example extension serves as canonical reference for host project optional-layer additions.
- [x] `optionalExtensions` is non-empty in the example router bundle.
- [x] The router test suite remains green.

## End of Session (v0.9)

- [x] Record the phase reached.
- [x] Record what is green, what is still red, and what the next session should do first.
- [x] Stop after the active phase is complete instead of starting the next one by default.

Session note (v0.9 completed):
- Completed through Phase 14: Optional Extension Coverage.
- Green: `go test -tags test ./internal/tests/router/... -v`.
- Green: `go run ./cmd/policycheck`.
- Nothing is red.
- The router implementation is now complete. Next steps depend on project requirements.

Session note (v0.8 baseline):
- Completed through Phase 10: Router Lock Integrity Tooling.
- Green: `go test -tags test ./internal/tests/router/... -count=1`.
- Green: `go test ./cmd/policycheck/...`.
- Green: `go run ./cmd/policycheck`.
- Green: `go test ./internal/tests/router/tools/wrlk -count=1`.
- Green: `go run ./internal/router/tools/wrlk --root . lock verify`.
- Note: `extensions.go` and `optional_extensions.go` moved to `internal/router/ext/` sub-package to break a circular import.
- Note: `optionalExtensions` slice is currently empty — Phase 14 addresses this.
- v0.9 next session should start with Phase 11 (portgen) — it has no dependencies on other v0.9 phases.
