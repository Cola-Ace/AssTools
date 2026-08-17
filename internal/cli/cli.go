package cli

import (
	"fmt"
	"io"
	"strings"
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
	if handler, ok := commandFor(args[0]); ok {
		return handler(args[1:], in, out, errOut)
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
	fmt.Fprintf(out, "asst: %s\n", message)
}

func printHelp(out io.Writer, command string) {
	if command == "" {
		fmt.Fprintln(out, "asst - cross-platform ASS subtitle toolkit")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		for _, command := range commandRegistry() {
			fmt.Fprintf(out, "  %s\n", command.usage)
		}
		fmt.Fprintln(out, "  asst help [command]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Commands:")
		for _, command := range commandRegistry() {
			fmt.Fprintf(out, "  %-10s %s\n", command.name, command.summary)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Exit codes: 0 success/warnings/cancel, 1 compliance findings, 2 usage or I/O failure")
		return
	}
	if commandSpec, ok := commandNamed(command); ok {
		commandSpec.help(out)
	}
}
