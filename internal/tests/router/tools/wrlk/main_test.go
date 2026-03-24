package wrlk_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	routerExtensionPath = "internal/router/extension.go"
	routerRegistryPath  = "internal/router/registry.go"
	routerLockPath      = "internal/router/router.lock"
	routerSnapshotPath  = "internal/router/router.snapshot.json"
)

type lockRecord struct {
	File     string `json:"file"`
	Checksum string `json:"checksum"`
}

type routerFileSnapshot struct {
	CreatedAt string               `json:"created_at"`
	Reason    string               `json:"reason"`
	Files     []routerSnapshotFile `json:"files"`
}

type routerSnapshotFile struct {
	File     string `json:"file"`
	Exists   bool   `json:"exists"`
	Checksum string `json:"checksum,omitempty"`
	Content  string `json:"content,omitempty"`
}

type commandResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func TestWrlkLockVerifyWorkflow(t *testing.T) {
	fixtureRoot := createRouterFixture(t, map[string]string{
		routerExtensionPath: "package router\n\nfunc RouterLoadExtensions() {}\n",
		routerRegistryPath:  "package router\n\nfunc RouterResolveProvider() {}\n",
	})
	writeLockFile(t, fixtureRoot, []string{routerExtensionPath, routerRegistryPath})

	result := runWrlkCommand(t, fixtureRoot, "lock", "verify")
	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "verified")
	assert.Contains(t, result.stdout, routerLockPath)

	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(fixtureRoot, filepath.FromSlash(routerRegistryPath)),
			[]byte("package router\n\nfunc RouterResolveProvider() { panic(\"drift\") }\n"),
			0o600,
		),
	)

	result = runWrlkCommand(t, fixtureRoot, "lock", "verify")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stderr, "checksum mismatch")
	assert.Contains(t, result.stderr, routerRegistryPath)
}

func TestWrlkLockUpdateWorkflow(t *testing.T) {
	fixtureRoot := createRouterFixture(t, map[string]string{
		routerExtensionPath: "package router\n\nfunc RouterLoadExtensions() {}\n",
		routerRegistryPath:  "package router\n\nfunc RouterResolveProvider() {}\n",
	})

	verifyResult := runWrlkCommand(t, fixtureRoot, "lock", "verify")
	require.Error(t, verifyResult.err)
	assert.NotEqual(t, verifyResult.exitCode, 0)
	assert.Contains(t, verifyResult.stderr, routerLockPath)
	assert.NoFileExists(t, filepath.Join(fixtureRoot, filepath.FromSlash(routerLockPath)))

	updateResult := runWrlkCommand(t, fixtureRoot, "lock", "update")
	require.NoError(t, updateResult.err, updateResult.stderr)
	assert.Equal(t, 0, updateResult.exitCode)
	assert.FileExists(t, filepath.Join(fixtureRoot, filepath.FromSlash(routerLockPath)))
	assert.Contains(t, updateResult.stdout, "updated")

	verifyResult = runWrlkCommand(t, fixtureRoot, "lock", "verify")
	require.NoError(t, verifyResult.err, verifyResult.stderr)
	assert.Equal(t, 0, verifyResult.exitCode)
	assert.Contains(t, verifyResult.stdout, "verified")
}

func TestWrlkLockRestoreWorkflow(t *testing.T) {
	fixtureRoot := createRouterFixture(t, map[string]string{
		routerExtensionPath:                   "package router\n\nfunc RouterLoadExtensions() {}\n",
		routerRegistryPath:                    "package router\n\nfunc RouterResolveProvider() {}\n",
		"internal/router/ports.go":            "package router\n\nconst PortPrimary = \"primary\"\n",
		"internal/router/registry_imports.go": "package router\n\nfunc RouterValidatePortName() {}\n",
	})
	writeLockFile(t, fixtureRoot, []string{routerExtensionPath, routerRegistryPath})
	writeSnapshotFile(t, fixtureRoot, routerFileSnapshot{
		CreatedAt: "2026-03-24T00:00:00Z",
		Reason:    "test snapshot",
		Files: []routerSnapshotFile{
			{File: routerExtensionPath, Exists: true, Content: "package router\n\nfunc RouterLoadExtensions() {}\n"},
			{File: "internal/router/ports.go", Exists: true, Content: "package router\n\nconst PortPrimary = \"primary\"\n"},
			{File: routerRegistryPath, Exists: true, Content: "package router\n\nfunc RouterResolveProvider() {}\n"},
			{File: "internal/router/registry_imports.go", Exists: true, Content: "package router\n\nfunc RouterValidatePortName() {}\n"},
			{File: routerLockPath, Exists: false},
		},
	})

	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(fixtureRoot, filepath.FromSlash(routerRegistryPath)),
			[]byte("package router\n\nfunc RouterResolveProvider() { panic(\"drift\") }\n"),
			0o600,
		),
	)
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(fixtureRoot, filepath.FromSlash("internal/router/ports.go")),
			[]byte("package router\n\nconst PortFoo = \"foo\"\n"),
			0o600,
		),
	)

	result := runWrlkCommand(t, fixtureRoot, "lock", "restore")
	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "router snapshot restored")

	restoredRegistry, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(routerRegistryPath)))
	require.NoError(t, err)
	assert.Equal(t, "package router\n\nfunc RouterResolveProvider() {}\n", string(restoredRegistry))

	restoredPorts, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash("internal/router/ports.go")))
	require.NoError(t, err)
	assert.Equal(t, "package router\n\nconst PortPrimary = \"primary\"\n", string(restoredPorts))

	assert.NoFileExists(t, filepath.Join(fixtureRoot, filepath.FromSlash(routerLockPath)))
}

