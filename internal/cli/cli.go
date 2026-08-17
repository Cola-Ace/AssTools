package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
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
		if command != "" && command != "info" && command != "check" && command != "normalize" {
			printUsageError(errOut, fmt.Sprintf("unknown command %q", command))
			printHelp(errOut, "")
			return ExitUsage
		}
		printHelp(out, command)
		return ExitOK
	}
	switch strings.ToLower(args[0]) {
	case "info":
		if len(args) != 2 {
			return usageForCommand(errOut, "info", "info requires exactly one .ass file")
		}
		return commands.Info(args[1], out, errOut)
	case "check":
		if len(args) != 2 {
			return usageForCommand(errOut, "check", "check requires exactly one .ass file")
		}
		return commands.Check(args[1], out, errOut)
	case "normalize":
		matrix := "auto"
		path := ""
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if arg == "--matrix" {
				if i+1 >= len(args) {
					return usageForCommand(errOut, "normalize", "--matrix requires a value")
				}
				matrix = args[i+1]
				i++
				continue
			}
			if strings.HasPrefix(arg, "--matrix=") {
				matrix = strings.TrimPrefix(arg, "--matrix=")
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return usageForCommand(errOut, "normalize", fmt.Sprintf("unknown option %q", arg))
			}
			if path != "" {
				return usageForCommand(errOut, "normalize", "normalize accepts one .ass file")
			}
			path = arg
		}
		if path == "" {
			return usageForCommand(errOut, "normalize", "normalize requires one .ass file")
		}
		if !strings.EqualFold(matrix, "auto") {
			if _, ok := canonicalMatrix(matrix); !ok {
				return usageForCommand(errOut, "normalize", fmt.Sprintf("invalid matrix value %q", matrix))
			}
		} else {
			matrix = "auto"
		}
		return commands.Normalize(path, matrix, in, out, errOut)
	default:
		printUsageError(errOut, fmt.Sprintf("unknown command %q", args[0]))
		printHelp(errOut, "")
		return ExitUsage
	}
}

func canonicalMatrix(value string) (string, bool) {
	values := map[string]string{
		"none": "None", "tv.601": "TV.601", "tv.709": "TV.709", "tv.240m": "TV.240M", "tv.fcc": "TV.FCC",
		"pc.601": "PC.601", "pc.709": "PC.709", "pc.240m": "PC.240M", "pc.fcc": "PC.FCC",
	}
	canonical, ok := values[strings.ToLower(strings.TrimSpace(value))]
	return canonical, ok
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
		fmt.Fprintln(out, "  asst info <input.ass>")
		fmt.Fprintln(out, "  asst check <input.ass>")
		fmt.Fprintln(out, "  asst normalize [--matrix <auto|value>] <input.ass>")
		fmt.Fprintln(out, "  asst help [command]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Commands:")
		fmt.Fprintln(out, "  info       summarize structure, timing, fonts, and compliance")
		fmt.Fprintln(out, "  check      print stable compliance diagnostics")
		fmt.Fprintln(out, "  normalize  preview safe edits, then optionally apply them")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Exit codes: 0 success/warnings/cancel, 1 compliance findings, 2 usage or I/O failure")
		return
	}
	switch command {
	case "info":
		fmt.Fprintln(out, "Usage: asst info <input.ass>")
		fmt.Fprintln(out, "Print file metadata, sections, styles, fonts, events, and a compliance summary.")
	case "check":
		fmt.Fprintln(out, "Usage: asst check <input.ass>")
		fmt.Fprintln(out, "Print diagnostics as path:line: severity[code]: message.")
	case "normalize":
		fmt.Fprintln(out, "Usage: asst normalize [--matrix <auto|value>] <input.ass>")
		fmt.Fprintln(out, "Preview safe edits and apply them only after a y/yes confirmation.")
		fmt.Fprintln(out, "The default matrix mode is auto; explicit values use canonical spelling.")
	}
}
