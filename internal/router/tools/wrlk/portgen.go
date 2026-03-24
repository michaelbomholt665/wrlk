// Package main implements the portgen CLI for the Port Router.
//
// Package Concerns:
//   - Single-action port registration: adds a constant to ports.go and a
//     switch case to registry_imports.go in one explicit command.
//   - Atomic writes for all three targets (ports.go, registry_imports.go, router.lock).
//   - Read-only dry-run mode for safe inspection.
//   - Zero third-party imports: stdlib only.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	portsRelPath      = "internal/router/ports.go"
	validationRelPath = "internal/router/registry_imports.go"
	lockRelPath       = "internal/router/router.lock"
)

// portConstantPattern matches an existing PortName constant declaration.
var portConstantPattern = regexp.MustCompile(`(?m)^\s*(\w+)\s+PortName\s*=\s*"([^"]+)"`)

// switchCasePattern matches the opening of the RouterValidatePortName switch.
var switchCasePattern = regexp.MustCompile(`(?m)(switch port \{)`)

// validationCaseLinePattern matches the router whitelist case line.
var validationCaseLinePattern = regexp.MustCompile(`(?m)^(\s*case\s+)([^:\n]+)(:)\s*$`)

// RouterRunPortgenCommand executes the portgen CLI as a wrlk subcommand.
func RouterRunPortgenCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWritePortgenUsage(stdout)
	}

	addOptions, err := RouterParsePortgenFlags(args)
	if err != nil {
		return &usageError{message: err.Error()}
	}

	if err := RouterAddPort(options.root, addOptions.name, addOptions.value, addOptions.dryRun, stdout); err != nil {
		return err
	}

	return nil
}

type portgenAddOptions struct {
	name   string
	value  string
	dryRun bool
}

