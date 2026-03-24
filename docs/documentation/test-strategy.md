# Router Test Strategy

This document describes the test strategy and test coverage for the router codebase in `internal/router/`.

## Test Location

All router tests are located in `internal/tests/router/`:

```
internal/tests/router/
├── helpers_test.go          # MockExtension, RouterSuite setup
├── registration_test.go     # Port registration tests (Phase 1)
├── resolution_test.go      # Provider resolution tests (Phase 2)
├── boot_test.go            # Router boot lifecycle tests (Phase 3)
├── restricted_test.go      # Restricted port access control tests
├── ext_boot_test.go        # Extension boot policy tests
├── benchmark_test.go       # Performance benchmarks
└── tools/wrlk/             # CLI tool tests
    ├── main_test.go        # lock verify/update/restore tests
    ├── live_test.go        # live run session tests
    └── ext_test.go         # wrlk ext add tests
```

## Test Framework

The router test suite uses:
- `testify/suite` for test organization
- `testify/assert` for non-fatal assertions
- `testify/require` for fatal assertions
- Standard library `testing` for benchmarks

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
)
```

## MockExtension Design

The `MockExtension` is the core test helper that simulates extension behavior:

```go
type MockExtension struct {
    mock.Mock
    
    // Controls boot behavior
    BootError         error
    AsyncDelay        time.Duration
    IsRequired        bool
    
    // Controls Consumes() return value  
    ConsumedPorts     []router.PortName
    ProvidedPorts      []router.PortName
    
    // Controls what this extension registers
    RegistersPort     router.PortName
    RegistersProvider router.Provider
    RegistrationCalls *int  // For tracking call counts
}
```

### Interface Implementations

- `MockExtension` - Base extension mock
- `MockAsyncExtension` - Async extension with delay simulation
- `MockErrorFormattingExtension` - Custom error formatter

### Factory Functions

```go
func requiredExtension(port router.PortName, provider router.Provider) *MockExtension
func optionalExtension(port router.PortName, provider router.Provider) *MockExtension
func failingRequiredExtension(err error) *MockExtension
func failingOptionalExtension(err error) *MockExtension
func asyncExtension(port router.PortName, delay time.Duration) *MockAsyncExtension
func unknownPortExtension() *MockExtension
```

## Test Phases

### Phase 1: Registration Tests (`registration_test.go`)

Tests the port whitelist and registration contract:

| Test                                   | What It Verifies                                              |
| -------------------------------------- | ------------------------------------------------------------- |
| `TestPortUnknown_IncludesPortName`     | Undeclared ports return PortUnknown with port name in message |
| `TestPortDuplicate_SecondFails`        | Duplicate registration returns PortDuplicate                  |
| `TestInvalidProvider_NilRejected`      | Nil providers are rejected with InvalidProvider               |
| `TestValidRegistration_Passes`         | Valid registrations succeed                                   |
| `TestAllDeclaredPorts_RegisterCleanly` | All declared ports register cleanly                           |

### Phase 2: Resolution Tests (`resolution_test.go`)

Tests the atomic publication model and resolution:

| Test                                       | What It Verifies                                 |
| ------------------------------------------ | ------------------------------------------------ |
| `TestRegistryNotBooted_BeforeBoot`         | Resolution before boot returns RegistryNotBooted |
| `TestPortNotFound_IncludesPortName`        | Unregistered ports return PortNotFound with name |
| `TestResolve_ReturnsCorrectProvider`       | Exact provider instance is returned              |
| `TestResolve_ImmutableAfterBoot`           | Second boot returns MultipleInitializations      |
| `TestResolve_ConcurrentReads_NoRace`       | 100 concurrent reads don't race (run with -race) |
| `TestPortContractMismatch_StructuredError` | PortContractMismatch is properly structured      |

### Phase 3: Boot Tests (`boot_test.go`)

Tests full boot lifecycle including topological sort:

| Test                                                                  | What It Verifies                                  |
| --------------------------------------------------------------------- | ------------------------------------------------- |
| `TestBoot_HappyPath`                                                  | Clean boot with required extensions succeeds      |
| `TestBoot_EmptyExtensionSlices`                                       | Empty extension slices succeed                    |
| `TestBoot_RequiredFails_AbortsAll`                                    | Required extension failure aborts boot            |
| `TestBoot_OptionalFails_Continues`                                    | Optional extension failure continues with warning |
| `TestBoot_AsyncCompletes_BeforeDeadline`                              | Async init completes within context deadline      |
| `TestBoot_AsyncTimeout`                                               | Async init timeout returns AsyncInitTimeout       |
| `TestBoot_ContextCancelled_StopsAsync`                                | Context cancellation stops async init             |
| `TestBoot_DependencyOrderViolation_MessageFormat`                     | Dependency error includes mandated message        |
| `TestBoot_OptionalLayer_BootsBeforeApplication`                       | Optional layer boots before application layer     |
| `TestBoot_CrossLayer_DependencyOrderViolation`                        | Cross-layer dependency violation detected         |
| `TestBoot_ErrorFormatter_UsedForThatExtension`                        | Custom error formatter applied                    |
| `TestBoot_ErrorFormatter_CannotDowngradeFatal`                        | Fatal errors cannot be downgraded                 |
| `TestBoot_TopologicalSort_ResolvesOutOfOrderSlice`                    | Topological sort reorders correctly               |
| `TestBoot_TopologicalSort_MultiLayerChain`                            | Multi-layer dependency chain sorts correctly      |
| `TestBoot_TopologicalSort_CyclicDependency_Fails`                     | Cyclic dependencies are detected                  |
| `TestBoot_DependencyGraph_DoesNotDoubleExecuteRegistration`           | Extensions registered once                        |
| `TestBoot_DependencyGraph_DetectsDuplicateProvidesBeforeRegistration` | Duplicate port detection before registration      |
| `TestBoot_NilExtension_IsIgnored`                                     | Nil extensions in slice are skipped               |
| `TestBoot_OptionalExtension_RegistersCapability`                      | Optional extensions can register providers        |
| `TestBoot_OptionalExtension_CapabilityConsumedByApplication`          | Application can consume optional layer ports      |
| `TestBoot_OptionalLayer_NoExtensions_BootStillSucceeds`               | Empty optional layer doesn't break boot           |

### Restricted Port Tests (`restricted_test.go`)

Tests consumer identity-based access control:

| Test                                               | What It Verifies                             |
| -------------------------------------------------- | -------------------------------------------- |
| `TestRestricted_TrustedConsumer_Resolves`          | Trusted consumer can resolve restricted port |
| `TestRestricted_UntrustedConsumer_AccessDenied`    | Untrusted consumer gets PortAccessDenied     |
| `TestRestricted_EmptyConsumerID_AccessDenied`      | Empty consumer ID denied                     |
| `TestRestricted_AnyConsumer_WildcardResolves`      | "Any" wildcard allows all consumers          |
| `TestRestricted_UnrestrictedPort_AlwaysResolvable` | Unrestricted ports always resolvable         |
| `TestRestricted_TrustPolicy_InMutableWiringOnly`   | Trust policy defined in mutable wiring       |

### Extension Boot Policy Tests (`ext_boot_test.go`)

Tests environment and policy enforcement at boot:

| Test                                                         | What It Verifies                              |
| ------------------------------------------------------------ | --------------------------------------------- |
| `TestBootExtensions_ProfileMismatch_FailsBeforeBoot`         | WRLK_ENV mismatch with ROUTER_PROFILE fails   |
| `TestBootExtensions_ProdAllowAny_FailsBeforeBoot`            | ROUTER_ALLOW_ANY in prod fails                |
| `TestBuildExtensionBundle_ProvidesMatchesRegistration`       | Extension bundle provides match registrations |
| `TestBuildExtensionBundle_OptionalExtensionsArePackageLevel` | Optional extensions are package-level         |

### Benchmarks (`benchmark_test.go`)

Performance validation tests:

| Benchmark                       | What It Measures                               |
| ------------------------------- | ---------------------------------------------- |
| `BenchmarkRouterResolve`        | Lock-free atomic.Pointer resolution throughput |
| `BenchmarkRouterResolveRWMutex` | RWMutex-based resolution for comparison        |

Run benchmarks with:
```bash
go test -tags test -bench=. -benchtime=3s ./internal/tests/router/...
```

## CLI Tool Tests

### Lock Tests (`tools/wrlk/main_test.go`)

| Test                                          | What It Verifies                        |
| --------------------------------------------- | --------------------------------------- |
| `TestWrlkLockVerifyWorkflow`                  | lock verify detects checksum mismatch   |
| `TestWrlkLockUpdateWorkflow`                  | lock update rewrites lock file          |
| `TestWrlkLockRestoreWorkflow`                 | lock restore recovers previous snapshot |
| `TestWrlkLockRestore_MissingSnapshot_Fails`   | Missing snapshot returns error          |
| `TestWrlkGuideCommand_PrintsOperationalGuide` | guide command outputs operational guide |
| `TestWrlkHelpFlag_PrintsTopLevelUsage`        | --help prints usage                     |
| `TestWrlkLockHelp_PrintsLockUsage`            | lock --help prints lock subcommands     |
| `TestWrlkAddHelp_PrintsAddUsage`              | add --help prints add usage             |
| `TestWrlkLiveHelp_PrintsLiveUsage`            | live --help prints live usage           |
| `TestWrlkLiveRunHelp_PrintsRunUsage`          | live run --help prints run usage        |

### Live Session Tests (`tools/wrlk/live_test.go`)

| Test                                            | What It Verifies                          |
| ----------------------------------------------- | ----------------------------------------- |
| `TestLive_Run_AllParticipantsSucceed_ExitsZero` | All success reports exit 0                |
| `TestLive_Run_OneParticipantFails_ExitsNonZero` | Any failure report exits non-zero         |
| `TestLive_Run_UnknownParticipant_Rejected`      | Unknown participant rejected with 400     |
| `TestLive_Run_DuplicateParticipant_Rejected`    | Duplicate participant rejected with 400   |
| `TestLive_Run_Timeout_IsBug`                    | Timeout is classified as verification bug |
| `TestLive_ReportPath_WrongMethod_NotFound`      | GET to /report returns 404                |
| `TestLive_ParseOptions_RequiresExpect`          | --expect is required                      |
| `TestLive_Run_WrongSubcommand_Rejected`         | Unknown subcommand rejected               |

### Extension Add Tests (`tools/wrlk/ext_test.go`)

| Test                                   | What It Verifies                            |
| -------------------------------------- | ------------------------------------------- |
| `TestExtAdd_CreatesDocGo`              | doc.go created at correct path              |
| `TestExtAdd_CreatesExtensionGo`        | extension.go created with correct structure |
| `TestExtAdd_SplicesOptionalExtensions` | optional_extensions.go updated with import  |
| `TestExtAdd_DryRun_NoWrite`            | --dry-run doesn't write files               |
| `TestExtAdd_DuplicateName_Fails`       | Duplicate extension name fails              |
| `TestExtAdd_MissingName_Fails`         | Missing --name fails                        |
| `TestExtAdd_InvalidName_Fails`         | Invalid names are rejected                  |
| `TestExtAdd_WritesSnapshot`            | Snapshot written before mutation            |
| `TestExtAdd_HelpFlag_PrintsUsage`      | --help prints usage                         |
| `TestExtHelp_PrintsExtUsage`           | ext --help prints subcommands               |

## Running the Tests

```bash
# All router tests
go test -tags test ./internal/tests/router/... -race -v

