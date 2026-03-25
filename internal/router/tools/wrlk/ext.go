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
	extOptionalRelPath    = "internal/router/ext/optional_extensions.go"
	extApplicationRelPath = "internal/router/ext/extensions.go"
	extExtensionsRelDir   = "internal/router/ext/extensions"
)

type extScaffoldSpec struct {
	commandPath          string
	compositionRelPath   string
	compositionVarName   string
	description          string
	dryRunTarget         string
	required             bool
	createPackage        bool
	snapshotReasonPrefix string
}

var (
	optionalExtensionSpec = extScaffoldSpec{
		commandPath:          "add",
		compositionRelPath:   extOptionalRelPath,
		compositionVarName:   "optionalExtensions",
		description:          "router capability extension",
		dryRunTarget:         "optional_extensions.go",
		required:             false,
		createPackage:        true,
		snapshotReasonPrefix: "wrlk ext add --name",
	}
	applicationExtensionSpec = extScaffoldSpec{
		commandPath:          "app add",
		compositionRelPath:   extApplicationRelPath,
		compositionVarName:   "extensions",
		description:          "application router extension",
		dryRunTarget:         "extensions.go",
		required:             true,
		createPackage:        false,
		snapshotReasonPrefix: "wrlk ext app add --name",
	}
)

// RouterRunExtCommand dispatches the `ext` command group.
func RouterRunExtCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteExtUsage(stdout)
	}

	switch args[0] {
	case "add":
		return RouterRunExtAddCommand(options, args[1:], stdout, stderr)
	case "app":
		return RouterRunExtAppCommand(options, args[1:], stdout, stderr)
	default:
		return &usageError{message: fmt.Sprintf("unknown ext subcommand %q", args[0])}
	}
}

// RouterWriteExtUsage prints the ext command usage message.
func RouterWriteExtUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext <subcommand>",
		"subcommands:",
		"  add       scaffold a new optional router capability extension package",
		"  app add   wire a required application extension into extensions.go",
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

// RouterRunExtAddCommand parses flags and orchestrates the optional extension scaffold.
func RouterRunExtAddCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionScaffoldCommand(
		options,
		args,
		stdout,
		stderr,
		optionalExtensionSpec,
	)
}

// RouterRunExtAppCommand dispatches the `ext app` command group.
func RouterRunExtAppCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteExtAppUsage(stdout)
	}

	switch args[0] {
	case "add":
		return RouterRunExtAppAddCommand(options, args[1:], stdout, stderr)
	default:
		return &usageError{message: fmt.Sprintf("unknown ext app subcommand %q", args[0])}
	}
}

// RouterWriteExtAppUsage prints the ext app usage message.
func RouterWriteExtAppUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext app <subcommand>",
		"subcommands:",
		"  add   wire a required application extension into extensions.go",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext app usage line: %w", err)
		}
	}

	return nil
}

// RouterRunExtAppAddCommand parses flags and orchestrates the application extension scaffold.
func RouterRunExtAppAddCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionScaffoldCommand(
		options,
		args,
		stdout,
		stderr,
		applicationExtensionSpec,
	)
}

// RouterRunExtensionScaffoldCommand parses flags and runs one scaffold variant.
func RouterRunExtensionScaffoldCommand(
	options globalOptions,
	args []string,
	stdout io.Writer,
	_ io.Writer,
	spec extScaffoldSpec,
) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWriteExtensionScaffoldUsage(stdout, spec)
	}

	addOptions, err := RouterParseExtAddFlags(args, spec)
	if err != nil {
		return &usageError{message: err.Error()}
	}

	modulePath, err := RouterReadModulePath(options.root)
	if err != nil {
		return fmt.Errorf("ext %s: %w", spec.commandPath, err)
	}

	if err := RouterAddExtension(options.root, modulePath, addOptions.name, addOptions.dryRun, stdout, spec); err != nil {
		return fmt.Errorf("ext %s: %w", spec.commandPath, err)
	}

	return nil
}

