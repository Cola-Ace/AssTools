package rules

import (
	"strings"
	"testing"

	"asstools/internal/ass"
)

func TestMatrixInferencePrefersLayout(t *testing.T) {
	data := []byte("[Script Info]\nLayoutResX: 1920\nLayoutResY: 1080\nPlayResX: 1280\nPlayResY: 720\n")
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	candidate, conflict := InferMatrix(doc)
	if candidate == nil || candidate.Value != "TV.709" || !strings.Contains(conflict, "PlayRes") {
		t.Fatalf("unexpected inference: %#v %q", candidate, conflict)
	}
}

func TestRunFindsExampleSafeEdits(t *testing.T) {
	data := []byte("[Script Info]\r\n; generated\r\nScriptType: v4.00+\r\nWrapStyle: 2\r\nScaledBorderAndShadow: yes\r\nYCbCr Matrix: TV.709\r\nLayoutResX: 1920\r\nLayoutResY: 1080\r\n[ Aegisub Project Garbage ]\r\nAudio File: x\r\n\r\n[V4+ Styles]\r\nFormat: Name, Fontname, Fontsize\r\nStyle: Default,Arial,20\r\n[Events]\r\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\r\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,one\\Ntwo\\nthree\r\n")
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	result := Run(doc, "auto")
	if result.ErrorCount() != 0 || len(result.Edits) < 3 {
		t.Fatalf("unexpected result: %#v", result)
	}
	seen := map[string]bool{}
	for _, diagnostic := range result.Diagnostics {
		seen[diagnostic.Code] = true
	}
	for _, code := range []string{"script-info-comment", "obsolete-section", "lowercase-break"} {
		if !seen[code] {
			t.Errorf("missing diagnostic %s", code)
		}
	}
}

func TestRunChecksDialogueOverrideSyntax(t *testing.T) {
	data := []byte(`[Script Info]
ScriptType: v4.00+
WrapStyle: 2
ScaledBorderAndShadow: yes
[V4+ Styles]
Format: Name, Fontname, Fontsize
Style: Default,Arial,20
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\pos(100,200}broken
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\move(0,0,100,100)}valid
Comment: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\pos(100,200}comment
`)
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	result := Run(doc)
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "override-syntax" {
			continue
		}
		count++
		if diagnostic.Line != 10 || diagnostic.Severity != SeverityError || !strings.Contains(diagnostic.Message, "unbalanced parentheses") {
			t.Fatalf("unexpected override syntax diagnostic: %#v", diagnostic)
		}
	}
	if count != 1 {
		t.Fatalf("expected one Dialogue override syntax diagnostic, got %d: %#v", count, result.Diagnostics)
	}
}

func TestRunRepairsDeterministicOverrideSyntaxErrors(t *testing.T) {
	data := []byte(`[Script Info]
ScriptType: v4.00+
WrapStyle: 2
ScaledBorderAndShadow: yes
YCbCr Matrix: TV.709
[V4+ Styles]
Format: Name, Fontname, Fontsize
Style: Default,Arial,20
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fax(0.2)}fax
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fscy150)}scale
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\move(0,0,100,100,(-269,1190))}move
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fax(0.2)}hello\nworld
`)
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	result := Run(doc)
	if result.ErrorCount() != 4 {
		t.Fatalf("expected four syntax errors, got %d: %#v", result.ErrorCount(), result.Diagnostics)
	}
	fixes := 0
	for _, edit := range result.Edits {
		if edit.Code == "override-syntax" {
			fixes++
		}
	}
	if fixes != 4 {
		t.Fatalf("expected four safe syntax fixes, got %d: %#v", fixes, result.Edits)
	}
	candidate, err := source.Render(ToReplacements(result.Edits))
	if err != nil {
		t.Fatal(err)
	}
	candidateSource, err := ass.ParseBytes(candidate)
	if err != nil {
		t.Fatal(err)
	}
	candidateDoc, err := ass.Parse(candidateSource)
	if err != nil {
		t.Fatal(err)
	}
	if after := Run(candidateDoc); after.ErrorCount() != 0 || after.WarningCount() != 0 {
		t.Fatalf("safe syntax fixes left errors: %#v", after.Diagnostics)
	}
}

func TestRunMarksVSFilterModTagsSeparatelyFromSyntax(t *testing.T) {
	data := []byte(`[Script Info]
ScriptType: v4.00+
WrapStyle: 2
ScaledBorderAndShadow: yes
[V4+ Styles]
Format: Name, Fontname, Fontsize
Style: Default,Arial,20
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fsvp10\mover(0,0,100,100,0,0,10,10)}valid
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fsvpbad}invalid
`)
	source, err := ass.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ass.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	result := Run(doc)
	vsWarnings := 0
	syntaxErrors := 0
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Code {
		case "vsfiltermod-override":
			vsWarnings++
			if diagnostic.Severity != SeverityWarning {
				t.Fatalf("unexpected VSFilterMod diagnostic: %#v", diagnostic)
			}
		case "override-syntax":
			syntaxErrors++
		}
	}
	if vsWarnings != 3 {
		t.Fatalf("expected three VSFilterMod warnings, got %d: %#v", vsWarnings, result.Diagnostics)
	}
	if syntaxErrors != 1 {
		t.Fatalf("expected one VSFilterMod syntax error, got %d: %#v", syntaxErrors, result.Diagnostics)
	}
}
