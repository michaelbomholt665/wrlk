package router_test

import (
	"context"

	"github.com/michaelbomholt665/wrlk/internal/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *RouterSuite) TestPortUnknown_IncludesPortName() {
	_, err := router.RouterLoadExtensions(nil, []router.Extension{
		unknownPortExtension(),
	}, context.Background())

	require.Error(s.T(), err)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortUnknown, routerErr.Code)
	assert.Contains(s.T(), err.Error(), "unknown_port")
}

func (s *RouterSuite) TestPortDuplicate_SecondFails() {
	firstProvider := struct{ Name string }{Name: "first"}
	secondProvider := struct{ Name string }{Name: "second"}

	_, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, firstProvider),
		requiredExtension(router.PortPrimary, secondProvider),
	}, context.Background())

	require.Error(s.T(), err)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortDuplicate, routerErr.Code)
	assert.Contains(s.T(), err.Error(), "primary")
}

func (s *RouterSuite) TestInvalidProvider_NilRejected() {
	_, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, nil),
	}, context.Background())

	require.Error(s.T(), err)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.InvalidProvider, routerErr.Code)
}

func (s *RouterSuite) TestValidRegistration_Passes() {
	expectedProvider := struct{ Name string }{Name: "config-provider"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), expectedProvider, provider)
}

func (s *RouterSuite) TestAllDeclaredPorts_RegisterCleanly() {
	configProvider := struct{ Name string }{Name: "primary"}
	walkProvider := struct{ Name string }{Name: "secondary"}
	scannerProvider := struct{ Name string }{Name: "tertiary"}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, configProvider),
		requiredExtension(router.PortSecondary, walkProvider),
		requiredExtension(router.PortTertiary, scannerProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	provider, resolveErr := router.RouterResolveProvider(router.PortPrimary)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), configProvider, provider)

	provider, resolveErr = router.RouterResolveProvider(router.PortSecondary)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), walkProvider, provider)

	provider, resolveErr = router.RouterResolveProvider(router.PortTertiary)
	require.NoError(s.T(), resolveErr)
	assert.Equal(s.T(), scannerProvider, provider)
}
