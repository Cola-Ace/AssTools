package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
	"asstools/internal/terminal"
)

func runCheck(args []string, in io.Reader, out, errOut io.Writer) int {
	ignoreVSFilterMod := false
	jsonOutput := hasJSONFlag(args)
	path := ""
	stdin := false
	for _, arg := range args {
		switch arg {
		case "--ignore-vsfiltermod", "--ignore-vsfiltermod-warning", "--ignore-vsfiltermod-warnings":
			ignoreVSFilterMod = true
			continue
		case "--json":
			jsonOutput = true
			continue
		case "-", "--input":
			if path != "" {
				return usageForCommandOutput(out, errOut, "check", "check accepts one .ass file or --input", jsonOutput)
			}
			path = "-"
			stdin = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return usageForCommandOutput(out, errOut, "check", fmt.Sprintf("unknown option %q", arg), jsonOutput)
		}
		if path != "" {
			return usageForCommandOutput(out, errOut, "check", "check accepts one .ass file", jsonOutput)
		}
		path = arg
	}
	if path == "" {
		return usageForCommandOutput(out, errOut, "check", "check requires exactly one .ass file", jsonOutput)
	}
	if stdin {
		return commands.CheckReaderWithOptions("-", in, out, errOut, ignoreVSFilterMod, jsonOutput)
	}
	return commands.CheckWithOptions(path, out, errOut, ignoreVSFilterMod, jsonOutput)
}

func printCheckHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst check [--ignore-vsfiltermod] [--json] [-|--input|<input.ass>]"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Print diagnostics as path:line: severity[code]: message.")
	fmt.Fprintln(out, "Use --ignore-vsfiltermod to hide VSFilterMod compatibility warnings; syntax errors remain visible.")
	fmt.Fprintln(out, "Use - or --input to read ASS data from standard input.")
	fmt.Fprintln(out, "Use --json for a single machine-readable JSON document.")
}
