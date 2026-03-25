package router_test

import (
	"context"

	"github.com/charmbracelet/huh"
	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/capabilities"
	"github.com/michaelbomholt665/wrlk/internal/router/ext/extensions/charmcli"
	"github.com/michaelbomholt665/wrlk/internal/router/ext/extensions/prettystyle"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cliStylerStub struct{}

func (s *cliStylerStub) StyleText(_ string, input string) (string, error) {
	return input, nil
}

func (s *cliStylerStub) StyleTable(_ string, _ []string, _ [][]string) (string, error) {
	return "table", nil
}

func (s *cliStylerStub) StyleLayout(_ string, _ string, content ...string) (string, error) {
	return "layout:" + string(rune(len(content))), nil
}

type cliChromeStub struct{}

func (s *cliChromeStub) StyleText(_ string, input string) (string, error) {
	return input, nil
}

func (s *cliChromeStub) StyleLayout(_ string, title string, content ...string) (string, error) {
	return title + ":" + string(rune(len(content))), nil
}

type cliInteractorStub struct{}

func (s *cliInteractorStub) StylePrompt(_ string, _ string, _ string, _ []capabilities.Choice) (any, error) {
	return "choice", nil
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

func (s *RouterSuite) TestResolveCLIInteractor_Succeeds() {
	expectedProvider := &cliInteractorStub{}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIInteraction, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	interactor, resolveErr := capabilities.ResolveCLIInteractor()
	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), expectedProvider, interactor)
}

func (s *RouterSuite) TestResolveCLIChromeStyler_Succeeds() {
	expectedProvider := &cliChromeStub{}

	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIChrome, expectedProvider),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	chromeStyler, resolveErr := capabilities.ResolveCLIChromeStyler()
	require.NoError(s.T(), resolveErr)
	assert.Same(s.T(), expectedProvider, chromeStyler)
}

func (s *RouterSuite) TestResolveCLIOutputStyler_RegistryNotBooted() {
	styler, err := capabilities.ResolveCLIOutputStyler()

	require.Error(s.T(), err)
	assert.Nil(s.T(), styler)
	requireRouterErrorCode(s.T(), err, router.RegistryNotBooted)
}

func (s *RouterSuite) TestResolveCLIInteractor_RegistryNotBooted() {
	interactor, err := capabilities.ResolveCLIInteractor()

	require.Error(s.T(), err)
	assert.Nil(s.T(), interactor)
	requireRouterErrorCode(s.T(), err, router.RegistryNotBooted)
}

func (s *RouterSuite) TestResolveCLIChromeStyler_RegistryNotBooted() {
	chromeStyler, err := capabilities.ResolveCLIChromeStyler()

	require.Error(s.T(), err)
	assert.Nil(s.T(), chromeStyler)
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

func (s *RouterSuite) TestResolveCLIInteractor_PortNotFound() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, &primaryProviderStub{path: "test-config.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	interactor, resolveErr := capabilities.ResolveCLIInteractor()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), interactor)
	requireRouterErrorCode(s.T(), resolveErr, router.PortNotFound)
}

func (s *RouterSuite) TestResolveCLIChromeStyler_PortNotFound() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortPrimary, &primaryProviderStub{path: "test-config.toml"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	chromeStyler, resolveErr := capabilities.ResolveCLIChromeStyler()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), chromeStyler)
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

func (s *RouterSuite) TestResolveCLIInteractor_PortContractMismatch() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIInteraction, struct{ Name string }{Name: "wrong-provider"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	interactor, resolveErr := capabilities.ResolveCLIInteractor()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), interactor)
	requireRouterErrorCode(s.T(), resolveErr, router.PortContractMismatch)
}

