package wrlk_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalGoMod = `module testmodule

go 1.24
`

const minimalOptionalExtensionsFile = `package ext

import (
	"testmodule/internal/router"
	"testmodule/internal/router/ext/extensions/telemetry"
)

// optionalExtensions is the canonical slice of router capability extensions.
var optionalExtensions = []router.Extension{
	&telemetry.Extension{},
}
`

const minimalApplicationExtensionsFile = `package ext

import (
	"testmodule/internal/router"
	"testmodule/internal/adapters/primary"
)

var extensions = []router.Extension{
	&primary.Extension{},
}
`

const (
	extOptionalRelPath    = "internal/router/ext/optional_extensions.go"
	extApplicationRelPath = "internal/router/ext/extensions.go"
	extExtensionsRelDir   = "internal/router/ext/extensions"
	adapterRelDir         = "internal/adapters"
)

func TestExtAdd_CreatesDocGo(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	docPath := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "metrics", "doc.go")
	require.FileExists(t, docPath)

	content, err := os.ReadFile(docPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "package metrics")
}

func TestExtAdd_CreatesExtensionGo(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	extPath := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "metrics", "extension.go")
	require.FileExists(t, extPath)

	content, err := os.ReadFile(extPath)
	require.NoError(t, err)
	src := string(content)

	assert.Contains(t, src, "package metrics")
	assert.Contains(t, src, "type Extension struct{}")
	assert.Contains(t, src, "func (e *Extension) Required()")
	assert.Contains(t, src, "func (e *Extension) RouterProvideRegistration(")
}

func TestExtAdd_SplicesOptionalExtensions(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extOptionalRelPath)
	assert.Contains(t, src, "/metrics\"")
	assert.Contains(t, src, "&metrics.Extension{}")
}

func TestExtAdd_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore := readFixtureFile(t, root, extOptionalRelPath)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	assert.Contains(t, result.stdout+result.stderr, "dry-run")
	assert.Contains(t, result.stdout+result.stderr, "metrics")
	assert.NoDirExists(t, filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "metrics"))
	assert.Equal(t, optionalBefore, readFixtureFile(t, root, extOptionalRelPath))
}

func TestExtAdd_DuplicateName_Fails(t *testing.T) {
	root := createExtFixture(t)

	first := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, first.err, first.stderr)

	optionalAfterFirst := readFixtureFile(t, root, extOptionalRelPath)

	second := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.Error(t, second.err)
	assert.NotEqual(t, 0, second.exitCode)
	assert.Contains(t, second.stdout+second.stderr, "already")
	assert.Equal(t, optionalAfterFirst, readFixtureFile(t, root, extOptionalRelPath))
}

func TestExtAdd_MissingName_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "name")
}

func TestExtAdd_InvalidName_Fails(t *testing.T) {
	root := createExtFixture(t)

	for _, badName := range []string{"my-extension", "my extension", "1start", "my.ext"} {
		result := runWrlkCommand(t, root, "ext", "add", "--name", badName)
		require.Error(t, result.err, "expected failure for name %q", badName)
		assert.NotEqual(t, 0, result.exitCode, "expected non-zero exit for name %q", badName)
	}
}

