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
	path := ""
	stdin := false
	for _, arg := range args {
		switch arg {
		case "--strict":
			strict = true
			continue
		case "-", "--input":
			if path != "" {
				return usageForCommand(errOut, "info", "info accepts one .ass file or --input")
			}
			path = "-"
			stdin = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return usageForCommand(errOut, "info", fmt.Sprintf("unknown option %q", arg))
		}
		if path != "" {
			return usageForCommand(errOut, "info", "info accepts one .ass file or --input")
		}
		path = arg
	}
	if path == "" {
		return usageForCommand(errOut, "info", "info requires exactly one .ass file")
	}
	if stdin {
		return commands.InfoReaderWithOptions("-", in, strict, out, errOut)
	}
	return commands.InfoWithOptions(path, strict, out, errOut)
}

func printInfoHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst info [--strict] [-|--input|<input.ass>]"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Print file metadata, sections, styles, fonts, events, and a compliance summary.")
	fmt.Fprintln(out, "Use - or --input to read ASS data from standard input.")
	fmt.Fprintln(out, "By default info always exits 0 after a successful load; --strict exits 1 for compliance errors or unresolved manual items.")
}