func (s *RouterSuite) TestResolveCLIChromeStyler_PortContractMismatch() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		requiredExtension(router.PortCLIChrome, struct{ Name string }{Name: "wrong-provider"}),
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	chromeStyler, resolveErr := capabilities.ResolveCLIChromeStyler()
	require.Error(s.T(), resolveErr)
	assert.Nil(s.T(), chromeStyler)
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
	assert.Contains(s.T(), rendered, "[  OK  ]")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleText_DebugAndFatal() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	debugRendered, debugErr := styler.StyleText(capabilities.TextKindDebug, "router trace")
	require.NoError(s.T(), debugErr)
	assert.Contains(s.T(), debugRendered, "[DEBUG ]")
	assert.Contains(s.T(), debugRendered, "router trace")

	fatalRendered, fatalErr := styler.StyleText(capabilities.TextKindFatal, "panic path")
	require.NoError(s.T(), fatalErr)
	assert.Contains(s.T(), fatalRendered, "[FATAL ]")
	assert.Contains(s.T(), fatalRendered, "panic path")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleText_UnsupportedKind() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	rendered, styleErr := styler.StyleText("mystery", "ready")
	require.Error(s.T(), styleErr)
	assert.Empty(s.T(), rendered)
	assert.Contains(s.T(), styleErr.Error(), "unsupported kind")
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
		capabilities.TableKindNormal,
		[]string{"Name", "Status"},
		[][]string{{"scanner", "ready"}},
	)
	require.NoError(s.T(), styleErr)
	assert.Contains(s.T(), rendered, "Name")
	assert.Contains(s.T(), rendered, "Status")
	assert.Contains(s.T(), rendered, "scanner")
	assert.Contains(s.T(), rendered, "ready")
	assert.Contains(s.T(), rendered, "╭")
	assert.Contains(s.T(), rendered, "│")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleTable_MergedAndMutedKinds() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	mergedRendered, mergedErr := styler.StyleTable(
		capabilities.TableKindMerged,
		[]string{"File", "Status"},
		[][]string{{"a.go", "ok"}, {"a.go", "cached"}},
	)
	require.NoError(s.T(), mergedErr)
	assert.Contains(s.T(), mergedRendered, "a.go")

	mutedRendered, mutedErr := styler.StyleTable(
		capabilities.TableKindMuted,
		[]string{"Name", "Status"},
		[][]string{{"scanner", "ready"}},
	)
	require.NoError(s.T(), mutedErr)
	assert.Contains(s.T(), mutedRendered, "scanner")
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
		capabilities.TableKindNormal,
		[]string{"Name", "Status"},
		[][]string{{"scanner"}},
	)
	require.Error(s.T(), styleErr)
	assert.Empty(s.T(), rendered)
	assert.Contains(s.T(), styleErr.Error(), "row 0 width 1 does not match header width 2")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleLayout() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	panelRendered, panelErr := styler.StyleLayout(
		capabilities.LayoutKindPanel,
		"Summary",
		"scanner ready",
	)
	require.NoError(s.T(), panelErr)
	assert.Contains(s.T(), panelRendered, "Summary")
	assert.Contains(s.T(), panelRendered, "scanner ready")

	splitRendered, splitErr := styler.StyleLayout(
		capabilities.LayoutKindSplit,
		"",
		"left",
		"right",
	)
	require.NoError(s.T(), splitErr)
	assert.Contains(s.T(), splitRendered, "left")
	assert.Contains(s.T(), splitRendered, "right")

	gutterRendered, gutterErr := styler.StyleLayout(
		capabilities.LayoutKindGutter,
		"INFO",
		"booted",
	)
	require.NoError(s.T(), gutterErr)
	assert.Contains(s.T(), gutterRendered, "INFO")
	assert.Contains(s.T(), gutterRendered, "booted")
}

func (s *RouterSuite) TestCLIStyleProvider_StyleLayout_UnsupportedKind() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&prettystyle.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	styler, resolveErr := capabilities.ResolveCLIOutputStyler()
	require.NoError(s.T(), resolveErr)

	rendered, styleErr := styler.StyleLayout("layout.unknown", "Summary", "scanner ready")
	require.Error(s.T(), styleErr)
	assert.Empty(s.T(), rendered)
	assert.Contains(s.T(), styleErr.Error(), "unsupported kind")
}

