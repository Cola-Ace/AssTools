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

func printSummary(out io.Writer, result rules.Result) {
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
	fmt.Fprintf(out, "Summary: %s, %s, %s\n",
		colorSummaryCount(out, result.ErrorCount(), "errors", terminal.Red),
		colorSummaryCount(out, result.WarningCount(), "warnings", terminal.Yellow),
		colorSummaryCount(out, result.ManualCount(), "manual items", terminal.Magenta),
	)
	fmt.Fprintf(out, "Status: %s\n", terminal.Color(out, statusStyle, status))
}

func colorSummaryCount(out io.Writer, count int, label, style string) string {
	if count == 0 {
		style = terminal.Dim
	}
	return terminal.Color(out, style, fmt.Sprintf("%d %s", count, label))
}
