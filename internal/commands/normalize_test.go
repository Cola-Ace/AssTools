package commands

import (
	"bytes"
	"strings"
	"testing"

	"asstools/internal/rules"
)

func TestPrintEditKeepsSingleBackslashesReadable(t *testing.T) {
	var out bytes.Buffer
	printEdit(&out, rules.Edit{
		Line:        4,
		Code:        "lowercase-break",
		Description: "normalize line break",
		Before:      `hello\nworld`,
		After:       `hello\Nworld`,
	})

	got := out.String()
	if strings.Contains(got, `\\n`) || strings.Contains(got, `\\N`) {
		t.Fatalf("printEdit doubled a backslash: %q", got)
	}
	for _, want := range []string{`before: "hello\nworld"`, `after:  "hello\Nworld"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("printEdit output is missing %q: %q", want, got)
		}
	}
}

func TestFormatEditValueEscapesLineBreaksWithoutDoublingBackslashes(t *testing.T) {
	if got, want := formatEditValue("before\r\nafter"), `"before\r\nafter"`; got != want {
		t.Fatalf("formatEditValue() = %q, want %q", got, want)
	}
}

func TestFormatEditValueKeepsPathBackslashesReadable(t *testing.T) {
	if got, want := formatEditValue(`D:\work\episode.ass`), `"D:\work\episode.ass"`; got != want {
		t.Fatalf("formatEditValue() = %q, want %q", got, want)
	}
}
