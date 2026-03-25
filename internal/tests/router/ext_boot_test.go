package router_test

import (
	"context"
	"sort"

	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/capabilities"
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
// optional extension set declares and registers the CLI capability ports.
func (s *RouterSuite) TestBuildExtensionBundle_OptionalExtensionsArePackageLevel() {
	optionalBundle, _ := ext.RouterBuildExtensionBundle()

	require.Len(s.T(), optionalBundle, 2, "exactly two optional router capability extensions expected")

	declaredPorts := make([]router.PortName, 0, len(optionalBundle))
	registeredPorts := make([]router.PortName, 0, len(optionalBundle))
	for _, capabilityExt := range optionalBundle {
		require.NotNil(s.T(), capabilityExt)

		declared := routerPortNamesSorted(capabilityExt.Provides())
		declaredPorts = append(declaredPorts, declared...)

		registered, err := router.RouterCollectProvidedPorts(capabilityExt)
		require.NoError(s.T(), err)
		registeredPorts = append(registeredPorts, registered...)
	}

	assert.Equal(
		s.T(),
		[]router.PortName{router.PortCLIChrome, router.PortCLIInteraction, router.PortCLIStyle},
		routerPortNamesSorted(declaredPorts),
	)
	assert.Equal(s.T(), routerPortNamesSorted(declaredPorts), routerPortNamesSorted(registeredPorts))
}

func (s *RouterSuite) TestBootExtensions_ResolvesBundledCLIProviders() {
	warnings, err := ext.RouterBootExtensions(context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	outputStyler, outputErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), outputErr)
	require.NotNil(s.T(), outputStyler)

	chromeStyler, chromeErr := capabilities.ResolveCLIChromeStyler()
	require.NoError(s.T(), chromeErr)
	require.NotNil(s.T(), chromeStyler)

	interactor, interactorErr := capabilities.ResolveCLIInteractor()
	require.NoError(s.T(), interactorErr)
	require.NotNil(s.T(), interactor)
}

func routerPortNamesSorted(ports []router.PortName) []router.PortName {
	sortedPorts := append([]router.PortName(nil), ports...)
	sort.Slice(sortedPorts, func(i, j int) bool {
		return sortedPorts[i] < sortedPorts[j]
	})

	return sortedPorts
}
