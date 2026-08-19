package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONOutputForAllCommands(t *testing.T) {
	data := []byte("[Script Info]\n")
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "info", args: []string{"info", "--json"}},
		{name: "check", args: []string{"check", "--json"}},
		{name: "normalize", args: []string{"normalize", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string(nil), test.args...)
			if test.name == "normalize" {
				path := filepath.Join(t.TempDir(), "sample.ass")
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
				args = append(args, path)
			} else {
				args = append(args, "-")
			}
			var out, errOut bytes.Buffer
			code := Run(args, bytes.NewReader(data), &out, &errOut)
			if (code != ExitOK && code != ExitCheck) || errOut.Len() != 0 {
				t.Fatalf("%s --json failed: code=%d out=%q err=%q", test.name, code, out.String(), errOut.String())
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
				t.Fatalf("%s --json returned invalid JSON: %v; output=%q", test.name, err, out.String())
			}
			if payload["command"] != test.name {
				t.Fatalf("%s --json command field = %v", test.name, payload["command"])
			}
		})
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"help", "--json"}, strings.NewReader(""), &out, &errOut); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("help --json failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	var helpPayload map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &helpPayload); err != nil || helpPayload["command"] != "help" {
		t.Fatalf("help --json returned invalid payload: err=%v out=%q", err, out.String())
	}
}

func TestInfoJSONSeparatesMatrixCandidateReason(t *testing.T) {
	data := []byte("[Script Info]\nYCbCr Matrix: None\nLayoutResX: 1920\nLayoutResY: 1080\n")
	var out, errOut bytes.Buffer
	if code := Run([]string{"info", "--json", "-"}, bytes.NewReader(data), &out, &errOut); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("info --json failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	var payload struct {
		Structure struct {
			MatrixCandidate       string `json:"matrix_candidate"`
			MatrixCandidateReason string `json:"matrix_candidate_reason"`
		} `json:"structure"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("info --json returned invalid JSON: %v", err)
	}
	if payload.Structure.MatrixCandidate != "TV.709" || payload.Structure.MatrixCandidateReason != "inferred from LayoutRes 1920x1080" {
		t.Fatalf("unexpected matrix fields: %#v", payload.Structure)
	}
}

func TestInfoJSONHoistsCommonStyleFields(t *testing.T) {
	data := []byte("[Script Info]\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\nStyle: Alt,Arial,18\n")
	var out, errOut bytes.Buffer
	if code := Run([]string{"info", "--json", "-"}, bytes.NewReader(data), &out, &errOut); code != ExitOK || errOut.Len() != 0 {
		t.Fatalf("info --json failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	var payload struct {
		Styles struct {
			Fields      []string            `json:"fields"`
			Definitions []map[string]string `json:"definitions"`
		} `json:"styles"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("info --json returned invalid JSON: %v", err)
	}
	if len(payload.Styles.Fields) != 3 || len(payload.Styles.Definitions) != 2 {
		t.Fatalf("unexpected style fields: %#v", payload.Styles)
	}
	for _, definition := range payload.Styles.Definitions {
		if definition["name"] == "" || definition["font_name"] == "" || definition["font_size"] == "" || definition["values"] != "" {
			t.Fatalf("style definition should contain complete direct values: %#v", definition)
		}
	}
}

func TestRunHelpAndUsageErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(out.String(), "asst - cross-platform") || !strings.Contains(out.String(), "normalize [--backup]") || !strings.Contains(out.String(), "Exit codes: 0 = success, warnings, cancellation, or non-strict info findings; 1 = compliance errors, unresolved manual items, or strict info findings; 2 = usage, encoding, I/O, backup, or replacement failures") {
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
	want := "asst: info requires exactly one .ass file\n\nUsage: asst info [--strict] [-|--input|<input.ass>]\n\nPrint file metadata, sections, styles, fonts, events, and a compliance summary.\nUse - or --input to read ASS data from standard input.\nBy default info always exits 0 after a successful load; --strict exits 1 for compliance errors or unresolved manual items.\n"
	if got := errOut.String(); got != want {
		t.Fatalf("usage spacing mismatch: got=%q want=%q", got, want)
	}
}

