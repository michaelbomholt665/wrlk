package router_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"policycheck/internal/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireRouterErrorCode(
	t *testing.T,
	err error,
	expectedCode router.RouterErrorCode,
) {
	t.Helper()

	var routerErr *router.RouterError
	require.ErrorAs(t, err, &routerErr)
	assert.Equal(t, expectedCode, routerErr.Code)
}

func assertRegistryNotBooted(t *testing.T, port router.PortName) {
	t.Helper()

	provider, err := router.RouterResolveProvider(port)
	require.Error(t, err)
	assert.Nil(t, provider)

	var routerErr *router.RouterError
	require.ErrorAs(t, err, &routerErr)
	assert.Equal(t, router.RegistryNotBooted, routerErr.Code)
}

func (s *RouterSuite) TestBoot_HappyPath() {
	configProvider := &configProviderStub{path: "isr.toml"}
	walkProvider := struct{ Name string }{Name: "walk"}
	scannerProvider := struct{ Name string }{Name: "scanner"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, configProvider),
		requiredExtension(router.PortWalk, walkProvider),
		requiredExtension(router.PortScanner, scannerProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), configProvider, provider)

	provider, resolveErr = router.RouterResolveProvider(router.PortWalk)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), walkProvider, provider)

	provider, resolveErr = router.RouterResolveProvider(router.PortScanner)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), scannerProvider, provider)
}

func (s *RouterSuite) TestBoot_EmptyExtensionSlices() {
	warnings, err := router.RouterLoadExtensions(nil, nil, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), resolveErr, &routerErr)
	assert.Equal(s.T(), router.PortNotFound, routerErr.Code)
}

func (s *RouterSuite) TestBoot_RequiredFails_AbortsAll() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		failingRequiredExtension(errors.New("required boot failed")),
		requiredExtension(router.PortConfig, &configProviderStub{path: "ignored.toml"}),
	}, context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RequiredExtensionFailed)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_OptionalFails_Continues() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		failingOptionalExtension(errors.New("optional boot failed")),
		requiredExtension(router.PortConfig, &configProviderStub{path: "isr.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	require.Len(s.T(), warnings, 1)
	requireRouterErrorCode(s.T(), warnings[0], router.OptionalExtensionFailed)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_AsyncCompletes_BeforeDeadline() {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		asyncExtension(router.PortConfig, 10*time.Millisecond),
	}, ctx)

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_AsyncTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		asyncExtension(router.PortConfig, 100*time.Millisecond),
	}, ctx)

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.AsyncInitTimeout)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_ContextCancelled_StopsAsync() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		asyncExtension(router.PortConfig, 100*time.Millisecond),
	}, ctx)

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.AsyncInitTimeout)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_DependencyOrderViolation_MessageFormat() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&MockExtension{
			IsRequired:    true,
			ConsumedPorts: []router.PortName{router.PortConfig},
			RegistersPort: router.PortWalk,
			RegistersProvider: struct{ Name string }{
				Name: "walk",
			},
		},
	}, context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.DependencyOrderViolation)
	assert.Contains(s.T(), err.Error(), "config")
	assert.Contains(s.T(), err.Error(), "If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong.")
	assert.Contains(s.T(), err.Error(), "Move the providing extension higher up in the correct extensions slice.")
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_OptionalLayer_BootsBeforeApplication() {
	warnings, err := router.RouterLoadExtensions(
		[]router.Extension{
			requiredExtension(router.PortConfig, &configProviderStub{path: "isr.toml"}),
		},
		[]router.Extension{
			&MockExtension{
				IsRequired:    true,
				ConsumedPorts: []router.PortName{router.PortConfig},
				RegistersPort: router.PortWalk,
				RegistersProvider: struct{ Name string }{
					Name: "walk",
				},
			},
		},
		context.Background(),
	)

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortWalk)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_CrossLayer_DependencyOrderViolation() {
	warnings, err := router.RouterLoadExtensions(
		nil,
		[]router.Extension{
			&MockExtension{
				IsRequired:    true,
				ConsumedPorts: []router.PortName{router.PortConfig},
				RegistersPort: router.PortWalk,
				RegistersProvider: struct{ Name string }{
					Name: "walk",
				},
			},
		},
		context.Background(),
	)

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.DependencyOrderViolation)
	assert.Contains(s.T(), err.Error(), "config")
	assertRegistryNotBooted(s.T(), router.PortWalk)
}

func (s *RouterSuite) TestBoot_ErrorFormatter_UsedForThatExtension() {
	bootErr := errors.New("formatter input")
	ext := &MockErrorFormattingExtension{
		MockExtension: MockExtension{
			BootError:  bootErr,
			IsRequired: true,
		},
		Formatter: func(err error) error {
			return fmt.Errorf("formatted extension error: %w", err)
		},
	}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	assert.ErrorContains(s.T(), err, "formatted extension error")
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_ErrorFormatter_CannotDowngradeFatal() {
	ext := &MockErrorFormattingExtension{
		MockExtension: MockExtension{
			BootError:  errors.New("fatal boot error"),
			IsRequired: true,
		},
		Formatter: func(err error) error {
			return &router.RouterError{
				Code: router.OptionalExtensionFailed,
				Err:  err,
			}
		},
	}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RequiredExtensionFailed)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBoot_ErrorFormatter_DoesNotMutateReturnedRouterError() {
	formattedErr := &router.RouterError{
		Code: router.OptionalExtensionFailed,
		Err:  errors.New("formatted failure"),
	}
	ext := &MockErrorFormattingExtension{
		MockExtension: MockExtension{
			BootError:  errors.New("fatal boot error"),
			IsRequired: true,
		},
		Formatter: func(err error) error {
			return formattedErr
		},
	}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RequiredExtensionFailed)
	assert.Equal(s.T(), router.OptionalExtensionFailed, formattedErr.Code)
}

