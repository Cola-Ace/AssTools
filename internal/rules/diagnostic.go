package rules

import "asstools/internal/ass"

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Edit struct {
	Line        int
	Start       int
	End         int
	Replacement []byte
	Code        string
	Description string
	Before      string
	After       string
	Safe        bool
}

type Diagnostic struct {
	Line      int
	Severity  Severity
	Code      string
	Message   string
	Edit      *Edit
	Manual    bool
	RuleOrder int
}

type Result struct {
	Diagnostics []Diagnostic
	Edits       []Edit
}

func (r Result) ErrorCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			count++
		}
	}
	return count
}

func (r Result) WarningCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityWarning && !diagnostic.Manual {
			count++
		}
	}
	return count
}

func (r Result) ManualCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Manual {
			count++
		}
	}
	return count
}

func replacementFromEdit(edit Edit) ass.Replacement {
	return ass.Replacement{Start: edit.Start, End: edit.End, Bytes: edit.Replacement}
}
