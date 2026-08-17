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