func TestWrlkLockRestore_MissingSnapshot_Fails(t *testing.T) {
	fixtureRoot := createRouterFixture(t, map[string]string{
		routerExtensionPath: "package router\n\nfunc RouterLoadExtensions() {}\n",
		routerRegistryPath:  "package router\n\nfunc RouterResolveProvider() {}\n",
	})

	result := runWrlkCommand(t, fixtureRoot, "lock", "restore")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stderr, "missing")
	assert.Contains(t, result.stderr, routerSnapshotPath)
}

func TestWrlkGuideCommand_PrintsOperationalGuide(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "guide")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "Router guide:")
	assert.Contains(t, result.stdout, "wrlk add")
	assert.Contains(t, result.stdout, "lock verify")
	assert.Contains(t, result.stdout, "lock restore")
	assert.Contains(t, result.stdout, "ext app add")
	assert.Contains(t, result.stdout, "contract-blind")
	assert.Contains(t, result.stdout, "Any")
}

func TestWrlkHelpFlag_PrintsTopLevelUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "usage: Router")
	assert.Contains(t, result.stdout, "lock verify")
	assert.Contains(t, result.stdout, "ext app add")
	assert.NotContains(t, result.stderr, "help requested")
}

func TestWrlkLockHelp_PrintsLockUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "lock", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "lock <subcommand>")
	assert.Contains(t, result.stdout, "restore")
}

func TestWrlkAddHelp_PrintsAddUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "add", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "usage: wrlk")
	assert.Contains(t, result.stdout, "--name")
}

func TestWrlkLiveHelp_PrintsLiveUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "live", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "live run")
}

func TestWrlkLiveRunHelp_PrintsRunUsage(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "live", "run", "--help")

	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "--expect")
	assert.Contains(t, result.stdout, "--timeout")
}

func createRouterFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for relativePath, content := range files {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
		require.NoError(t, os.WriteFile(absolutePath, []byte(content), 0o600))
	}

	return root
}

func writeLockFile(t *testing.T, root string, lockedFiles []string) {
	t.Helper()

	lockAbsolutePath := filepath.Join(root, filepath.FromSlash(routerLockPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(lockAbsolutePath), 0o755))

	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	for _, relativePath := range lockedFiles {
		record := lockRecord{
			File:     relativePath,
			Checksum: checksumForFile(t, root, relativePath),
		}
		require.NoError(t, encoder.Encode(record))
	}

	require.NoError(t, os.WriteFile(lockAbsolutePath, payload.Bytes(), 0o600))
}

func writeSnapshotFile(t *testing.T, root string, snapshot routerFileSnapshot) {
	t.Helper()

	snapshotAbsolutePath := filepath.Join(root, filepath.FromSlash(routerSnapshotPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotAbsolutePath), 0o755))

	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(snapshotAbsolutePath, append(payload, '\n'), 0o600))
}

func checksumForFile(t *testing.T, root string, relativePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	require.NoError(t, err)

	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func runWrlkCommand(t *testing.T, targetRoot string, args ...string) commandResult {
	t.Helper()

	repoRoot := repositoryRoot(t)
	commandArgs := append([]string{"run", "./internal/router/tools/wrlk", "--root", targetRoot}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Dir = repoRoot

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := commandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}

	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if !assert.ErrorAs(t, err, &exitErr) {
		return result
	}

	result.exitCode = exitErr.ExitCode()
	return result
}

func copyRelativePath(t *testing.T, sourceRoot string, destinationRoot string, relativePath string) {
	t.Helper()

	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(relativePath))
	destinationPath := filepath.Join(destinationRoot, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(destinationPath), 0o755))
	copyFile(t, sourcePath, destinationPath)
}

func copyDirectory(t *testing.T, sourceDir string, destinationDir string) {
	t.Helper()

	require.NoError(t, filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destinationDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		copyFile(t, path, targetPath)
		return nil
	}))
}

func copyFile(t *testing.T, sourcePath string, destinationPath string) {
	t.Helper()

	sourceFile, err := os.Open(sourcePath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, sourceFile.Close())
	}()

	require.NoError(t, os.MkdirAll(filepath.Dir(destinationPath), 0o755))

	destinationFile, err := os.Create(destinationPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, destinationFile.Close())
	}()

	_, err = io.Copy(destinationFile, sourceFile)
	require.NoError(t, err)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
}