// RouterParsePortgenFlags parses portgen add subcommand flags.
func RouterParsePortgenFlags(args []string) (portgenAddOptions, error) {
	options := portgenAddOptions{}

	fs := flag.NewFlagSet("wrlk add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.name, "name", "", "port constant name (e.g. PortFoo)")
	fs.StringVar(&options.value, "value", "", "port string value (e.g. foo)")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing")

	if err := fs.Parse(args); err != nil {
		return portgenAddOptions{}, fmt.Errorf("parse portgen add flags: %w", err)
	}

	if options.name == "" {
		return portgenAddOptions{}, fmt.Errorf("--name is required")
	}
	if options.value == "" {
		return portgenAddOptions{}, fmt.Errorf("--value is required")
	}

	return options, nil
}

// RouterAddPort is the top-level action: injects the constant, the switch case, and rewrites the lock.
func RouterAddPort(root, name, value string, dryRun bool, stdout io.Writer) error {
	portsPath := filepath.Join(root, filepath.FromSlash(portsRelPath))
	validationPath := filepath.Join(root, filepath.FromSlash(validationRelPath))

	portsContent, err := os.ReadFile(portsPath)
	if err != nil {
		return fmt.Errorf("read ports file: %w", err)
	}

	validationContent, err := os.ReadFile(validationPath)
	if err != nil {
		return fmt.Errorf("read validation file: %w", err)
	}

	if err := RouterCheckPortNotDuplicate(name, portsContent); err != nil {
		return err
	}

	updatedPorts, err := RouterInjectPortConstant(portsContent, name, value)
	if err != nil {
		return fmt.Errorf("inject port constant: %w", err)
	}

	updatedValidation, err := RouterInjectValidationCase(validationContent, name)
	if err != nil {
		return fmt.Errorf("inject validation case: %w", err)
	}

	if dryRun {
		return RouterWritePortgenDryRunOutput(stdout, name, value, portsRelPath, validationRelPath)
	}

	if err := RouterWriteSnapshotBeforeMutation(
		root,
		fmt.Sprintf("wrlk add --name %s --value %s", name, value),
	); err != nil {
		return fmt.Errorf("write snapshot before portgen: %w", err)
	}

	if err := RouterWritePortsFile(portsPath, updatedPorts); err != nil {
		return err
	}

	if err := RouterWriteValidationFile(validationPath, updatedValidation); err != nil {
		return err
	}

	if err := RouterWriteLockAfterPortgen(root); err != nil {
		return err
	}

	if err := RouterWritePortgenMessage(stdout, "wrlk: added port %s = %q\n", name, value); err != nil {
		return fmt.Errorf("write portgen success message: %w", err)
	}

	return nil
}

// RouterCheckPortNotDuplicate returns an error if the port constant name already exists.
func RouterCheckPortNotDuplicate(name string, portsContent []byte) error {
	matches := portConstantPattern.FindAllSubmatch(portsContent, -1)
	for _, match := range matches {
		if string(match[1]) == name {
			return fmt.Errorf("wrlk: port %q already declared in ports.go", name)
		}
	}

	return nil
}

// RouterInjectPortConstant appends a new PortName constant into the const block.
func RouterInjectPortConstant(content []byte, name, value string) ([]byte, error) {
	src := string(content)

	// Find the closing paren of the const block.
	closingIdx := strings.LastIndex(src, ")")
	if closingIdx < 0 {
		return nil, fmt.Errorf("could not locate const block closing paren in ports.go")
	}

	newLine := fmt.Sprintf("\t// %s is the %s provider port.\n\t%s PortName = %q\n", name, value, name, value)
	updated := src[:closingIdx] + newLine + src[closingIdx:]

	return []byte(updated), nil
}

// RouterInjectValidationCase injects a new case into RouterValidatePortName's switch.
func RouterInjectValidationCase(content []byte, name string) ([]byte, error) {
	src := string(content)

	// Confirm the expected RouterValidatePortName switch exists before editing.
	loc := switchCasePattern.FindStringIndex(src)
	if loc == nil {
		return nil, fmt.Errorf("could not locate switch port statement in registry_imports.go")
	}

	caseLoc := validationCaseLinePattern.FindStringSubmatchIndex(src)
	if caseLoc == nil {
		return nil, fmt.Errorf("could not locate validation case list in registry_imports.go")
	}

	listEnd := caseLoc[5]
	updated := src[:listEnd] + ", " + name + src[listEnd:]

	return []byte(updated), nil
}

// RouterWritePortsFile writes updated ports.go content atomically.
func RouterWritePortsFile(path string, content []byte) error {
	if err := RouterAtomicWriteFile(path, content); err != nil {
		return fmt.Errorf("write ports file %s: %w", path, err)
	}

	return nil
}

// RouterWriteValidationFile writes updated registry_imports.go content atomically.
func RouterWriteValidationFile(path string, content []byte) error {
	if err := RouterAtomicWriteFile(path, content); err != nil {
		return fmt.Errorf("write validation file %s: %w", path, err)
	}

	return nil
}

// RouterWriteLockAfterPortgen recomputes and rewrites router.lock using the same logic as wrlk lock update.
func RouterWriteLockAfterPortgen(root string) error {
	records, err := RouterComputeLockRecords(root)
	if err != nil {
		return fmt.Errorf("compute lock records after portgen: %w", err)
	}

	if err := RouterWriteLockRecords(root, records); err != nil {
		return fmt.Errorf("write lock file after portgen: %w", err)
	}

	return nil
}

// RouterAtomicWriteFile writes content to path using a temp-file-and-rename pattern.
func RouterAtomicWriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}

	tmpFile, err := os.CreateTemp(dir, "portgen.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}

	tmpPath := tmpFile.Name()
	writeErr := RouterWriteAndCloseTempFile(tmpFile, content)
	if writeErr != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("write temp file %s: %w (cleanup: %v)", tmpPath, writeErr, removeErr)
		}

		return writeErr
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}

	return nil
}

// RouterWriteAndCloseTempFile flushes and closes a temp file, cleaning up on error.
func RouterWriteAndCloseTempFile(file *os.File, content []byte) error {
	tmpPath := file.Name()

	if _, err := file.Write(content); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("write temp file %s: %w (close: %v)", tmpPath, err, closeErr)
		}

		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}

	if err := file.Sync(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("sync temp file %s: %w (close: %v)", tmpPath, err, closeErr)
		}

		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	return nil
}

// RouterWritePortgenDryRunOutput prints dry-run intent to stdout.
func RouterWritePortgenDryRunOutput(stdout io.Writer, name, value, portsRel, validationRel string) error {
	lines := []string{
		fmt.Sprintf("wrlk dry-run: would add port %s = %q", name, value),
		fmt.Sprintf("  %s — inject: %s PortName = %q", portsRel, name, value),
		fmt.Sprintf("  %s — inject: append %s to RouterValidatePortName", validationRel, name),
		fmt.Sprintf("  %s — rewrite with updated checksums", lockRelPath),
	}

	for _, line := range lines {
		if err := RouterWritePortgenMessage(stdout, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWritePortgenUsage prints the portgen CLI usage message.
func RouterWritePortgenUsage(w io.Writer) error {
	lines := []string{
		"usage: wrlk [--root PATH] add [flags]",
		"flags:",
		"  --name <ConstantName> --value <string> [--dry-run]",
	}

	for _, line := range lines {
		if err := RouterWritePortgenMessage(w, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWritePortgenMessage writes one formatted message.
func RouterWritePortgenMessage(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write wrlk message: %w", err)
	}

	return nil
}
