package commands

import (
	"fmt"
	"io"
)

func Check(path string, out, errOut io.Writer) int {
	_, result, err := load(path, "auto")
	if err != nil {
		fmt.Fprintf(errOut, "asst: %s\n", err)
		return 2
	}
	if len(result.Diagnostics) == 0 {
		fmt.Fprintln(out, "No diagnostics.")
		fmt.Fprintln(out)
	} else {
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintf(out, "%s:%d: %s[%s]: %s\n", path, diagnostic.Line, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
		}
		fmt.Fprintln(out)
	}
	printSummary(out, result)
	if result.ErrorCount() > 0 {
		return 1
	}
	return 0
}
