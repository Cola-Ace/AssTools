package commands

import (
	"encoding/json"
	"io"

	"asstools/internal/rules"
)

type jsonSummary struct {
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	ManualItems int `json:"manual_items"`
}

type jsonDiagnostic struct {
	Line     int       `json:"line"`
	Severity string    `json:"severity"`
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Manual   bool      `json:"manual"`
	Edit     *jsonEdit `json:"edit"`
}

type jsonEdit struct {
	Line        int    `json:"line"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Replacement string `json:"replacement"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Safe        bool   `json:"safe"`
}

type jsonError struct {
	Message string `json:"message"`
}

type jsonErrorEnvelope struct {
	Command string     `json:"command"`
	Status  string     `json:"status"`
	Error   *jsonError `json:"error"`
}

type checkJSON struct {
	Command     string           `json:"command"`
	Status      string           `json:"status"`
	Path        string           `json:"path"`
	Summary     jsonSummary      `json:"summary"`
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
}

type normalizeJSON struct {
	Command            string           `json:"command"`
	Status             string           `json:"status"`
	Path               string           `json:"path"`
	Output             string           `json:"output"`
	MatrixMode         string           `json:"matrix_mode"`
	MatrixDecision     string           `json:"matrix_decision"`
	Preview            bool             `json:"preview"`
	Changes            []jsonEdit       `json:"changes"`
	ManualItems        []jsonDiagnostic `json:"manual_items"`
	Applied            bool             `json:"applied"`
	Backup             string           `json:"backup"`
	BackupWritten      bool             `json:"backup_written"`
	OutputWritten      bool             `json:"output_written"`
	Summary            jsonSummary      `json:"summary"`
	Diagnostics        []jsonDiagnostic `json:"diagnostics"`
	Recheck            *jsonSummary     `json:"recheck"`
	RecheckDiagnostics []jsonDiagnostic `json:"recheck_diagnostics"`
}

func encodeJSON(out io.Writer, value interface{}) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeJSONError(out io.Writer, command string, err error) error {
	return encodeJSON(out, jsonErrorEnvelope{
		Command: command,
		Status:  "error",
		Error:   &jsonError{Message: err.Error()},
	})
}

func summaryJSON(result rules.Result) jsonSummary {
	return jsonSummary{
		Errors:      result.ErrorCount(),
		Warnings:    result.WarningCount(),
		ManualItems: result.ManualCount(),
	}
}

func diagnosticsJSON(diagnostics []rules.Diagnostic) []jsonDiagnostic {
	items := make([]jsonDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		item := jsonDiagnostic{
			Line:     diagnostic.Line,
			Severity: string(diagnostic.Severity),
			Code:     diagnostic.Code,
			Message:  diagnostic.Message,
			Manual:   diagnostic.Manual,
		}
		if diagnostic.Edit != nil {
			item.Edit = editJSON(*diagnostic.Edit)
		}
		items = append(items, item)
	}
	return items
}

func editsJSON(edits []rules.Edit) []jsonEdit {
	items := make([]jsonEdit, 0, len(edits))
	for _, edit := range edits {
		items = append(items, *editJSON(edit))
	}
	return items
}

func editJSON(edit rules.Edit) *jsonEdit {
	return &jsonEdit{
		Line:        edit.Line,
		Start:       edit.Start,
		End:         edit.End,
		Replacement: string(edit.Replacement),
		Code:        edit.Code,
		Description: edit.Description,
		Before:      edit.Before,
		After:       edit.After,
		Safe:        edit.Safe,
	}
}

func resultStatus(result rules.Result) string {
	if result.ErrorCount() > 0 {
		return "errors"
	}
	if result.ManualCount() > 0 {
		return "manual_review"
	}
	if result.WarningCount() > 0 {
		return "warnings"
	}
	return "ok"
}
