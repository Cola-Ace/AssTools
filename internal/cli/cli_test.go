package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpAndUsageErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(out.String(), "asst - cross-platform") || !strings.Contains(out.String(), "normalize [--backup]") || !strings.Contains(out.String(), "Exit codes: 0 = success, warnings, or cancellation; 1 = compliance errors or unresolved manual items; 2 = usage, encoding, I/O, backup, or replacement failures") {
		t.Fatalf("help failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"normalize", "--matrix", "bad", "x.ass"}, strings.NewReader(""), &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "invalid matrix") {
		t.Fatalf("usage failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"info"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("missing input should be a usage error: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	want := "asst: info requires exactly one .ass file\n\nUsage: asst info <input.ass>\n\nPrint file metadata, sections, styles, fonts, events, and a compliance summary.\n"
	if got := errOut.String(); got != want {
		t.Fatalf("usage spacing mismatch: got=%q want=%q", got, want)
	}
}

func TestNormalizeCancelDoesNotWrite(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	data := []byte("[Script Info]\n; generated\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\nLayoutResX: 1920\nLayoutResY: 1080\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hello\\n\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", path}, strings.NewReader("n\n"), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	for _, want := range []string{"Input: \"" + path + "\"", "Confirm [y/N]"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("normalize output is missing raw path %q: %q", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Backup:") {
		t.Fatalf("normalize should not prompt for a backup: %q", out.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("input changed: %v %q", err, got)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup exists after cancel: %v", err)
	}
}

func TestNormalizeConfirmAppliesWithoutBackup(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	data := []byte("[Script Info]\n; generated\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\nLayoutResX: 1920\nLayoutResY: 1080\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hello\\n\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", path}, strings.NewReader("y\n"), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "Applied ") || strings.Contains(out.String(), "Backup written") {
		t.Fatalf("normalize output has unexpected apply/backup status: %q", out.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, data) {
		t.Fatalf("input was not normalized: %q", got)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup exists after confirmation: %v", err)
	}
}

func TestNormalizeYesAppliesWithoutConfirmation(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	data := []byte("[Script Info]\n; generated\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\nLayoutResX: 1920\nLayoutResY: 1080\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hello\\n\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", "--yes", path}, strings.NewReader(""), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize --yes failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "Confirm [y/N]") || !strings.Contains(out.String(), "Applied ") {
		t.Fatalf("normalize --yes should apply without prompting: %q", out.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, data) {
		t.Fatalf("input was not normalized: %q", got)
	}
}

func TestNormalizeBackupOptIn(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	data := []byte("[Script Info]\n; generated\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\nLayoutResX: 1920\nLayoutResY: 1080\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hello\\n\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", "--backup", path}, strings.NewReader("y\n"), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize with backup failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "Backup written: \"") {
		t.Fatalf("normalize did not report backup: %q", out.String())
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, data) {
		t.Fatalf("backup does not match original input: %q", backup)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, data) {
		t.Fatalf("input was not normalized: %q", got)
	}
}

func TestCheckCanIgnoreVSFilterModWarningsButNotSyntaxErrors(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	data := []byte(`[Script Info]
ScriptType: v4.00+
WrapStyle: 2
ScaledBorderAndShadow: yes
YCbCr Matrix: TV.709
LayoutResX: 1920
LayoutResY: 1080
[V4+ Styles]
Format: Name, Fontname, Fontsize
Style: Default,Arial,20
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fsvp10}valid
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"check", path}, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(out.String(), "vsfiltermod-override") {
		t.Fatalf("check should report VSFilterMod warning: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"check", "--ignore-vsfiltermod", path}, strings.NewReader(""), &out, &errOut); code != 0 || strings.Contains(out.String(), "vsfiltermod-override") {
		t.Fatalf("check should ignore VSFilterMod warning: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}

	invalid := []byte(strings.Replace(string(data), `{\fsvp10}valid`, `{\fsvpbad}invalid`, 1))
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"check", "--ignore-vsfiltermod", path}, strings.NewReader(""), &out, &errOut); code != 1 || !strings.Contains(out.String(), "override-syntax") {
		t.Fatalf("check should retain VSFilterMod syntax error: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestNormalizeRepairsSafeOverrideSyntaxErrors(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
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
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", "--yes", path}, strings.NewReader(""), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "Applied 4 changes") || !strings.Contains(out.String(), "Recheck: 0 errors") {
		t.Fatalf("normalize did not repair syntax errors: %q", out.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`\fax0.2`, `\fscy150`, `\move(0,0,100,100,-269,1190)`} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("syntax repair is missing (%q): %q", want, got)
		}
	}
	for _, invalid := range []string{`\fax(0.2)`, `\fscy150)`, `\move(0,0,100,100,(-269,1190))`} {
		if strings.Contains(string(got), invalid) {
			t.Fatalf("syntax error was not repaired (%q): %q", invalid, got)
		}
	}
}
