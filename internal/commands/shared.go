package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"asstools/internal/ass"
	"asstools/internal/rules"
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
	if result.ErrorCount() > 0 {
		status = "errors found"
	} else if result.ManualCount() > 0 {
		status = "manual review required"
	} else if result.WarningCount() > 0 {
		status = "compliant with warnings"
	}
	fmt.Fprintf(out, "Summary: %d errors, %d warnings, %d manual items\n", result.ErrorCount(), result.WarningCount(), result.ManualCount())
	fmt.Fprintf(out, "Status: %s\n", status)
}
