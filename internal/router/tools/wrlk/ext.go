package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	extOptionalRelPath  = "internal/router/ext/optional_extensions.go"
	extExtensionsRelDir = "internal/router/ext/extensions"
)

// RouterRunExtCommand dispatches the `ext` command group.
func RouterRunExtCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteExtUsage(stdout)
	}

	switch args[0] {
	case "add":
		return RouterRunExtAddCommand(options, args[1:], stdout, stderr)
	default:
		return &usageError{message: fmt.Sprintf("unknown ext subcommand %q", args[0])}
	}
}

// RouterWriteExtUsage prints the ext command usage message.
func RouterWriteExtUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext <subcommand>",
		"subcommands:",
		"  add   scaffold a new router capability extension package",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext usage line: %w", err)
		}
	}

	return nil
}

type extAddOptions struct {
	name   string
	dryRun bool
}

// RouterRunExtAddCommand parses flags and orchestrates the extension scaffold.
func RouterRunExtAddCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWriteExtAddUsage(stdout)
	}

	addOptions, err := RouterParseExtAddFlags(args)
	if err != nil {
		return &usageError{message: err.Error()}
	}

	modulePath, err := RouterReadModulePath(options.root)
	if err != nil {
		return fmt.Errorf("ext add: %w", err)
	}

	if err := RouterAddExtension(options.root, modulePath, addOptions.name, addOptions.dryRun, stdout); err != nil {
		return err
	}

	return nil
}

// RouterWriteExtAddUsage prints the ext add subcommand usage message.
func RouterWriteExtAddUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext add [flags]",
		"flags:",
		"  --name <ExtensionName>  package name for the new router capability extension",
		"  --dry-run               print changes without writing",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext add usage line: %w", err)
		}
	}

	return nil
}

// RouterParseExtAddFlags parses the flags for the ext add subcommand.
func RouterParseExtAddFlags(args []string) (extAddOptions, error) {
	options := extAddOptions{}

	fs := flag.NewFlagSet("wrlk ext add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.name, "name", "", "package name for the new router capability extension")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing")

	if err := fs.Parse(args); err != nil {
		return extAddOptions{}, fmt.Errorf("parse ext add flags: %w", err)
	}

	if options.name == "" {
		return extAddOptions{}, fmt.Errorf("--name is required")
	}

	if err := RouterValidateExtensionName(options.name); err != nil {
		return extAddOptions{}, err
	}

	return options, nil
}

// RouterValidateExtensionName returns an error if name is not a valid Go package name.
func RouterValidateExtensionName(name string) error {
	if name == "" {
		return fmt.Errorf("extension name must not be empty")
	}

	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("extension name %q must start with a letter", name)
		}

		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("extension name %q must contain only letters and digits (got %q)", name, r)
		}
	}

	return nil
}

// RouterAddExtension scaffolds a new router capability extension package.
func RouterAddExtension(root, modulePath, name string, dryRun bool, stdout io.Writer) error {
	extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), name)

	if _, err := os.Stat(extDir); err == nil {
		return fmt.Errorf("wrlk: extension %q already exists at %s", name, filepath.Join(extExtensionsRelDir, name))
	}

	optionalPath := filepath.Join(root, filepath.FromSlash(extOptionalRelPath))
	optionalContent, err := os.ReadFile(optionalPath)
	if err != nil {
		return fmt.Errorf("read optional_extensions.go: %w", err)
	}

	updatedOptional, err := RouterInjectOptionalExtension(optionalContent, name, modulePath)
	if err != nil {
		return fmt.Errorf("inject optional extension: %w", err)
	}

	if dryRun {
		return RouterWriteExtAddDryRunOutput(stdout, name, root, modulePath)
	}

	if err := RouterWriteExtSnapshotBeforeMutation(
		root,
		fmt.Sprintf("wrlk ext add --name %s", name),
	); err != nil {
		return fmt.Errorf("write snapshot before ext add: %w", err)
	}

	docPath := filepath.Join(extDir, "doc.go")
	docContent := RouterExtDocTemplate(name)

	if err := RouterAtomicWriteFile(docPath, []byte(docContent)); err != nil {
		return fmt.Errorf("write extension doc.go for %s: %w", name, err)
	}

	extensionPath := filepath.Join(extDir, "extension.go")
	extensionContent := RouterExtExtensionTemplate(name, modulePath)

	if err := RouterAtomicWriteFile(extensionPath, []byte(extensionContent)); err != nil {
		return fmt.Errorf("write extension.go for %s: %w", name, err)
	}

	if err := RouterAtomicWriteFile(optionalPath, updatedOptional); err != nil {
		return fmt.Errorf("write optional_extensions.go: %w", err)
	}

	if err := RouterWriteExtMessage(stdout, "wrlk: added router extension %q at %s\n", name, filepath.Join(extExtensionsRelDir, name)); err != nil {
		return fmt.Errorf("write ext add success message: %w", err)
	}

	return nil
}

