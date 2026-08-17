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
	if code := Run(nil, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(out.String(), "asst - cross-platform") {
		t.Fatalf("help failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"normalize", "--matrix", "bad", "x.ass"}, strings.NewReader(""), &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "invalid matrix") {
		t.Fatalf("usage failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
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
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("input changed: %v %q", err, got)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("backup exists after cancel: %v", err)
	}
}
