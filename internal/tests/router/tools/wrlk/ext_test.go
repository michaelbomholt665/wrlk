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

// — Minimal ext fixture content —

const minimalGoMod = `module testmodule

go 1.24
`

const minimalOptionalExtensionsFile = `package ext

import (
	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/ext/extensions/telemetry"
)

// optionalExtensions is the canonical slice of router capability extensions.
var optionalExtensions = []router.Extension{
	&telemetry.Extension{},
}
`

const minimalApplicationExtensionsFile = `package ext

import (
	"github.com/michaelbomholt665/wrlk/internal/router"
	"github.com/michaelbomholt665/wrlk/internal/router/ext/extensions/primary"
)

var extensions = []router.Extension{
	&primary.Extension{},
}
`

const (
	extOptionalRelPath    = "internal/router/ext/optional_extensions.go"
	extApplicationRelPath = "internal/router/ext/extensions.go"
	extExtensionsRelDir   = "internal/router/ext/extensions"
)

// — Tests —

// TestExtAdd_CreatesDocGo verifies that doc.go is created at the correct path.
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

// TestExtAdd_CreatesExtensionGo verifies that extension.go is created with the correct package and Extension type.
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

// TestExtAdd_SplicesOptionalExtensions verifies that optional_extensions.go is updated with the new import and entry.
func TestExtAdd_SplicesOptionalExtensions(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)
	src := string(content)

	assert.Contains(t, src, "/metrics\"")
	assert.Contains(t, src, "&metrics.Extension{}")
}

// TestExtAdd_DryRun_NoWrite verifies that --dry-run prints intent without creating any files.
func TestExtAdd_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)

	result := runWrlkCommand(t, root, "ext", "add", "--name", "metrics", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	combined := result.stdout + result.stderr
	assert.True(
		t,
		strings.Contains(combined, "metrics") && strings.Contains(combined, "dry-run"),
		"expected name and dry-run indication in output, got stdout=%q stderr=%q",
		result.stdout, result.stderr,
	)

	extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "metrics")
	assert.NoDirExists(t, extDir, "dry-run must not create the extension directory")

	optionalAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(optionalBefore), string(optionalAfter), "optional_extensions.go must not be modified in dry-run mode")
}

// TestExtAdd_DuplicateName_Fails verifies that adding the same extension name twice fails.
func TestExtAdd_DuplicateName_Fails(t *testing.T) {
	root := createExtFixture(t)

	first := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.NoError(t, first.err, first.stderr)

	optionalAfterFirst, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)

	second := runWrlkCommand(t, root, "ext", "add", "--name", "metrics")
	require.Error(t, second.err)
	assert.NotEqual(t, 0, second.exitCode)
	assert.True(
		t,
		strings.Contains(second.stdout+second.stderr, "metrics"),
		"expected extension name in error output",
	)

	// optional_extensions.go must not be modified by the failing second add.
	optionalAfterSecond, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(optionalAfterFirst), string(optionalAfterSecond))
}

// TestExtAdd_MissingName_Fails verifies that omitting --name fails with an error.
func TestExtAdd_MissingName_Fails(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "add")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)

	combined := result.stdout + result.stderr
	assert.True(
		t,
		strings.Contains(combined, "name") || strings.Contains(combined, "required"),
		"expected actionable error mentioning --name, got stdout=%q stderr=%q",
		result.stdout, result.stderr,
	)
}

// TestExtAdd_InvalidName_Fails verifies that names containing disallowed characters are rejected.
func TestExtAdd_InvalidName_Fails(t *testing.T) {
	root := createExtFixture(t)

	for _, badName := range []string{"my-extension", "my extension", "1start", "my.ext"} {
		result := runWrlkCommand(t, root, "ext", "add", "--name", badName)
		require.Error(t, result.err, "expected failure for name %q", badName)
		assert.NotEqual(t, 0, result.exitCode, "expected non-zero exit for name %q", badName)
	}
}

