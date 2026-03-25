package router_test

import (
	"context"

	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/capabilities"
	"github.com/michaelbomholt665/wrlk/internal/router/ext/extensions/prettystyle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cliStylerStub struct{}

func (s *cliStylerStub) StyleText(_ string, input string) (string, error) {
	return input, nil
}

func (s *cliStylerStub) StyleTable(_ []string, _ [][]string) (string, error) {
	return "table", nil
}

func (s *RouterSuite) TestResolveCLIOutputStyler_Succeeds() {
	expectedProvider := &cliStylerStub{}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIStyle, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), expectedProvider, styler)
}

func (s *RouterSuite) TestResolveCLIOutputStyler_RegistryNotBooted() {
	styler, err := capabilities.ResolveCLIOutputStyler()

	require.Error(s.T(), err)
	assert.Nil(s.T(), styler)
	requireRouterErrorCode(s.T(), err, router.RegistryNotBooted)
}

func (s *RouterSuite) TestResolveCLIOutputStyler_PortNotFound() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, &primaryProviderStub{path: "test-config.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), styler)
	requireRouterErrorCode(s.T(), resolveErr, router.PortNotFound)
}

func (s *RouterSuite) TestResolveCLIOutputStyler_PortContractMismatch() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIStyle, struct{ Name string }{Name: "wrong-provider"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), styler)
	requireRouterErrorCode(s.T(), resolveErr, router.PortContractMismatch)
}

func (s *RouterSuite) TestCLIStyleProvider_StyleText() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	rendered, styleErr := styler.StyleText(capabilities.TextKindSuccess, "ready")
	require.NoError(s.T(), styleErr)
	assert.Contains(s.T(), rendered, "ready")
	assert.Contains(s.T(), rendered, "\x1b[")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleTable() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	rendered, styleErr := styler.StyleTable(
		[]string{"Name", "Status"},
		[][]string{{"scanner", "ready"}},
	)
	require.NoError(s.T(), styleErr)
	assert.Contains(s.T(), rendered, "Name")
	assert.Contains(s.T(), rendered, "Status")
	assert.Contains(s.T(), rendered, "scanner")
	assert.Contains(s.T(), rendered, "ready")
	assert.Contains(s.T(), rendered, "+")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleTable_MalformedInput() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	rendered, styleErr := styler.StyleTable(
		[]string{"Name", "Status"},
		[][]string{{"scanner"}},
	)
	require.Error(s.T(), styleErr)
	assert.Empty(s.T(), rendered)
	assert.Contains(s.T(), styleErr.Error(), "row 0 width 1 does not match header width 2")
}
