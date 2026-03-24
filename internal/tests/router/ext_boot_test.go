package router_test

import (
	"context"
	"sort"

	"policycheck/internal/router"
	"policycheck/internal/router/ext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *RouterSuite) TestBootExtensions_ProfileMismatch_FailsBeforeBoot() {
	s.T().Setenv("POLICYCHECK_ENV", "prod")
	s.T().Setenv("ROUTER_PROFILE", "dev")

	warnings, err := ext.RouterBootExtensions(context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RouterEnvironmentMismatch)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBootExtensions_ProdAllowAny_FailsBeforeBoot() {
	s.T().Setenv("POLICYCHECK_ENV", "prod")
	s.T().Setenv("ROUTER_ALLOW_ANY", "true")

	warnings, err := ext.RouterBootExtensions(context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RouterProfileInvalid)
	assertRegistryNotBooted(s.T(), router.PortConfig)
}

func (s *RouterSuite) TestBuildExtensionBundle_ProvidesMatchesRegistration() {
	optionalBundle, applicationBundle := ext.RouterBuildExtensionBundle()
	allExtensions := append(optionalBundle, applicationBundle...)

	for _, extensionInstance := range allExtensions {
		providedPorts := routerPortNamesSorted(extensionInstance.Provides())

		registeredPorts, err := router.RouterCollectProvidedPorts(extensionInstance)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), providedPorts, routerPortNamesSorted(registeredPorts))
	}
}

func routerPortNamesSorted(ports []router.PortName) []router.PortName {
	sortedPorts := append([]router.PortName(nil), ports...)
	sort.Slice(sortedPorts, func(i, j int) bool {
		return sortedPorts[i] < sortedPorts[j]
	})

	return sortedPorts
}
