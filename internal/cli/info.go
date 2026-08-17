package cli

import (
	"fmt"
	"io"

	"asstools/internal/commands"
	"asstools/internal/terminal"
)

func runInfo(args []string, _ io.Reader, out, errOut io.Writer) int {
	if len(args) != 1 {
		return usageForCommand(errOut, "info", "info requires exactly one .ass file")
	}
	return commands.Info(args[0], out, errOut)
}

func printInfoHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst info <input.ass>"))
	fmt.Fprintln(out, "Print file metadata, sections, styles, fonts, events, and a compliance summary.")
}
