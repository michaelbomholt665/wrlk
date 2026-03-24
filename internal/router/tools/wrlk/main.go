package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	exitCodeSuccess     = 0
	exitCodeFailure     = 1
	exitCodeUsage       = 2
	exitCodeInternalBug = 3
	defaultRootPath     = "."
	defaultLiveTimeout  = "15s"
)

// main runs the Router CLI entrypoint.
func main() {
	os.Exit(RouterRunCLIProcess(os.Args[1:], os.Stdout, os.Stderr))
}

// routerWriteAndReturn writes a formatted message to w; if the write fails it
// returns exitCodeInternalBug, otherwise it returns successCode.
func routerWriteAndReturn(w io.Writer, successCode int, format string, args ...any) int {
	if err := RouterWriteCLIMessage(w, format, args...); err != nil {
		return exitCodeInternalBug
	}

	return successCode
}

// RouterRunCLIProcess executes the CLI and returns a process exit code.
func RouterRunCLIProcess(args []string, stdout io.Writer, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	if handledCode, handled := RouterHandleTopLevelUsage(args, stdout, stderr); handled {
		return handledCode
	}

	options, remainingArgs, err := RouterParseGlobalOptions(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if usageErr := RouterWriteCLIUsage(stdout); usageErr != nil {
				return exitCodeInternalBug
			}
			return exitCodeSuccess
		}

		return routerWriteAndReturn(stderr, exitCodeUsage, "Router usage error: %s\n", err)
	}

	if len(remainingArgs) > 0 && RouterIsHelpToken(remainingArgs[0]) {
		if usageErr := RouterWriteCLIUsage(stdout); usageErr != nil {
			return exitCodeInternalBug
		}
		return exitCodeSuccess
	}

	if len(remainingArgs) == 0 {
		if usageErr := RouterWriteCLIUsage(stderr); usageErr != nil {
			return exitCodeInternalBug
		}
		return exitCodeUsage
	}

	err = RouterDispatchCLICommand(options, remainingArgs, stdout, stderr)
	return RouterMapCommandResult(err, stderr)
}

type globalOptions struct {
	root string
}

// RouterParseGlobalOptions parses flags shared by all command groups.
func RouterParseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := globalOptions{}

	fs := flag.NewFlagSet("Router", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.root, "root", defaultRootPath, "repository root")

	if err := fs.Parse(args); err != nil {
		return globalOptions{}, nil, fmt.Errorf("parse global flags: %w", err)
	}

	return options, fs.Args(), nil
}

// RouterDispatchCLICommand routes the parsed command tree to a concrete handler.
func RouterDispatchCLICommand(
	options globalOptions,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	switch args[0] {
	case "lock":
		return RouterRunLockCommand(options, args[1:], stdout)
	case "live":
		return RouterRunLiveCommand(options, args[1:], stdout, stderr)
	case "add":
		return RouterRunPortgenCommand(options, args[1:], stdout, stderr)
	case "guide":
		return RouterWriteGuide(stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown command %q", args[0])}
	}
}

// RouterWriteCLIUsage prints the top-level CLI usage message.
func RouterWriteCLIUsage(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, "usage: Router [--root PATH] <command> <subcommand> [flags]"); err != nil {
		return fmt.Errorf("write CLI usage header: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "commands:"); err != nil {
		return fmt.Errorf("write CLI usage commands header: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  lock verify"); err != nil {
		return fmt.Errorf("write CLI usage lock verify command: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  lock update"); err != nil {
		return fmt.Errorf("write CLI usage lock update command: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  lock restore"); err != nil {
		return fmt.Errorf("write CLI usage lock restore command: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  live run"); err != nil {
		return fmt.Errorf("write CLI usage live run command: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  add"); err != nil {
		return fmt.Errorf("write CLI usage add command: %w", err)
	}
	if _, err := fmt.Fprintln(writer, "  guide"); err != nil {
		return fmt.Errorf("write CLI usage guide command: %w", err)
	}

	return nil
}

// RouterWriteGuide prints a concise operational guide for the router tooling.
func RouterWriteGuide(writer io.Writer) error {
	lines := []string{
		"Router guide:",
		"  - Use `wrlk add --name <PortName> --value <string>` to add a new router port.",
		"  - `wrlk add` writes a local restore snapshot before mutating router files.",
		"  - Use `wrlk lock verify` to detect drift in checksum-tracked router core files.",
		"  - Use `wrlk lock update` only when intentional router core changes are accepted.",
		"  - Use `wrlk lock restore` to restore the previous local router snapshot.",
		"  - The router core stays contract-blind; business logic must resolve and cast to typed port contracts.",
		"  - `Any` is acceptable only in contract-blind infrastructure or explicit relaxed policy wiring, not business logic.",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write router guide line: %w", err)
		}
	}

	return nil
}

// RouterHandleTopLevelUsage handles top-level help and empty-argument usage flows.
func RouterHandleTopLevelUsage(args []string, stdout io.Writer, stderr io.Writer) (int, bool) {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		if usageErr := RouterWriteCLIUsage(stdout); usageErr != nil {
			return exitCodeInternalBug, true
		}
		return exitCodeSuccess, true
	}

	if len(args) == 0 {
		if usageErr := RouterWriteCLIUsage(stderr); usageErr != nil {
			return exitCodeInternalBug, true
		}
		return exitCodeUsage, true
	}

	return exitCodeSuccess, false
}

// RouterMapCommandResult converts a command error into the correct process exit code.
func RouterMapCommandResult(err error, stderr io.Writer) int {
	if err == nil {
		return exitCodeSuccess
	}

	var bugErr *verificationBugError
	if errors.As(err, &bugErr) {
		return routerWriteAndReturn(stderr, exitCodeInternalBug, "Router internal failure: %s\n", err)
	}

	var usageErr *usageError
	if errors.As(err, &usageErr) {
		return routerWriteAndReturn(stderr, exitCodeUsage, "Router usage error: %s\n", err)
	}

	return routerWriteAndReturn(stderr, exitCodeFailure, "%s\n", err)
}

// RouterWriteCLIMessage writes one formatted CLI message.
func RouterWriteCLIMessage(writer io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(writer, format, args...); err != nil {
		return fmt.Errorf("write CLI message: %w", err)
	}

	return nil
}

type usageError struct {
	message string
}

// Error returns the usage error message.
func (e *usageError) Error() string {
	return e.message
}

// RouterIsHelpToken reports whether the provided argument is a conventional help token.
func RouterIsHelpToken(value string) bool {
	switch value {
	case "--help", "-h", "help":
		return true
	default:
		return false
	}
}
