package commands

import (
	"fmt"
	"io"

	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func Check(path string, out, errOut io.Writer, ignoreVSFilterModWarnings ...bool) int {
	_, result, err := load(path, "auto")
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: %s", err)))
		return 2
	}
	if len(ignoreVSFilterModWarnings) > 0 && ignoreVSFilterModWarnings[0] {
		result.Diagnostics = withoutVSFilterModWarnings(result.Diagnostics)
	}
	if len(result.Diagnostics) == 0 {
		fmt.Fprintln(out, terminal.Color(out, terminal.Green, "No diagnostics."))
		fmt.Fprintln(out)
	} else {
		for _, diagnostic := range result.Diagnostics {
			style := terminal.Yellow
			if diagnostic.Manual {
				style = terminal.Magenta
			} else if diagnostic.Severity == "error" {
				style = terminal.Red
			}
			fmt.Fprintf(out, "%s:%d: %s[%s]: %s\n", path, diagnostic.Line, terminal.Color(out, style, string(diagnostic.Severity)), diagnostic.Code, diagnostic.Message)
		}
		fmt.Fprintln(out)
	}
	printSummary(out, result)
	if result.ErrorCount() > 0 {
		return 1
	}
	return 0
}

func withoutVSFilterModWarnings(diagnostics []rules.Diagnostic) []rules.Diagnostic {
	filtered := make([]rules.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "vsfiltermod-override" && diagnostic.Severity == rules.SeverityWarning {
			continue
		}
		filtered = append(filtered, diagnostic)
	}
	return filtered
}
