package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"asstools/internal/output"
	"asstools/internal/terminal"
)

const (
	ExitOK    = 0
	ExitCheck = 1
	ExitUsage = 2
)

func Run(args []string, in io.Reader, out, errOut io.Writer) (code int) {
	trackedOut := output.Track(out)
	trackedErrOut := output.Track(errOut)
	out = trackedOut
	errOut = trackedErrOut
	defer func() {
		if trackedOut.Err() != nil || trackedErrOut.Err() != nil {
			code = ExitUsage
		}
	}()
	if len(args) == 0 {
		printHelp(out, "")
		return ExitOK
	}
	if args[0] == "-h" || args[0] == "--help" {
		if len(args) == 2 && args[1] == "--json" {
			return printHelpJSON(out, "")
		}
		if len(args) > 1 {
			printUsageError(errOut, "unexpected arguments after --help")
			printHelp(errOut, "")
			return ExitUsage
		}
		printHelp(out, "")
		return ExitOK
	}
	if strings.EqualFold(args[0], "help") {
		jsonOutput := false
		command := ""
		for _, arg := range args[1:] {
			if arg == "--json" {
				jsonOutput = true
				continue
			}
			if command != "" {
				printUsageError(errOut, "help accepts at most one command")
				printHelp(errOut, "")
				return ExitUsage
			}
			command = arg
		}
		if command != "" {
			if _, ok := commandNamed(command); !ok {
				printUsageError(errOut, fmt.Sprintf("unknown command %q", command))
				printHelp(errOut, "")
				return ExitUsage
			}
			if jsonOutput {
				return printHelpJSON(out, command)
			}
		}
		if jsonOutput {
			return printHelpJSON(out, command)
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
	if strings.EqualFold(command, "info") {
		printInfoUsageHelp(errOut)
		return ExitUsage
	}
	printHelp(errOut, command)
	return ExitUsage
}

func usageForCommandOutput(out, errOut io.Writer, command, message string, jsonOutput bool) int {
	if !jsonOutput {
		return usageForCommand(errOut, command, message)
	}
	payload := struct {
		Command string `json:"command"`
		Status  string `json:"status"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}{Command: command, Status: "error"}
	payload.Error.Message = message
	encoder := json.NewEncoder(errOut)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return ExitUsage
	}
	return ExitUsage
}

func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
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
		fmt.Fprintln(out, "  asst help [--json] [command]")
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

type helpJSON struct {
	Command  string            `json:"command"`
	Status   string            `json:"status"`
	Target   string            `json:"target"`
	Usage    string            `json:"usage"`
	Summary  string            `json:"summary"`
	Commands []helpCommandJSON `json:"commands"`
}

type helpCommandJSON struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
}

func printHelpJSON(out io.Writer, target string) int {
	payload := helpJSON{Command: "help", Status: "ok", Target: target, Commands: make([]helpCommandJSON, 0)}
	if target == "" {
		for _, command := range commandRegistry() {
			payload.Commands = append(payload.Commands, helpCommandJSON{Name: command.name, Usage: command.usage, Summary: command.summary})
		}
		payload.Commands = append(payload.Commands, helpCommandJSON{Name: "help", Usage: "asst help [--json] [command]", Summary: "show command help"})
	} else if target == "help" {
		payload.Usage = "asst help [--json] [command]"
		payload.Summary = "show command help"
	} else if command, ok := commandNamed(target); ok {
		payload.Usage = command.usage
		payload.Summary = command.summary
	} else {
		return ExitUsage
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return ExitUsage
	}
	return ExitOK
}