func TestExtAdd_WritesSnapshot(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore := readFixtureFile(t, root, extOptionalRelPath)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)

	snapshotPath := filepath.Join(root, "internal", "router", "router.snapshot.json")
	require.FileExists(t, snapshotPath)

	snapshotContent, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)

	var snapshot struct {
		Reason string `json:"reason"`
		Files  []struct {
			File    string `json:"file"`
			Exists  bool   `json:"exists"`
			Content string `json:"content"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(snapshotContent, &snapshot))

	assert.Contains(t, snapshot.Reason, "wrlk ext add")
	assert.Contains(t, snapshot.Reason, "metrics")

	var foundOptional bool
	for _, file := range snapshot.Files {
		if file.File == extOptionalRelPath {
			foundOptional = true
			assert.True(t, file.Exists)
			assert.Equal(t, optionalBefore, file.Content)
		}
	}
	assert.True(t, foundOptional)
}

func TestExtInstall_WiresExistingOptionalExtension(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(extExtensionsRelDir, "metrics")))

	result := runWrlkCommand(t, root, "ext", "install", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extOptionalRelPath)
	assert.Contains(t, src, "/metrics\"")
	assert.Contains(t, src, "&metrics.Extension{}")
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "metrics", "doc.go"))
}

func TestExtInstall_DuplicateAlreadyWired_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "install", "--name", "telemetry")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "already")
}

func TestExtInstall_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(extExtensionsRelDir, "metrics")))

	optionalBefore := readFixtureFile(t, root, extOptionalRelPath)

	result := runWrlkCommand(t, root, "ext", "install", "--name", "metrics", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	assert.Contains(t, result.stdout+result.stderr, "dry-run")
	assert.Contains(t, result.stdout+result.stderr, "optional_extensions.go")
	assert.Equal(t, optionalBefore, readFixtureFile(t, root, extOptionalRelPath))
}

func TestExtRemove_UnwiresOptionalExtension(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "remove", "--name", "telemetry")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extOptionalRelPath)
	assert.NotContains(t, src, "/telemetry\"")
	assert.NotContains(t, src, "&telemetry.Extension{}")
}

func TestExtRemove_NotFound_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "remove", "--name", "metrics")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "not")
}

func TestExtRemove_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore := readFixtureFile(t, root, extOptionalRelPath)

	result := runWrlkCommand(t, root, "ext", "remove", "--name", "telemetry", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	assert.Contains(t, result.stdout+result.stderr, "dry-run")
	assert.Contains(t, result.stdout+result.stderr, "remove")
	assert.Equal(t, optionalBefore, readFixtureFile(t, root, extOptionalRelPath))
}

func TestExtHelp_PrintsExtUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "ext")
	assert.Contains(t, result.stdout, "add")
	assert.Contains(t, result.stdout, "install")
	assert.Contains(t, result.stdout, "remove")
	assert.Contains(t, result.stdout, "app add")
	assert.Contains(t, result.stdout, "app remove")
}

func TestExtAdd_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "add", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
}

func TestExtInstall_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "install", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
	assert.Contains(t, result.stdout, "router capability extension")
}

func TestExtRemove_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "remove", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
}

func TestExtAppAdd_WiresExistingAdapter(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "billing")))

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extApplicationRelPath)
	assert.Contains(t, src, "/internal/adapters/billing\"")
	assert.Contains(t, src, "&billing.Extension{}")
	assert.NoDirExists(t, filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "billing"))
}

func TestExtAppAdd_InlineEmptySlice_WiresInsideSlice(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "billing")))
	writeExtFixtureFile(t, root, extApplicationRelPath, `package ext

import (
	"testmodule/internal/router"
)

var extensions = []router.Extension{}
`)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extApplicationRelPath)
	assert.Contains(t, src, "var extensions = []router.Extension{\n\t&billing.Extension{},\n}")
	assert.NotContains(t, src, "return append([]router.Extension(nil), optionalExtensions...), append([]router.Extension(nil), extensions...)\n\t&billing.Extension{},")
	assert.Contains(t, src, "/internal/adapters/billing\"")
}

func TestExtAppAdd_DuplicateAlreadyWired_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "primary")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "already")
}

func TestExtAppAdd_MissingAdapter_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "does not exist")
	assert.Contains(t, result.stdout+result.stderr, "internal/adapters/billing")
}

func TestExtAppAdd_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "billing")))

	applicationBefore := readFixtureFile(t, root, extApplicationRelPath)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	assert.Equal(t, applicationBefore, readFixtureFile(t, root, extApplicationRelPath))
	assert.Contains(t, result.stdout+result.stderr, "extensions.go")
	assert.NoDirExists(t, filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "billing"))
}

func TestExtAppRemove_UnwiresApplicationAdapter(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "app", "remove", "--name", "primary")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	src := readFixtureFile(t, root, extApplicationRelPath)
	assert.NotContains(t, src, "/internal/adapters/primary\"")
	assert.NotContains(t, src, "&primary.Extension{}")
}

func TestExtAppRemove_NotFound_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "app", "remove", "--name", "billing")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stdout+result.stderr, "not")
}

func TestExtAppRemove_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)

	applicationBefore := readFixtureFile(t, root, extApplicationRelPath)

	result := runWrlkCommand(t, root, "ext", "app", "remove", "--name", "primary", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	assert.Equal(t, applicationBefore, readFixtureFile(t, root, extApplicationRelPath))
	assert.Contains(t, result.stdout+result.stderr, "dry-run")
	assert.Contains(t, result.stdout+result.stderr, "remove")
}

func TestExtAppAdd_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "app", "add", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
	assert.Contains(t, result.stdout, "application adapter extension")
}

func TestExtAppRemove_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "app", "remove", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
}

func TestExtAppHelp_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "app", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "ext app")
	assert.Contains(t, result.stdout, "add")
	assert.Contains(t, result.stdout, "remove")
}

func createExtFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	writeExtFixtureFile(t, root, "go.mod", minimalGoMod)
	writeExtFixtureFile(t, root, extOptionalRelPath, minimalOptionalExtensionsFile)
	writeExtFixtureFile(t, root, extApplicationRelPath, minimalApplicationExtensionsFile)

	writeExtFixtureFile(t, root, filepath.ToSlash(filepath.Join(extExtensionsRelDir, "telemetry", "doc.go")), "package telemetry\n")
	writeExtFixtureFile(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "primary", "doc.go")), "package primary\n")

	writeExtFixtureFile(t, root, "internal/router/extension.go", "package router\n\nfunc RouterLoadExtensions() {}\n")
	writeExtFixtureFile(t, root, "internal/router/registry.go", "package router\n\nfunc RouterResolveProvider() {}\n")
	writeExtFixtureFile(t, root, "internal/router/ports.go", "package router\n")
	writeExtFixtureFile(t, root, "internal/router/registry_imports.go", "package router\n")

	return root
}

func createPackageDir(t *testing.T, root, relativeDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(relativeDir)), 0o755))
}

func readFixtureFile(t *testing.T, root, relativePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	require.NoError(t, err)
	return string(content)
}

func writeExtFixtureFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, []byte(content), 0o600))
}

func TestExtMutationOutputMentionsTargetFiles(t *testing.T) {
	root := createExtFixture(t)
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(adapterRelDir, "billing")))
	createPackageDir(t, root, filepath.ToSlash(filepath.Join(extExtensionsRelDir, "metrics")))

	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{name: "install", args: []string{"ext", "install", "--name", "metrics", "--dry-run"}, expected: extOptionalRelPath},
		{name: "remove", args: []string{"ext", "remove", "--name", "telemetry", "--dry-run"}, expected: extOptionalRelPath},
		{name: "app add", args: []string{"ext", "app", "add", "--name", "billing", "--dry-run"}, expected: extApplicationRelPath},
		{name: "app remove", args: []string{"ext", "app", "remove", "--name", "primary", "--dry-run"}, expected: extApplicationRelPath},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runWrlkCommand(t, root, testCase.args...)
			require.NoError(t, result.err, result.stderr)
			assert.Equal(t, 0, result.exitCode)
			assert.True(t, strings.Contains(result.stdout, testCase.expected) || strings.Contains(result.stderr, testCase.expected))
		})
	}
}
