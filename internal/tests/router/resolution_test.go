package router_test

import (
	"context"
	"policycheck/internal/router"
	"sync"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configContract interface {
	ConfigPath() string
}

type configProviderStub struct {
	path string
}

func (c configProviderStub) ConfigPath() string {
	return c.path
}

func (s *RouterSuite) TestRegistryNotBooted_BeforeBoot() {
	provider, err := router.RouterResolveProvider(router.PortConfig)

	require.Error(s.T(), err)
	assert.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.RegistryNotBooted, routerErr.Code)
}

func (s *RouterSuite) TestPortNotFound_IncludesPortName() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, configProviderStub{path: "isr.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortWalk)

	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), resolveErr, &routerErr)
	assert.Equal(s.T(), router.PortNotFound, routerErr.Code)
	assert.Contains(s.T(), resolveErr.Error(), "walk")
}

func (s *RouterSuite) TestResolve_ReturnsCorrectProvider() {
	expectedProvider := &configProviderStub{path: "isr.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)

	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), expectedProvider, provider)
}

func (s *RouterSuite) TestResolve_ImmutableAfterBoot() {
	firstProvider := configProviderStub{path: "first.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, firstProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	secondWarnings, secondErr := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, configProviderStub{path: "second.toml"}),
	}, context.Background())

	require.Error(s.T(), secondErr)
	assert.Nil(s.T(), secondWarnings)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), secondErr, &routerErr)
	assert.Equal(s.T(), router.MultipleInitializations, routerErr.Code)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), firstProvider, provider)
}

func (s *RouterSuite) TestResolve_ConcurrentReads_NoRace() {
	expectedProvider := &configProviderStub{path: "isr.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	const goroutines = 100

	results := make(chan router.Provider, goroutines)
	errorsCh := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
			if resolveErr != nil {
				errorsCh <- resolveErr
				return
			}

			results <- provider
		}()
	}

	wg.Wait()
	close(results)
	close(errorsCh)

	for resolveErr := range errorsCh {
		require.NoError(s.T(), resolveErr)
	}

	for provider := range results {
		assert.Equal(s.T(), expectedProvider, provider)
	}
}

func (s *RouterSuite) TestPortContractMismatch_StructuredError() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortConfig, struct{ Name string }{Name: "wrong-provider"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortConfig)
	require.NoError(s.T(), resolveErr)

	_, ok := provider.(configContract)
	require.False(s.T(), ok)

	contractErr := &router.RouterError{
		Code: router.PortContractMismatch,
		Port: router.PortConfig,
	}

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), contractErr, &routerErr)
	assert.Equal(s.T(), router.PortContractMismatch, routerErr.Code)
	assert.Equal(s.T(), router.PortConfig, routerErr.Port)
}