// RouterInjectOptionalExtension splices a new import and entry into optional_extensions.go.
func RouterInjectOptionalExtension(content []byte, name, modulePath string) ([]byte, error) {
	src := string(content)
	importPath := modulePath + "/" + strings.ReplaceAll(extExtensionsRelDir, "\\", "/") + "/" + name

	// Inject the import. Find the closing paren of the existing import block.
	importClosingIdx := strings.Index(src, "\n)")
	if importClosingIdx < 0 {
		return nil, fmt.Errorf("could not locate import block closing paren in optional_extensions.go")
	}

	importLine := fmt.Sprintf("\t%q", importPath)
	withImport := src[:importClosingIdx] + "\n" + importLine + src[importClosingIdx:]

	// Inject the entry into the optionalExtensions var slice.
	// Find the last entry before the closing brace of the slice literal.
	closingBrace := strings.LastIndex(withImport, "\n}")
	if closingBrace < 0 {
		return nil, fmt.Errorf("could not locate optionalExtensions slice closing brace in optional_extensions.go")
	}

	entryLine := fmt.Sprintf("\t&%s.Extension{},", name)
	result := withImport[:closingBrace] + "\n" + entryLine + withImport[closingBrace:]

	return []byte(result), nil
}

// RouterReadModulePath reads the module path from go.mod in the given root.
func RouterReadModulePath(root string) (string, error) {
	goModPath := filepath.Join(root, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// RouterExtDocTemplate returns the doc.go content for a new router capability extension.
func RouterExtDocTemplate(name string) string {
	return fmt.Sprintf(`// Package %s is a router capability extension.
//
// Package Concerns:
//   - Implements router.Extension only; no imports from internal/adapters or internal/ports.
//   - Failure to boot results in an OptionalExtensionFailed warning; boot continues.
package %s
`, name, name)
}

// RouterExtExtensionTemplate returns the extension.go content for a new router capability extension.
func RouterExtExtensionTemplate(name, modulePath string) string {
	return fmt.Sprintf(`package %s

import (
	"fmt"

	"log"

	"%s/internal/router"
)

// Extension is the optional router capability extension for %s.
// It satisfies router.Extension and registers a provider under a router port
// before application extensions boot.
type Extension struct{}

// Required reports that the %s extension is optional.
// A boot failure produces an OptionalExtensionFailed warning; boot continues.
func (e *Extension) Required() bool {
	return false
}

// Consumes reports the ports this extension requires before it can boot.
// Update this slice when the extension depends on ports provided by other extensions.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports the ports this extension registers during boot.
// Update this slice to match the port registered in RouterProvideRegistration.
func (e *Extension) Provides() []router.PortName {
	return nil
}

// RouterProvideRegistration registers the %s provider into the boot registry.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	// TODO: replace with the actual port and provider.
	// Example:
	//   if err := reg.RouterRegisterProvider(router.Port%s, yourProvider); err != nil {
	//       return fmt.Errorf("%s extension: %%w", err)
	//   }
	log.Printf("%s extension initialized")
	_ = fmt.Sprintf  // suppress import error until TODO is resolved
	_ = reg

	return nil
}
`, name, modulePath, name, name, name, strings.Title(name), name, name)
}

// RouterWriteExtAddDryRunOutput prints dry-run intent to stdout.
func RouterWriteExtAddDryRunOutput(stdout io.Writer, name, root, modulePath string) error {
	extRelPath := filepath.Join(extExtensionsRelDir, name)

	lines := []string{
		fmt.Sprintf("wrlk dry-run: would add router extension %q", name),
		fmt.Sprintf("  %s — create extension package directory", extRelPath),
		fmt.Sprintf("  %s/doc.go — create package doc", extRelPath),
		fmt.Sprintf("  %s/extension.go — create Extension struct implementing router.Extension", extRelPath),
		fmt.Sprintf("  %s — inject import %q and &%s.Extension{}", extOptionalRelPath, modulePath+"/"+strings.ReplaceAll(extExtensionsRelDir, "\\", "/")+"/"+name, name),
	}

	for _, line := range lines {
		if err := RouterWriteExtMessage(stdout, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWriteExtMessage writes one formatted ext message.
func RouterWriteExtMessage(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write wrlk ext message: %w", err)
	}

	return nil
}

// RouterWriteExtSnapshotBeforeMutation captures router core files and
// optional_extensions.go before an ext add mutation. It extends the standard
// snapshot with extOptionalRelPath so that lock restore can also recover the
// composition file, which lives outside the core router kernel.
func RouterWriteExtSnapshotBeforeMutation(root, reason string) error {
	extSnapshotFiles := append(append([]string(nil), snapshotRouterFiles...), extOptionalRelPath)
	snapshot, err := RouterCaptureNamedSnapshot(root, reason, extSnapshotFiles)
	if err != nil {
		return err
	}

	if err := RouterWriteSnapshot(root, snapshot); err != nil {
		return err
	}

	return nil
}

// RouterCaptureNamedSnapshot builds a snapshot for an explicit list of relative file paths.
func RouterCaptureNamedSnapshot(root, reason string, files []string) (routerFileSnapshot, error) {
	snapshotFiles := make([]routerSnapshotFile, 0, len(files))

	for _, relativePath := range files {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		content, err := os.ReadFile(absolutePath)
		if err != nil {
			if os.IsNotExist(err) {
				snapshotFiles = append(snapshotFiles, routerSnapshotFile{
					File:   relativePath,
					Exists: false,
				})
				continue
			}

			return routerFileSnapshot{}, fmt.Errorf("read ext snapshot file %s: %w", relativePath, err)
		}

		snapshotFiles = append(snapshotFiles, routerSnapshotFile{
			File:    relativePath,
			Exists:  true,
			Content: string(content),
		})
	}

	return routerFileSnapshot{
		CreatedAt: RouterSnapshotTimestamp(),
		Reason:    reason,
		Files:     snapshotFiles,
	}, nil
}