func (s *RouterSuite) TestBoot_TopologicalSort_ResolvesOutOfOrderSlice() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&MockExtension{
			IsRequired:    true,
			ConsumedPorts: []router.PortName{router.PortConfig},
			RegistersPort: router.PortWalk,
			RegistersProvider: struct{ Name string }{
				Name: "walk",
			},
		},
		requiredExtension(router.PortConfig, &configProviderStub{path: "isr.toml"}),
	}, context.Background())

	require.NoError(s.T(), err, "Topological sort should reorder the slice so config boots before walk")
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortWalk)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_TopologicalSort_MultiLayerChain() {
	// A provides Config
	// B provides Walk, consumes Config
	// C provides Scanner, consumes Walk
	// Order given: C, A, B -> Sort should make it A, B, C
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&MockExtension{ // C
			IsRequired:    true,
			ConsumedPorts: []router.PortName{router.PortWalk},
			RegistersPort: router.PortScanner,
			RegistersProvider: struct{ Name string }{
				Name: "scanner",
			},
		},
		requiredExtension(router.PortConfig, &configProviderStub{path: "isr.toml"}), // A
		&MockExtension{ // B
			IsRequired:    true,
			ConsumedPorts: []router.PortName{router.PortConfig},
			RegistersPort: router.PortWalk,
			RegistersProvider: struct{ Name string }{
				Name: "walk",
			},
		},
	}, context.Background())

	require.NoError(s.T(), err, "Topological sort should sequence A -> B -> C correctly")
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortScanner)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_TopologicalSort_CyclicDependency_Fails() {
	// A provides Config, consumes Walk
	// B provides Walk, consumes Config
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&MockExtension{
			IsRequired:        true,
			ConsumedPorts:     []router.PortName{router.PortWalk},
			RegistersPort:     router.PortConfig,
			RegistersProvider: struct{ Name string }{},
		},
		&MockExtension{
			IsRequired:        true,
			ConsumedPorts:     []router.PortName{router.PortConfig},
			RegistersPort:     router.PortWalk,
			RegistersProvider: struct{ Name string }{},
		},
	}, context.Background())

	require.Error(s.T(), err, "Cyclic dependency must be detected and fail the boot")
	assert.Nil(s.T(), warnings)

	requireRouterErrorCode(s.T(), err, router.RouterCyclicDependency)
}

func (s *RouterSuite) TestBoot_DependencyGraph_DoesNotDoubleExecuteRegistration() {
	registrationCalls := 0
	ext := &MockExtension{
		IsRequired:        true,
		RegistersPort:     router.PortConfig,
		RegistersProvider: &configProviderStub{path: "isr.toml"},
		RegistrationCalls: &registrationCalls,
	}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)
	assert.Equal(s.T(), 1, registrationCalls)
}

func (s *RouterSuite) TestBoot_DependencyGraph_DetectsDuplicateProvidesBeforeRegistration() {
	firstCalls := 0
	secondCalls := 0
	firstExt := &MockExtension{
		IsRequired:        true,
		RegistersPort:     router.PortConfig,
		RegistersProvider: &configProviderStub{path: "first.toml"},
		RegistrationCalls: &firstCalls,
	}
	secondExt := &MockExtension{
		IsRequired:        true,
		RegistersPort:     router.PortConfig,
		RegistersProvider: &configProviderStub{path: "second.toml"},
		RegistrationCalls: &secondCalls,
	}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		firstExt,
		secondExt,
	}, context.Background())

	require.Nil(s.T(), warnings)
	require.Error(s.T(), err)
	requireRouterErrorCode(s.T(), err, router.PortDuplicate)
	assert.Equal(s.T(), 0, firstCalls)
	assert.Equal(s.T(), 0, secondCalls)
}

func (s *RouterSuite) TestBoot_NilExtension_IsIgnored() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		nil,
		requiredExtension(router.PortConfig, &configProviderStub{path: "isr.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)
}

func (s *RouterSuite) TestBoot_OptionalExtension_RegistersCapability() {
	warnings, err := router.RouterLoadExtensions(
		[]router.Extension{
			&MockExtension{
				IsRequired:        false,
				RegistersPort:     router.PortConfig,
				RegistersProvider: struct{ Name string }{Name: "optional"},
			},
		},
		nil,
		context.Background(),
	)

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_OptionalExtension_CapabilityConsumedByApplication() {
	warnings, err := router.RouterLoadExtensions(
		[]router.Extension{
			&MockExtension{
				IsRequired:        false,
				RegistersPort:     router.PortWalk,
				RegistersProvider: struct{ Name string }{Name: "optional-walk"},
			},
		},
		[]router.Extension{
			&MockExtension{
				IsRequired:        true,
				ConsumedPorts:     []router.PortName{router.PortWalk},
				RegistersPort:     router.PortConfig,
				RegistersProvider: struct{ Name string }{Name: "application-config"},
			},
		},
		context.Background(),
	)

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestBoot_OptionalLayer_NoExtensions_BootStillSucceeds() {
	warnings, err := router.RouterLoadExtensions(
		[]router.Extension{},
		[]router.Extension{
			&MockExtension{
				IsRequired:        true,
				RegistersPort:     router.PortWalk,
				RegistersProvider: struct{ Name string }{Name: "application-walk"},
			},
		},
		context.Background(),
	)

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortWalk)
	require.NoError(s.T(), resolveErr)
	assert.NotNil(s.T(), provider)
}
