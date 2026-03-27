package wrlk_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalRouterManifestFile = `package router

type PortManifestEntry struct {
	Name  string
	Value string
}

type OptionalExtensionManifestEntry struct {
	Name string
}

var DeclaredPorts = []PortManifestEntry{
	{Name: "PortPrimary", Value: "primary"},
}

var DeclaredOptionalExtensions = []OptionalExtensionManifestEntry{
	{Name: "telemetry"},
}
`

const minimalAppManifestFile = `package ext

type ApplicationExtensionManifestEntry struct {
	Name string
}

var DeclaredApplicationExtensions = []ApplicationExtensionManifestEntry{
	{Name: "primary"},
}
`

func TestRegisterPortRouter_UpdatesManifestAndGeneratedFiles(t *testing.T) {
	root := createRegisterFixture(t)

	result := runWrlkCommand(t, root, "register", "--port", "--router", "--name", "PortBilling", "--value", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	manifest := readFixtureFile(t, root, "internal/router/router_manifest.go")
	assert.Contains(t, manifest, `{Name: "PortBilling", Value: "billing"}`)

	ports := readFixtureFile(t, root, "internal/router/ports.go")
	assert.Contains(t, ports, `PortBilling PortName = "billing"`)

	registryImports := readFixtureFile(t, root, "internal/router/registry_imports.go")
	assert.Contains(t, registryImports, "case PortPrimary:")
	assert.Contains(t, registryImports, "case PortBilling:")
}

func TestRegisterExtRouter_UpdatesManifestAndOptionalExtensions(t *testing.T) {
	root := createRegisterFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(extExtensionsRelDir, "metrics")))

	result := runWrlkCommand(t, root, "register", "--ext", "--router", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	manifest := readFixtureFile(t, root, "internal/router/router_manifest.go")
	assert.Contains(t, manifest, `{Name: "metrics"}`)

	optionalExtensions := readFixtureFile(t, root, extOptionalRelPath)
	assert.Contains(t, optionalExtensions, `/internal/router/ext/extensions/metrics"`)
	assert.Contains(t, optionalExtensions, "&metrics.Extension{}")
}

func TestRegisterExtApp_UpdatesManifestAndApplicationExtensions(t *testing.T) {
	root := createRegisterFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "billing")))

	result := runWrlkCommand(t, root, "register", "--ext", "--app", "--name", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	manifest := readFixtureFile(t, root, "internal/router/ext/app_manifest.go")
	assert.Contains(t, manifest, `{Name: "billing"}`)

	applicationExtensions := readFixtureFile(t, root, extApplicationRelPath)
	assert.Contains(t, applicationExtensions, `/internal/adapters/billing"`)
	assert.Contains(t, applicationExtensions, "&billing.Extension{}")
}

func TestRegisterRejectsInvalidFlagMixes(t *testing.T) {
	root := createRegisterFixture(t)

	testCases := []struct {
		name    string
		args    []string
		message string
	}{
		{
			name:    "port app invalid",
			args:    []string{"register", "--port", "--app", "--name", "PortFoo", "--value", "foo"},
			message: "--port requires --router",
		},
		{
			name:    "ext missing target",
			args:    []string{"register", "--ext", "--name", "billing"},
			message: "--ext requires exactly one of --router or --app",
		},
		{
			name:    "multiple primary selectors",
			args:    []string{"register", "--port", "--ext", "--router", "--name", "billing", "--value", "foo"},
			message: "exactly one of --port or --ext",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runWrlkCommand(t, root, testCase.args...)
			require.Error(t, result.err)
			assert.NotEqual(t, 0, result.exitCode)
			assert.Contains(t, result.stdout+result.stderr, testCase.message)
		})
	}
}

func createRegisterFixture(t *testing.T) string {
	t.Helper()

	root := createExtFixture(t)
	writeExtFixtureFile(t, root, "internal/router/router_manifest.go", minimalRouterManifestFile)
	writeExtFixtureFile(t, root, "internal/router/ext/app_manifest.go", minimalAppManifestFile)

	return root
}
