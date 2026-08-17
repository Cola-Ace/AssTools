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

func TestPrintEditsSeparatesChanges(t *testing.T) {
	var out bytes.Buffer
	printEdits(&out, []rules.Edit{
		{Line: 1, Code: "first", Description: "first change", Before: "old", After: "new"},
		{Line: 2, Code: "second", Description: "second change", Before: "left", After: "right"},
	})

	got := out.String()
	if !strings.Contains(got, "after:  \"new\"\n\n  line 2") {
		t.Fatalf("printEdits did not separate changes with a blank line: %q", got)
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Fatalf("printEdits added a trailing blank line: %q", got)
	}
}

func TestEditValueDiffKeepsSharedTextOutsideChange(t *testing.T) {
	before, after := editValueDiff(`"hello old world"`, `"hello new world"`)

	if before.prefix != `"hello ` || before.changed != "old" || before.suffix != ` world"` {
		t.Fatalf("before diff = %#v", before)
	}
	if after.prefix != `"hello ` || after.changed != "new" || after.suffix != ` world"` {
		t.Fatalf("after diff = %#v", after)
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
