package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"asstools/internal/ass"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func load(path, matrixMode string) (*ass.Document, rules.Result, error) {
	path = cleanPath(path)
	if !strings.EqualFold(filepath.Ext(path), ".ass") {
		return nil, rules.Result{}, fmt.Errorf("input must have a .ass extension")
	}
	source, _, err := ass.Load(path)
	if err != nil {
		return nil, rules.Result{}, err
	}
	doc, err := ass.Parse(source)
	if err != nil {
		return nil, rules.Result{}, err
	}
	return doc, rules.Run(doc, matrixMode), nil
}

func cleanPath(path string) string {
	if path == "" || path == "-" {
		return path
	}
	if strings.HasPrefix(path, `.\`) {
		path = "./" + path[2:]
	}
	if strings.HasPrefix(path, "./") {
		return filepath.Clean(path)
	}
	return path
}

func loadReader(in io.Reader, matrixMode string) (*ass.Document, rules.Result, error) {
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, rules.Result{}, err
	}
	return checkBytes(data, matrixMode)
}

func checkBytes(data []byte, matrixMode string) (*ass.Document, rules.Result, error) {
	source, err := ass.ParseBytes(data)
	if err != nil {
		return nil, rules.Result{}, err
	}
	doc, err := ass.Parse(source)
	if err != nil {
		return nil, rules.Result{}, err
	}
	return doc, rules.Run(doc, matrixMode), nil
}

func printSummary(out io.Writer, result rules.Result) error {
	status := "compliant"
	statusStyle := terminal.Green
	if result.ErrorCount() > 0 {
		status = "errors found"
		statusStyle = terminal.Red
	} else if result.ManualCount() > 0 {
		status = "manual review required"
		statusStyle = terminal.Magenta
	} else if result.WarningCount() > 0 {
		status = "compliant with warnings"
		statusStyle = terminal.Yellow
	}
	if err := writeOutputf(out, "Summary: %s, %s, %s\n",
		colorSummaryCount(out, result.ErrorCount(), "errors", terminal.Red),
		colorSummaryCount(out, result.WarningCount(), "warnings", terminal.Yellow),
		colorSummaryCount(out, result.ManualCount(), "manual items", terminal.Magenta),
	); err != nil {
		return err
	}
	return writeOutputf(out, "Status: %s\n", terminal.Color(out, statusStyle, status))
}

func printComplianceDetails(out io.Writer, result rules.Result) error {
	if err := writeOutputln(out, "Details:"); err != nil {
		return err
	}
	if len(result.Diagnostics) == 0 {
		return writeOutputln(out, "  none")
	}
	for _, diagnostic := range result.Diagnostics {
		style := terminal.Yellow
		if diagnostic.Manual {
			style = terminal.Magenta
		} else if diagnostic.Severity == rules.SeverityError {
			style = terminal.Red
		}
		manual := ""
		if diagnostic.Manual {
			manual = " (manual)"
		}
		severity := terminal.Color(out, style, string(diagnostic.Severity))
		if err := writeOutputf(out, "  line %d: %s[%s]%s: %s\n", diagnostic.Line, severity, diagnostic.Code, manual, diagnostic.Message); err != nil {
			return err
		}
	}
	return nil
}

func writeOutput(out io.Writer, value string) error {
	written, err := io.WriteString(out, value)
	if err == nil && written != len(value) {
		return io.ErrShortWrite
	}
	return err
}

func writeOutputf(out io.Writer, format string, values ...interface{}) error {
	return writeOutput(out, fmt.Sprintf(format, values...))
}

func writeOutputln(out io.Writer, values ...interface{}) error {
	return writeOutput(out, fmt.Sprintln(values...))
}

func colorSummaryCount(out io.Writer, count int, label, style string) string {
	if count == 0 {
		style = terminal.Dim
	}
	return terminal.Color(out, style, fmt.Sprintf("%d %s", count, label))
}
