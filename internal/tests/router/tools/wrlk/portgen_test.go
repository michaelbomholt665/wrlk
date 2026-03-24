package wrlk_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// — Minimal router fixture file content —

const minimalPortsFile = `package router

// PortName is a typed router port identifier.
type PortName string

// Provider is the registered implementation for a router port.
type Provider any

const (
	// PortPrimary is the configuration provider port.
	PortPrimary PortName = "primary"
)
`

const minimalGoModFile = `module portgenfixture

go 1.24
`

const minimalValidationFile = `package router

import "sync/atomic"

var registry atomic.Pointer[map[PortName]Provider]

// RouterValidatePortName reports whether the port is declared in the router whitelist.
func RouterValidatePortName(port PortName) bool {
	switch port {
	case PortPrimary:
		return true
	default:
		return false
	}
}
`

const minimalExtensionFile = `package router

// RouterLoadExtensions loads extension registrations.
func RouterLoadExtensions() {}
`

const minimalRegistryFile = `package router

// RouterResolveProvider resolves a provider.
func RouterResolveProvider() {}
`

const (
	portsRelPath      = "internal/router/ports.go"
	validationRelPath = "internal/router/registry_imports.go"
	extensionRelPath  = "internal/router/extension.go"
	registryRelPath   = "internal/router/registry.go"
	lockRelPath       = "internal/router/router.lock"
	snapshotRelPath   = "internal/router/router.snapshot.json"
)

// TestPortgen_Add_UpdatesPortsFile verifies that a new port constant is injected into ports.go.
func TestPortgen_Add_UpdatesPortsFile(t *testing.T) {
	root := createPortgenFixture(t)

	result := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, result.err, result.stderr)
	require.Equal(t, 0, result.exitCode)

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)

	assert.Contains(t, string(content), `PortFoo PortName = "foo"`)
}

// TestPortgen_Add_UpdatesValidation verifies that a new switch case is injected into registry_imports.go.
func TestPortgen_Add_UpdatesValidation(t *testing.T) {
	root := createPortgenFixture(t)

	result := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, result.err, result.stderr)

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)

	assert.Contains(t, string(content), "case PortPrimary, PortFoo:")
}

// TestPortgen_Add_ValidationCompiles verifies the generated validation whitelist compiles
// and recognizes the newly added port at runtime.
func TestPortgen_Add_ValidationCompiles(t *testing.T) {
	root := createPortgenFixture(t)

	result := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, result.err, result.stderr)

	writeFixtureFile(t, root, "internal/router/registry_imports_generated_test.go", `package router

import "testing"

func TestGeneratedValidationIncludesAddedPort(t *testing.T) {
	if !RouterValidatePortName(PortFoo) {
		t.Fatal("PortFoo should be accepted")
	}
}
`)

	cmd := exec.Command("go", "test", "./internal/router", "-count=1")
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

// TestPortgen_Add_UpdatesLock verifies that router.lock is rewritten after a portgen add.
func TestPortgen_Add_UpdatesLock(t *testing.T) {
	root := createPortgenFixture(t)
	writeLockFixture(t, root)

	result := runPortgenCommand(t, root, "add", "--name", "PortBar", "--value", "bar")
	require.NoError(t, result.err, result.stderr)

	lockPath := filepath.Join(root, filepath.FromSlash(lockRelPath))
	require.FileExists(t, lockPath)

	records := loadLockRecords(t, lockPath)
	require.NotEmpty(t, records)

	// All tracked checksums must match current on-disk files.
	for _, record := range records {
		expected := checksumForFile(t, root, record.File)
		assert.Equal(t, expected, record.Checksum, "checksum mismatch for %s", record.File)
	}
}

// TestPortgen_Add_WritesSnapshot verifies that portgen captures a restorable snapshot before mutation.
func TestPortgen_Add_WritesSnapshot(t *testing.T) {
	root := createPortgenFixture(t)
	writeLockFixture(t, root)

	portsBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	validationBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)

	result := runPortgenCommand(t, root, "add", "--name", "PortBar", "--value", "bar")
	require.NoError(t, result.err, result.stderr)

	snapshotContent, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(snapshotRelPath)))
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
	assert.Contains(t, snapshot.Reason, "wrlk add")

	filesByPath := make(map[string]struct {
		Exists  bool
		Content string
	}, len(snapshot.Files))
	for _, file := range snapshot.Files {
		filesByPath[file.File] = struct {
			Exists  bool
			Content string
		}{
			Exists:  file.Exists,
			Content: file.Content,
		}
	}

	assert.Equal(t, string(portsBefore), filesByPath[portsRelPath].Content)
	assert.Equal(t, string(validationBefore), filesByPath[validationRelPath].Content)
	assert.True(t, filesByPath[lockRelPath].Exists)
}

