package cli

import (
	"fmt"
	"io"

	"asstools/internal/commands"
)

func runCheck(args []string, _ io.Reader, out, errOut io.Writer) int {
	if len(args) != 1 {
		return usageForCommand(errOut, "check", "check requires exactly one .ass file")
	}
	return commands.Check(args[0], out, errOut)
}

func printCheckHelp(out io.Writer) {
	fmt.Fprintln(out, "Usage: asst check <input.ass>")
	fmt.Fprintln(out, "Print diagnostics as path:line: severity[code]: message.")
}