// RouterWriteExtensionScaffoldUsage prints one scaffold variant usage message.
func RouterWriteExtensionScaffoldUsage(writer io.Writer, spec extScaffoldSpec) error {
	lines := []string{
		fmt.Sprintf("usage: Router [--root PATH] ext %s [flags]", spec.commandPath),
		"flags:",
		fmt.Sprintf("  --name <ExtensionName>  package name for the new %s", spec.description),
		"  --dry-run               print changes without writing",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext scaffold usage line: %w", err)
		}
	}

	return nil
}

// RouterParseExtAddFlags parses the flags for extension scaffold commands.
func RouterParseExtAddFlags(args []string, spec extScaffoldSpec) (extAddOptions, error) {
	options := extAddOptions{}

	fs := flag.NewFlagSet("wrlk ext "+spec.commandPath, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.name, "name", "", "package name for the new extension")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing")

	if err := fs.Parse(args); err != nil {
		return extAddOptions{}, fmt.Errorf("parse ext %s flags: %w", spec.commandPath, err)
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

// RouterAddExtension scaffolds a new router extension package.
func RouterAddExtension(
	root string,
	modulePath string,
	name string,
	dryRun bool,
	stdout io.Writer,
	spec extScaffoldSpec,
) error {
	compositionPath := filepath.Join(root, filepath.FromSlash(spec.compositionRelPath))
	compositionContent, err := os.ReadFile(compositionPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	updatedComposition, err := RouterInjectExtension(
		compositionContent,
		name,
		modulePath,
		spec.compositionVarName,
	)
	if err != nil {
		return fmt.Errorf("inject extension into %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	if dryRun {
		return RouterWriteExtAddDryRunOutput(stdout, name, modulePath, spec)
	}

	if err := RouterWriteExtSnapshotBeforeMutation(
		root,
		fmt.Sprintf("%s %s", spec.snapshotReasonPrefix, name),
	); err != nil {
		return fmt.Errorf("write snapshot before ext scaffold: %w", err)
	}

	if spec.createPackage {
		extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), name)
		if _, err := os.Stat(extDir); err == nil {
			return fmt.Errorf("extension %q already exists at %s", name, filepath.Join(extExtensionsRelDir, name))
		}

		docPath := filepath.Join(extDir, "doc.go")
		docContent := RouterExtDocTemplate(name, spec)
		if err := RouterAtomicWriteFile(docPath, []byte(docContent)); err != nil {
			return fmt.Errorf("write extension doc.go for %s: %w", name, err)
		}

		extensionPath := filepath.Join(extDir, "extension.go")
		extensionContent := RouterExtExtensionTemplate(name, modulePath, spec)
		if err := RouterAtomicWriteFile(extensionPath, []byte(extensionContent)); err != nil {
			return fmt.Errorf("write extension.go for %s: %w", name, err)
		}
	}

	if err := RouterAtomicWriteFile(compositionPath, updatedComposition); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	successFormat := "wrlk: added %s %q at %s\n"
	successArgs := []any{spec.description, name, filepath.Join(extExtensionsRelDir, name)}
	if !spec.createPackage {
		successFormat = "wrlk: wired %s %q in %s\n"
		successArgs = []any{spec.description, name, spec.compositionRelPath}
	}

	if err := RouterWriteExtMessage(stdout, successFormat, successArgs...); err != nil {
		return fmt.Errorf("write ext scaffold success message: %w", err)
	}

	return nil
}

// RouterInjectExtension splices a new import and entry into one extension composition file.
func RouterInjectExtension(
	content []byte,
	name string,
	modulePath string,
	compositionVarName string,
) ([]byte, error) {
	src := string(content)
	importPath := modulePath + "/" + strings.ReplaceAll(extExtensionsRelDir, "\\", "/") + "/" + name

	importClosingIdx := strings.Index(src, "\n)")
	if importClosingIdx < 0 {
		return nil, fmt.Errorf("could not locate import block closing paren")
	}

	importLine := fmt.Sprintf("\t%q", importPath)
	if strings.Contains(src, importLine+"\n") || strings.Contains(src, importLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is already imported in %s", name, compositionVarName)
	}
	withImport := src[:importClosingIdx] + "\n" + importLine + src[importClosingIdx:]

	sliceMarker := fmt.Sprintf("var %s = []router.Extension{", compositionVarName)
	sliceStartIdx := strings.Index(withImport, sliceMarker)
	if sliceStartIdx < 0 {
		return nil, fmt.Errorf("could not locate %s slice", compositionVarName)
	}

	closingBrace := strings.Index(withImport[sliceStartIdx:], "\n}")
	if closingBrace < 0 {
		return nil, fmt.Errorf("could not locate %s slice closing brace", compositionVarName)
	}

	closingBrace += sliceStartIdx
	entryLine := fmt.Sprintf("\t&%s.Extension{},", name)
	if strings.Contains(withImport, entryLine+"\n") || strings.Contains(withImport, entryLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is already wired in %s", name, compositionVarName)
	}
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

// RouterExtDocTemplate returns the doc.go content for a new router extension.
func RouterExtDocTemplate(name string, spec extScaffoldSpec) string {
	comment := "Failure to boot results in an OptionalExtensionFailed warning; boot continues."
	if spec.required {
		comment = "Boot failure is fatal because application extensions are required."
	}

	return fmt.Sprintf(`// Package %s is a %s.
//
// Usage:
//   - Describe what this extension provides and when a consumer should depend on it.
//
// Package Concerns:
//   - Implements router.Extension only; keep wiring explicit.
//   - %s
package %s
`, name, spec.description, comment, name)
}

// RouterExtExtensionTemplate returns the extension.go content for a new router extension.
func RouterExtExtensionTemplate(name, modulePath string, spec extScaffoldSpec) string {
	requiredValue := "false"
	requiredComment := "optional"
	if spec.required {
		requiredValue = "true"
		requiredComment = "required"
	}

	portSuffix := RouterUppercaseFirst(name)

	return fmt.Sprintf(`package %s

import (
	"fmt"

	"log"

	"%s/internal/router"
)

// Extension is the %s for %s.
type Extension struct{}

// Required reports that the %s extension is %s.
func (e *Extension) Required() bool {
	return %s
}

// Consumes reports the ports this extension requires before it can boot.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports the ports this extension registers during boot.
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
	_ = fmt.Sprintf // suppress import error until TODO is resolved
	_ = reg

	return nil
}
`, name, modulePath, spec.description, name, name, requiredComment, requiredValue, name, portSuffix, name, name)
}

// RouterWriteExtAddDryRunOutput prints dry-run intent to stdout.
func RouterWriteExtAddDryRunOutput(
	stdout io.Writer,
	name string,
	modulePath string,
	spec extScaffoldSpec,
) error {
	extRelPath := filepath.Join(extExtensionsRelDir, name)
	importPath := modulePath + "/" + strings.ReplaceAll(extExtensionsRelDir, "\\", "/") + "/" + name

	lines := []string{
		fmt.Sprintf("wrlk dry-run: would add %s %q", spec.description, name),
		fmt.Sprintf("  %s - inject import %q and &%s.Extension{}", spec.compositionRelPath, importPath, name),
	}
	if spec.createPackage {
		lines = []string{
			fmt.Sprintf("wrlk dry-run: would add %s %q", spec.description, name),
			fmt.Sprintf("  %s - create extension package directory", extRelPath),
			fmt.Sprintf("  %s/doc.go - create package doc", extRelPath),
			fmt.Sprintf("  %s/extension.go - create Extension struct implementing router.Extension", extRelPath),
			fmt.Sprintf("  %s - inject import %q and &%s.Extension{}", spec.compositionRelPath, importPath, name),
		}
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

// RouterUppercaseFirst uppercases the first rune in a scaffold name for examples.
func RouterUppercaseFirst(value string) string {
	if value == "" {
		return value
	}

	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// RouterWriteExtSnapshotBeforeMutation captures router core files and extension
// composition files before an ext mutation.
func RouterWriteExtSnapshotBeforeMutation(root, reason string) error {
	extSnapshotFiles := append(
		append([]string(nil), snapshotRouterFiles...),
		extOptionalRelPath,
		extApplicationRelPath,
	)
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
