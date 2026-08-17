package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asstools/internal/ass"
	"asstools/internal/rules"
)

func TestPrintStyleTableAlignsColumns(t *testing.T) {
	var out bytes.Buffer
	printStyleTable(&out, []styleRow{
		{
			name:   "Default",
			fields: []styleColumn{{key: "name", label: "Name"}, {key: "fontname", label: "Fontname"}, {key: "fontsize", label: "Fontsize"}},
			values: map[string]string{"name": "Default", "fontname": "Arial", "fontsize": "20"},
		},
		{
			name:   "Long Name",
			fields: []styleColumn{{key: "name", label: "Name"}, {key: "fontname", label: "Fontname"}, {key: "fontsize", label: "Fontsize"}},
			values: map[string]string{"name": "Long Name", "fontname": "Dream Han Sans SC W20", "fontsize": "60"},
		},
	})

	want := "  Name       Fontname               Fontsize\n" +
		"  Default    Arial                  20\n" +
		"  Long Name  Dream Han Sans SC W20  60\n"
	if got := out.String(); got != want {
		t.Fatalf("unexpected style table:\n%s", got)
	}
}

func TestPrintStyleTableIncludesAllFormatFields(t *testing.T) {
	var out bytes.Buffer
	printStyleTable(&out, []styleRow{
		{
			name: "Default",
			fields: []styleColumn{
				{key: "name", label: "Name"},
				{key: "fontname", label: "Fontname"},
				{key: "fontsize", label: "Fontsize"},
				{key: "primarycolour", label: "PrimaryColour"},
				{key: "alignment", label: "Alignment"},
			},
			values: map[string]string{
				"name": "Default", "fontname": "Arial", "fontsize": "20",
				"primarycolour": "&H00FFFFFF", "alignment": "2",
			},
		},
	})

	for _, value := range []string{"PrimaryColour", "Alignment", "&H00FFFFFF"} {
		if !strings.Contains(out.String(), value) {
			t.Fatalf("style table is missing %q:\n%s", value, out.String())
		}
	}
}

func TestStyleCellTruncatesLongValues(t *testing.T) {
	got := styleCell(strings.Repeat("x", styleCellMaxWidth+5))
	if len([]rune(got)) != styleCellMaxWidth || !strings.HasSuffix(got, "...") {
		t.Fatalf("unexpected truncated cell %q", got)
	}
}

func TestStyleSummaryUsesEachStyleFormat(t *testing.T) {
	data := []byte("[V4+ Styles]\n" +
		"Format: Name, Fontname, Fontsize\n" +
		"Style: Legacy,Arial,20\n" +
		"Format: Name, Fontsize, Fontname, Alignment\n" +
		"Style: Modern,22,Noto Sans,7\n")
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	styles, _, _ := styleRowsSummary(doc)
	if len(styles) != 2 {
		t.Fatalf("expected two styles, got %d", len(styles))
	}
	if got := styles[0].values["fontsize"]; got != "20" {
		t.Fatalf("legacy fontsize = %q", got)
	}
	if got := styles[1].values["fontname"]; got != "Noto Sans" {
		t.Fatalf("modern fontname = %q", got)
	}
	if got := styles[1].values["alignment"]; got != "7" {
		t.Fatalf("modern alignment = %q", got)
	}
}

func TestPrintComplianceDetails(t *testing.T) {
	var out bytes.Buffer
	printComplianceDetails(&out, rules.Result{Diagnostics: []rules.Diagnostic{
		{Line: 2, Severity: rules.SeverityError, Code: "bad-value", Message: "value is invalid"},
		{Line: 4, Severity: rules.SeverityWarning, Code: "style-case", Message: "style name casing differs"},
		{Line: 6, Severity: rules.SeverityWarning, Code: "unknown-override", Message: "override requires review", Manual: true},
	}})

	want := "Details:\n" +
		"  line 2: error[bad-value]: value is invalid\n" +
		"  line 4: warning[style-case]: style name casing differs\n" +
		"  line 6: warning[unknown-override] (manual): override requires review\n"
	if got := out.String(); got != want {
		t.Fatalf("unexpected compliance details:\n%s", got)
	}
}

func TestPrintComplianceDetailsWhenEmpty(t *testing.T) {
	var out bytes.Buffer
	printComplianceDetails(&out, rules.Result{})
	if got, want := out.String(), "Details:\n  none\n"; got != want {
		t.Fatalf("unexpected empty compliance details: %q", got)
	}
}

func TestInfoIncludesComplianceDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ass")
	if err := os.WriteFile(path, []byte("[Script Info]\n; generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := Info(path, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("info failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	for _, want := range []string{"== Compliance ==", "Details:", "script-info-comment", "semicolon comment is present in Script Info"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("info output is missing %q:\n%s", want, out.String())
		}
	}
}
