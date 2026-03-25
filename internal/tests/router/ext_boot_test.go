package router_test

import (
	"context"
	"sort"

	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/ext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *RouterSuite) TestBootExtensions_ProfileMismatch_FailsBeforeBoot() {
	s.T().Setenv("WRLK_ENV", "prod")
	s.T().Setenv("ROUTER_PROFILE", "dev")

	warnings, err := ext.RouterBootExtensions(context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RouterEnvironmentMismatch)
	assertRegistryNotBooted(s.T(), router.PortPrimary)
}

func (s *RouterSuite) TestBootExtensions_ProdAllowAny_FailsBeforeBoot() {
	s.T().Setenv("WRLK_ENV", "prod")
	s.T().Setenv("ROUTER_ALLOW_ANY", "true")

	warnings, err := ext.RouterBootExtensions(context.Background())

	require.Error(s.T(), err)
	assert.Nil(s.T(), warnings)
	requireRouterErrorCode(s.T(), err, router.RouterProfileInvalid)
	assertRegistryNotBooted(s.T(), router.PortPrimary)
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

func (s *RouterSuite) TestBuildExtensionBundle_ApplicationExtensionsStartEmpty() {
	_, applicationBundle := ext.RouterBuildExtensionBundle()

	assert.Empty(s.T(), applicationBundle)
}

// TestBuildExtensionBundle_OptionalExtensionsArePackageLevel verifies that the
// optional extensions slice is wired through internal/router/ext/extensions/<name>/
// sub-packages rather than inline types. It asserts the expected count and that the
// sole optional extension declares and registers router.PortCLIStyle.
func (s *RouterSuite) TestBuildExtensionBundle_OptionalExtensionsArePackageLevel() {
	optionalBundle, _ := ext.RouterBuildExtensionBundle()

	require.Len(s.T(), optionalBundle, 1, "exactly one optional router capability extension expected")

	capabilityExt := optionalBundle[0]
	require.NotNil(s.T(), capabilityExt)

	// Validates Provides() declaration matches what is actually registered.
	declared := routerPortNamesSorted(capabilityExt.Provides())
	require.Equal(s.T(), []router.PortName{router.PortCLIStyle}, declared)

	registered, err := router.RouterCollectProvidedPorts(capabilityExt)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), declared, routerPortNamesSorted(registered))
}

func routerPortNamesSorted(ports []router.PortName) []router.PortName {
	sortedPorts := append([]router.PortName(nil), ports...)
	sort.Slice(sortedPorts, func(i, j int) bool {
		return sortedPorts[i] < sortedPorts[j]
	})

	return sortedPorts
}