// TestPortgen_AddRestoreVerifyWorkflow verifies the common add -> restore -> verify workflow.
func TestPortgen_AddRestoreVerifyWorkflow(t *testing.T) {
	root := createPortgenFixture(t)
	writeLockFixture(t, root)

	portsBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	validationBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)
	lockBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lockRelPath)))
	require.NoError(t, err)

	addResult := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, addResult.err, addResult.stderr)
	assert.Contains(t, addResult.stdout, "added port PortFoo")

	portsAfterAdd, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	assert.Contains(t, string(portsAfterAdd), `PortFoo PortName = "foo"`)

	restoreResult := runWrlkCommand(t, root, "lock", "restore")
	require.NoError(t, restoreResult.err, restoreResult.stderr)
	assert.Contains(t, restoreResult.stdout, "router snapshot restored")

	portsAfterRestore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(portsBefore), string(portsAfterRestore))

	validationAfterRestore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(validationBefore), string(validationAfterRestore))

	lockAfterRestore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lockRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(lockBefore), string(lockAfterRestore))

	verifyResult := runWrlkCommand(t, root, "lock", "verify")
	require.NoError(t, verifyResult.err, verifyResult.stderr)
	assert.Contains(t, verifyResult.stdout, "router lock verified")
}

// TestPortgen_AddRestoreAddAgain verifies that restore returns the fixture to a reusable pre-add state.
func TestPortgen_AddRestoreAddAgain(t *testing.T) {
	root := createPortgenFixture(t)
	writeLockFixture(t, root)

	firstAdd := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, firstAdd.err, firstAdd.stderr)

	restoreResult := runWrlkCommand(t, root, "lock", "restore")
	require.NoError(t, restoreResult.err, restoreResult.stderr)

	secondAdd := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, secondAdd.err, secondAdd.stderr)

	portsContent, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(portsContent), `PortFoo PortName = "foo"`))

	validationContent, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(validationContent), "PortFoo"))
}

// TestPortgen_Add_Idempotent verifies that adding the same port twice fails with an actionable error.
func TestPortgen_Add_Idempotent(t *testing.T) {
	root := createPortgenFixture(t)

	result := runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.NoError(t, result.err, result.stderr)

	result = runPortgenCommand(t, root, "add", "--name", "PortFoo", "--value", "foo")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)
	assert.True(
		t,
		strings.Contains(result.stderr, "PortFoo") || strings.Contains(result.stdout, "PortFoo"),
		"expected port name in output, got stdout=%q stderr=%q",
		result.stdout, result.stderr,
	)
}

// TestPortgen_Add_DuplicateName_Fails verifies that a constant name already present is rejected before writing.
func TestPortgen_Add_DuplicateName_Fails(t *testing.T) {
	root := createPortgenFixture(t)

	// PortPrimary already exists in the fixture.
	contentBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)

	result := runPortgenCommand(t, root, "add", "--name", "PortPrimary", "--value", "config2")
	require.Error(t, result.err)
	assert.NotEqual(t, 0, result.exitCode)

	// File must not have been modified.
	contentAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	assert.Equal(t, string(contentBefore), string(contentAfter), "ports.go must not be modified on duplicate-name failure")
}

// TestPortgen_Add_DryRun_NoWrite verifies that --dry-run prints intent without modifying any file.
func TestPortgen_Add_DryRun_NoWrite(t *testing.T) {
	root := createPortgenFixture(t)

	portsBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	validationBefore, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)

	result := runPortgenCommand(t, root, "add", "--name", "PortDry", "--value", "dry", "--dry-run")
	require.NoError(t, result.err, result.stderr)
	assert.Equal(t, 0, result.exitCode)

	// Output must indicate what would happen.
	combined := result.stdout + result.stderr
	assert.True(
		t,
		strings.Contains(combined, "PortDry") || strings.Contains(combined, "dry-run"),
		"expected dry-run indication in output, got %q",
		combined,
	)

	portsAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(portsRelPath)))
	require.NoError(t, err)
	validationAfter, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validationRelPath)))
	require.NoError(t, err)

	assert.Equal(t, string(portsBefore), string(portsAfter), "ports.go must not be written in dry-run mode")
	assert.Equal(t, string(validationBefore), string(validationAfter), "registry_imports.go must not be written in dry-run mode")
}

// — Fixture helpers —

func createPortgenFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", minimalGoModFile)
	writeFixtureFile(t, root, portsRelPath, minimalPortsFile)
	writeFixtureFile(t, root, validationRelPath, minimalValidationFile)
	writeFixtureFile(t, root, extensionRelPath, minimalExtensionFile)
	writeFixtureFile(t, root, registryRelPath, minimalRegistryFile)

	return root
}

func writeFixtureFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, []byte(content), 0o600))
}

func writeLockFixture(t *testing.T, root string) {
	t.Helper()

	lockAbsPath := filepath.Join(root, filepath.FromSlash(lockRelPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(lockAbsPath), 0o755))

	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	for _, rel := range []string{extensionRelPath, registryRelPath} {
		record := lockRecord{
			File:     rel,
			Checksum: checksumForFile(t, root, rel),
		}
		require.NoError(t, encoder.Encode(record))
	}

	require.NoError(t, os.WriteFile(lockAbsPath, payload.Bytes(), 0o600))
}

func loadLockRecords(t *testing.T, lockPath string) []lockRecord {
	t.Helper()

	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	records := make([]lockRecord, 0)
	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var rec lockRecord
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		records = append(records, rec)
	}

	return records
}

func runPortgenCommand(t *testing.T, targetRoot string, args ...string) commandResult {
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
