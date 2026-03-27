package wrlk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleSync_RewritesBundledRouterImportsToCurrentModule(t *testing.T) {
	root := createExtFixture(t)

	writeExtFixtureFile(t, root, "internal/router/capabilities/cli.go", `package capabilities

import "github.com/michaelbomholt665/wrlk/internal/router"
`)
	writeExtFixtureFile(t, root, "internal/router/ext/extensions/telemetry/extension.go", `package telemetry

import "github.com/michaelbomholt665/wrlk/internal/router"
`)
	writeExtFixtureFile(t, root, "internal/router/tools/wrlk/register.go", `package main

const templateSource = "github.com/michaelbomholt665/wrlk/internal/router"
`)

	result := runWrlkCommand(t, root, "module", "sync")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, `module sync: rewrote 3 file(s)`)
	assert.Contains(t, result.stdout, `"github.com/michaelbomholt665/wrlk"`)
	assert.Contains(t, result.stdout, `"testmodule"`)

	assert.Contains(t, readFixtureFile(t, root, "internal/router/capabilities/cli.go"), `"testmodule/internal/router"`)
	assert.Contains(t, readFixtureFile(t, root, "internal/router/ext/extensions/telemetry/extension.go"), `"testmodule/internal/router"`)
	assert.Contains(t, readFixtureFile(t, root, "internal/router/tools/wrlk/register.go"), `"testmodule/internal/router"`)
}

func TestRegisterExtRouter_GeneratesOptionalExtensionsWithModulePathFromGoMod(t *testing.T) {
	root := createRegisterFixture(t)
	createPackageDir(t, root, extExtensionsRelDir+"/metrics")

	result := runWrlkCommand(t, root, "register", "--ext", "--router", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	optionalExtensions := readFixtureFile(t, root, extOptionalRelPath)
	assert.Contains(t, optionalExtensions, `"testmodule/internal/router"`)
	assert.Contains(t, optionalExtensions, `"testmodule/internal/router/ext/extensions/metrics"`)
	assert.NotContains(t, optionalExtensions, "github.com/michaelbomholt665/wrlk")
}
