package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
	"asstools/internal/terminal"
)

func runCheck(args []string, _ io.Reader, out, errOut io.Writer) int {
	ignoreVSFilterMod := false
	path := ""
	for _, arg := range args {
		switch arg {
		case "--ignore-vsfiltermod", "--ignore-vsfiltermod-warning", "--ignore-vsfiltermod-warnings":
			ignoreVSFilterMod = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return usageForCommand(errOut, "check", fmt.Sprintf("unknown option %q", arg))
		}
		if path != "" {
			return usageForCommand(errOut, "check", "check accepts one .ass file")
		}
		path = arg
	}
	if path == "" {
		return usageForCommand(errOut, "check", "check requires exactly one .ass file")
	}
	return commands.Check(path, out, errOut, ignoreVSFilterMod)
}

func printCheckHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst check [--ignore-vsfiltermod] <input.ass>"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Print diagnostics as path:line: severity[code]: message.")
	fmt.Fprintln(out, "Use --ignore-vsfiltermod to hide VSFilterMod compatibility warnings; syntax errors remain visible.")
}
