package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
	"asstools/internal/terminal"
)

func runInfo(args []string, in io.Reader, out, errOut io.Writer) int {
	strict := false
	jsonOutput := hasJSONFlag(args)
	path := ""
	stdin := false
	for _, arg := range args {
		switch arg {
		case "--strict":
			strict = true
			continue
		case "--json":
			jsonOutput = true
			continue
		case "-", "--input":
			if path != "" {
				return usageForCommandOutput(out, errOut, "info", "info accepts one .ass file or --input", jsonOutput)
			}
			path = "-"
			stdin = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return usageForCommandOutput(out, errOut, "info", fmt.Sprintf("unknown option %q", arg), jsonOutput)
		}
		if path != "" {
			return usageForCommandOutput(out, errOut, "info", "info accepts one .ass file or --input", jsonOutput)
		}
		path = arg
	}
	if path == "" {
		return usageForCommandOutput(out, errOut, "info", "info requires exactly one .ass file", jsonOutput)
	}
	if stdin {
		return commands.InfoReaderWithJSONOptions("-", in, strict, jsonOutput, out, errOut)
	}
	return commands.InfoWithJSONOptions(path, strict, jsonOutput, out, errOut)
}

func printInfoHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst info [--strict] [--json] [-|--input|<input.ass>]"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Print file metadata, sections, styles, fonts, events, and a compliance summary.")
	fmt.Fprintln(out, "Use - or --input to read ASS data from standard input.")
	fmt.Fprintln(out, "Use --json for a single machine-readable JSON document.")
	fmt.Fprintln(out, "By default info always exits 0 after a successful load; --strict exits 1 for compliance errors or unresolved manual items.")
}

func printInfoUsageHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst info [--strict] [-|--input|<input.ass>]"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Print file metadata, sections, styles, fonts, events, and a compliance summary.")
	fmt.Fprintln(out, "Use - or --input to read ASS data from standard input.")
	fmt.Fprintln(out, "By default info always exits 0 after a successful load; --strict exits 1 for compliance errors or unresolved manual items.")
}
