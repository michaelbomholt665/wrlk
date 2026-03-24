package router_test

import (
	"context"
	"sync"

	"github.com/michaelbomholt665/wrlk/internal/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type primaryContract interface {
	ConfigPath() string
}

type primaryProviderStub struct {
	path string
}

func (c primaryProviderStub) ConfigPath() string {
	return c.path
}

func (s *RouterSuite) TestRegistryNotBooted_BeforeBoot() {
	provider, err := router.RouterResolveProvider(router.PortPrimary)

	require.Error(s.T(), err)
	assert.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.RegistryNotBooted, routerErr.Code)
}

func (s *RouterSuite) TestPortNotFound_IncludesPortName() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, primaryProviderStub{path: "test-config.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortSecondary)

	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), resolveErr, &routerErr)
	assert.Equal(s.T(), router.PortNotFound, routerErr.Code)
	assert.Contains(s.T(), resolveErr.Error(), "secondary")
}

func (s *RouterSuite) TestResolve_ReturnsCorrectProvider() {
	expectedProvider := &primaryProviderStub{path: "test-config.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)

	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), expectedProvider, provider)
}

func (s *RouterSuite) TestResolve_ImmutableAfterBoot() {
	firstProvider := primaryProviderStub{path: "first.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, firstProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	secondWarnings, secondErr := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, primaryProviderStub{path: "second.toml"}),
	}, context.Background())

	require.Error(s.T(), secondErr)
	assert.Nil(s.T(), secondWarnings)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), secondErr, &routerErr)
	assert.Equal(s.T(), router.MultipleInitializations, routerErr.Code)

	provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), firstProvider, provider)
}

func (s *RouterSuite) TestResolve_ConcurrentReads_NoRace() {
	expectedProvider := &primaryProviderStub{path: "test-config.toml"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, expectedProvider),
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

			provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)
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
		requiredExtension(router.PortPrimary, struct{ Name string }{Name: "wrong-provider"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)
	require.NoError(s.T(), resolveErr)

	_, ok := provider.(primaryContract)
	require.False(s.T(), ok)

	contractErr := &router.RouterError{
		Code: router.PortContractMismatch,
		Port: router.PortPrimary,
	}

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), contractErr, &routerErr)
	assert.Equal(s.T(), router.PortContractMismatch, routerErr.Code)
	assert.Equal(s.T(), router.PortPrimary, routerErr.Port)
}
