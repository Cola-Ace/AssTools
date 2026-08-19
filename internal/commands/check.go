package commands

import (
	"fmt"
	"io"

	"asstools/internal/output"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func Check(path string, out, errOut io.Writer, ignoreVSFilterModWarnings ...bool) int {
	return CheckWithOptions(path, out, errOut, firstBool(ignoreVSFilterModWarnings), false)
}

func CheckReader(path string, in io.Reader, out, errOut io.Writer, ignoreVSFilterModWarnings ...bool) int {
	return CheckReaderWithOptions(path, in, out, errOut, firstBool(ignoreVSFilterModWarnings), false)
}

func CheckWithOptions(path string, out, errOut io.Writer, ignoreVSFilterModWarnings, jsonOutput bool) int {
	return check(path, nil, out, errOut, ignoreVSFilterModWarnings, jsonOutput)
}

func CheckReaderWithOptions(path string, in io.Reader, out, errOut io.Writer, ignoreVSFilterModWarnings, jsonOutput bool) int {
	return check(path, in, out, errOut, ignoreVSFilterModWarnings, jsonOutput)
}

func firstBool(values []bool) bool {
	return len(values) > 0 && values[0]
}

func check(path string, in io.Reader, out, errOut io.Writer, ignoreVSFilterModWarnings, jsonOutput bool) (code int) {
	path = cleanPath(path)
	trackedOut := output.Track(out)
	trackedErrOut := output.Track(errOut)
	out = trackedOut
	errOut = trackedErrOut
	defer func() {
		if trackedOut.Err() != nil || trackedErrOut.Err() != nil {
			code = 2
		}
	}()
	var result rules.Result
	var err error
	if in == nil {
		_, result, err = load(path, "auto")
	} else {
		_, result, err = loadReader(in, "auto")
	}
	if err != nil {
		if jsonOutput {
			if jsonErr := writeJSONError(errOut, "check", err); jsonErr != nil {
				return 2
			}
			return 2
		}
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: %s", err)))
		return 2
	}
	if ignoreVSFilterModWarnings {
		result.Diagnostics = withoutVSFilterModWarnings(result.Diagnostics)
	}
	if jsonOutput {
		payload := checkJSON{
			Command:     "check",
			Status:      resultStatus(result),
			Path:        path,
			Summary:     summaryJSON(result),
			Diagnostics: diagnosticsJSON(result.Diagnostics),
		}
		if err := encodeJSON(out, payload); err != nil {
			return 2
		}
		if result.ErrorCount() > 0 {
			return 1
		}
		return 0
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
	if err := printSummary(out, result); err != nil {
		return 2
	}
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