# Specific test file
go test -tags test ./internal/tests/router/... -run TestRegistration -race -v

# CLI tool tests only
go test -tags test ./internal/tests/router/tools/wrlk/... -race -v

# Benchmarks
go test -tags test -bench=. -benchtime=3s ./internal/tests/router/...
```

The `-tags test` flag is required to expose the `RouterResetForTest()` helper in `registry.go`.

## Assert/Require Decision Rule

```
RouterBootExtensions fails unexpectedly         → require
RouterResolveProvider fails before type assert  → require
Required extension boot path                    → require (Required() == true)
Optional extension boot path                    → assert  (Required() == false)
Checking fields on a RouterError                → assert (see all field failures)
Checking error message contains port name       → assert (message format, not fatal)
Checking warning list length + contents         → assert (both failures useful together)
```

## What This Document Covers

- Router runtime tests (`internal/tests/router/*.go`)
- CLI tool tests (`internal/tests/router/tools/wrlk/*.go`)

## What This Document Does NOT Cover

- Real adapter implementations (adapter tests live in `internal/tests/adapters/`)
- Integration tests (router + real adapters booted together)
- router.lock enforcement (host tooling or wrlk CLI concern)

## Related Documents

- [API Reference](api-reference.md) - Router API documentation
- [Usage Guide](usage.md) - How to use the router
- [Troubleshooting](troubleshooting.md) - Common errors and solutions
- `docs/router/router-test-strategy.md` - Original test strategy document (v0.8.0)