func (s *RouterSuite) TestCLIChromeStyler_TextAndLayout() {
	warnings, err := router.RouterLoadExtensions(nil, []router.Extension{
		&charmcli.Extension{},
	}, context.Background())

	require.NoError(s.T(), err)
	assert.Empty(s.T(), warnings)

	chromeStyler, resolveErr := capabilities.ResolveCLIChromeStyler()
	require.NoError(s.T(), resolveErr)

	renderedText, textErr := chromeStyler.StyleText(capabilities.TextKindHeader, "Policy Engine Interactive")
	require.NoError(s.T(), textErr)
	assert.Contains(s.T(), renderedText, "Policy Engine Interactive")

	renderedLayout, layoutErr := chromeStyler.StyleLayout(
		capabilities.LayoutKindPanel,
		"Function Quality",
		"Monitors health metrics across Go, Python, and TypeScript source files.",
	)
	require.NoError(s.T(), layoutErr)
	assert.Contains(s.T(), renderedLayout, "FUNCTION QUALITY")
	assert.Contains(s.T(), renderedLayout, "Monitors health metrics across Go, Python, and TypeScript source files.")
}

func (s *RouterSuite) TestCLIInteractor_PromptKinds() {
	provider := &charmcli.Provider{
		RunForm: func(_ *huh.Form) error {
			return nil
		},
	}

	selectResult, selectErr := provider.StylePrompt(
		capabilities.PromptKindSelect,
		"Choose renderer",
		"Select one output provider.",
		[]capabilities.Choice{
			{Key: "pretty", Label: "Pretty"},
			{Key: "plain", Label: "Plain"},
		},
	)
	require.NoError(s.T(), selectErr)
	assert.Equal(s.T(), "pretty", selectResult)

	toggleResult, toggleErr := provider.StylePrompt(
		capabilities.PromptKindToggle,
		"Enable outputs",
		"Pick all that apply.",
		[]capabilities.Choice{
			{Key: "table", Label: "Table"},
			{Key: "panel", Label: "Panel"},
		},
	)
	require.NoError(s.T(), toggleErr)
	assert.Equal(s.T(), []string{}, toggleResult)

	confirmResult, confirmErr := provider.StylePrompt(
		capabilities.PromptKindConfirm,
		"Proceed?",
		"Continue with boot.",
		nil,
	)
	require.NoError(s.T(), confirmErr)
	assert.Equal(s.T(), false, confirmResult)

	inputResult, inputErr := provider.StylePrompt(
		capabilities.PromptKindInput,
		"Port value",
		"Enter the port name.",
		nil,
	)
	require.NoError(s.T(), inputErr)
	assert.Equal(s.T(), "", inputResult)
}

func (s *RouterSuite) TestCLIInteractor_PromptKinds_InvalidInput() {
	provider := &charmcli.Provider{
		RunForm: func(_ *huh.Form) error {
			return nil
		},
	}

	selectResult, selectErr := provider.StylePrompt(
		capabilities.PromptKindSelect,
		"Choose renderer",
		"Select one output provider.",
		nil,
	)
	require.Error(s.T(), selectErr)
	assert.Equal(s.T(), "", selectResult)

	toggleResult, toggleErr := provider.StylePrompt(
		capabilities.PromptKindToggle,
		"Enable outputs",
		"Pick all that apply.",
		nil,
	)
	require.Error(s.T(), toggleErr)
	assert.Nil(s.T(), toggleResult)

	unknownResult, unknownErr := provider.StylePrompt("prompt.unknown", "Unknown", "", nil)
	require.Error(s.T(), unknownErr)
	assert.Nil(s.T(), unknownResult)
	assert.Contains(s.T(), unknownErr.Error(), "unsupported kind")
}

func (s *RouterSuite) TestCLIInteractor_PromptKinds_FormFailure() {
	provider := &charmcli.Provider{
		RunForm: func(_ *huh.Form) error {
			return assert.AnError
		},
	}

	result, err := provider.StylePrompt(
		capabilities.PromptKindConfirm,
		"Proceed?",
		"Continue with boot.",
		nil,
	)
	require.Error(s.T(), err)
	assert.Equal(s.T(), false, result)
	assert.Contains(s.T(), err.Error(), "style prompt confirm")
}