func TestRunCommandHelpIsCaseInsensitive(t *testing.T) {
	for _, command := range []string{"info", "check", "normalize"} {
		for _, name := range []string{command, strings.ToUpper(command)} {
			for _, flag := range []string{"-h", "--help"} {
				var out, errOut bytes.Buffer
				if code := Run([]string{name, flag}, strings.NewReader(""), &out, &errOut); code != ExitOK {
					t.Fatalf("%s %s failed: code=%d out=%q err=%q", name, flag, code, out.String(), errOut.String())
				}
				if !strings.Contains(out.String(), "Usage: asst "+command) || errOut.Len() != 0 {
					t.Fatalf("%s %s printed unexpected help: out=%q err=%q", name, flag, out.String(), errOut.String())
				}
			}
		}
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"help", "INFO"}, strings.NewReader(""), &out, &errOut); code != ExitOK || !strings.Contains(out.String(), "Usage: asst info") || errOut.Len() != 0 {
		t.Fatalf("help INFO failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
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

func TestInfoRemovesCurrentDirectoryPrefix(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	if err := os.WriteFile("sample.ass", []byte("[Script Info]\n; generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"./sample.ass", `.\sample.ass`} {
		var out, errOut bytes.Buffer
		if code := Run([]string{"info", input}, strings.NewReader(""), &out, &errOut); code != 0 || errOut.Len() != 0 {
			t.Fatalf("info %q failed: code=%d out=%q err=%q", input, code, out.String(), errOut.String())
		}
		if !strings.Contains(out.String(), `Path: "sample.ass"`) || strings.Contains(out.String(), input) {
			t.Fatalf("info did not clean current-directory prefix %q: %q", input, out.String())
		}
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

func TestNormalizeOutputWritesSeparateFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sample.ass")
	outputPath := filepath.Join(directory, "normalized.ass")
	data := []byte("[Script Info]\n; generated\nScriptType: v4.00+\nWrapStyle: 2\nScaledBorderAndShadow: yes\nYCbCr Matrix: TV.709\nLayoutResX: 1920\nLayoutResY: 1080\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,hello\\n\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"normalize", "--yes", "--output", outputPath, path}, strings.NewReader(""), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("normalize --output failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	gotInput, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotInput, data) {
		t.Fatalf("input changed while writing output: %q", gotInput)
	}
	gotOutput, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(gotOutput, data) || !strings.Contains(out.String(), "Output: \""+outputPath+"\"") {
		t.Fatalf("output was not normalized separately: out=%q output=%q", out.String(), gotOutput)
	}
}

func TestInfoAndCheckReadStdin(t *testing.T) {
	data := []byte("[Script Info]\n; generated\n")
	var out, errOut bytes.Buffer
	if code := Run([]string{"info", "--input"}, bytes.NewReader(data), &out, &errOut); code != 0 || errOut.Len() != 0 || !strings.Contains(out.String(), "Path: \"-\"") {
		t.Fatalf("info --input failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"check", "-"}, bytes.NewReader(data), &out, &errOut); code != 1 || errOut.Len() != 0 || !strings.Contains(out.String(), "-:2:") {
		t.Fatalf("check - failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestInfoStrictExitCode(t *testing.T) {
	data := []byte("[Script Info]\n")
	var out, errOut bytes.Buffer
	if code := Run([]string{"info", "-"}, bytes.NewReader(data), &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("non-strict info returned code %d, out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"info", "--strict", "-"}, bytes.NewReader(data), &out, &errOut); code != 1 || errOut.Len() != 0 {
		t.Fatalf("strict info returned code %d, out=%q err=%q", code, out.String(), errOut.String())
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