// TestExtAdd_WritesSnapshot verifies that a restore snapshot is captured before the
// extension is scaffolded, and that it includes optional_extensions.go so the
// composition file can be restored alongside the router core.
func TestExtAdd_WritesSnapshot(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)

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
	assert.NotEmpty(t, snapshot.Files, "snapshot must record at least the tracked router core files")

	// optional_extensions.go must be captured with its pre-mutation content,
	// allowing lock restore to recover the composition file alongside the router core.
	var foundOptional bool
	for _, file := range snapshot.Files {
		if file.File == extOptionalRelPath {
			foundOptional = true
			assert.True(t, file.Exists, "optional_extensions.go must be marked as existing in snapshot")
			assert.Equal(
				t,
				string(optionalBefore),
				file.Content,
				"snapshot must store the pre-mutation content of optional_extensions.go",
			)
		}
	}
	assert.True(t, foundOptional, "snapshot must include optional_extensions.go")
}

// TestExtAdd_HelpFlag_PrintsUsage verifies that --help prints the ext add usage text.
func TestExtAdd_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "add", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
}

// TestExtHelp_PrintsExtUsage verifies that `ext --help` prints the ext subcommand list.
func TestExtHelp_PrintsExtUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "ext")
	assert.Contains(t, result.stdout, "add")
	assert.Contains(t, result.stdout, "app add")
}

func TestExtAppAdd_SplicesApplicationExtensions(t *testing.T) {
	root := createExtFixture(t)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extApplicationRelPath)))
	require.NoError(t, err)
	src := string(content)

	assert.Contains(t, src, "/billing\"")
	assert.Contains(t, src, "&billing.Extension{}")

	extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "billing")
	assert.NoDirExists(t, extDir, "ext app add must not create a router capability extension package")
}

func TestExtAppAdd_DryRun_NoWrite(t *testing.T) {
	root := createExtFixture(t)

	applicationBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extApplicationRelPath)))
	require.NoError(t, err)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), "billing")
	assert.NoDirExists(t, extDir, "dry-run must not create the extension directory")

	applicationAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extApplicationRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(applicationBefore), string(applicationAfter), "extensions.go must not be modified in dry-run mode")
	assert.Contains(t, result.stdout+result.stderr, "extensions.go")
}

func TestExtAppAdd_HelpFlag_PrintsUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "ext", "app", "add", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--name")
	assert.Contains(t, result.stdout, "application router extension")
}

func TestExtAppAdd_WritesOnlyApplicationComposition(t *testing.T) {
	root := createExtFixture(t)

	optionalBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)

	result := runWrlkCommand(t, root, "ext", "app", "add", "--name", "billing")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	optionalAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(extOptionalRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(optionalBefore), string(optionalAfter))
	assert.NotContains(t, result.stdout, "create extension package directory")
	assert.Contains(t, result.stdout, "wired application router extension")
}

// — Fixture helpers —

func createExtFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	// go.mod so the tool can read the module path.
	writeExtFixtureFile(t, root, "go.mod", minimalGoMod)

	// The minimal optional_extensions.go the tool will splice into.
	writeExtFixtureFile(t, root, extOptionalRelPath, minimalOptionalExtensionsFile)
	writeExtFixtureFile(t, root, extApplicationRelPath, minimalApplicationExtensionsFile)

	// Minimal router core files are required by RouterWriteSnapshotBeforeMutation.
	writeExtFixtureFile(t, root, "internal/router/extension.go", "package router\n\nfunc RouterLoadExtensions() {}\n")
	writeExtFixtureFile(t, root, "internal/router/registry.go", "package router\n\nfunc RouterResolveProvider() {}\n")
	writeExtFixtureFile(t, root, "internal/router/ports.go", "package router\n")
	writeExtFixtureFile(t, root, "internal/router/registry_imports.go", "package router\n")

	return root
}

func writeExtFixtureFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, []byte(content), 0o600))
}
