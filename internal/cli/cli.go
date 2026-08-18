package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/terminal"
)

const (
	ExitOK    = 0
	ExitCheck = 1
	ExitUsage = 2
)

func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		printHelp(out, "")
		return ExitOK
	}
	if args[0] == "-h" || args[0] == "--help" {
		if len(args) > 1 {
			printUsageError(errOut, "unexpected arguments after --help")
			printHelp(errOut, "")
			return ExitUsage
		}
		printHelp(out, "")
		return ExitOK
	}
	if strings.EqualFold(args[0], "help") {
		if len(args) > 2 {
			printUsageError(errOut, "help accepts at most one command")
			printHelp(errOut, "")
			return ExitUsage
		}
		command := ""
		if len(args) == 2 {
			command = args[1]
		}
		if command != "" {
			if _, ok := commandNamed(command); !ok {
				printUsageError(errOut, fmt.Sprintf("unknown command %q", command))
				printHelp(errOut, "")
				return ExitUsage
			}
		}
		printHelp(out, command)
		return ExitOK
	}
	if command, ok := commandNamed(args[0]); ok {
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			command.help(out)
			return ExitOK
		}
		return command.handler(args[1:], in, out, errOut)
	}
	printUsageError(errOut, fmt.Sprintf("unknown command %q", args[0]))
	printHelp(errOut, "")
	return ExitUsage
}

func usageForCommand(errOut io.Writer, command, message string) int {
	printUsageError(errOut, message)
	printHelp(errOut, command)
	return ExitUsage
}

func printUsageError(out io.Writer, message string) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Red, fmt.Sprintf("asst: %s", message)))
	fmt.Fprintln(out)
}

func printHelp(out io.Writer, command string) {
	if command == "" {
		fmt.Fprintln(out, terminal.Color(out, terminal.Bold+terminal.Cyan, "asst - cross-platform ASS subtitle toolkit"))
		fmt.Fprintln(out)
		fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage:"))
		for _, command := range commandRegistry() {
			fmt.Fprintf(out, "  %s\n", command.usage)
		}
		fmt.Fprintln(out, "  asst help [command]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Commands:"))
		for _, command := range commandRegistry() {
			fmt.Fprintf(out, "  %-10s %s\n", command.name, command.summary)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, terminal.Color(out, terminal.Dim, "Exit codes: 0 = success, warnings, cancellation, or non-strict info findings; 1 = compliance errors, unresolved manual items, or strict info findings; 2 = usage, encoding, I/O, backup, or replacement failures"))
		return
	}
	if commandSpec, ok := commandNamed(command); ok {
		commandSpec.help(out)
	}
}
