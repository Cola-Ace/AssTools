package ass

import (
	"bytes"
	"testing"
)

func TestParseBytesPreservesBOMAndMixedNewlines(t *testing.T) {
	data := append([]byte{0xef, 0xbb, 0xbf}, []byte("a\r\nb\nc")...)
	source, err := ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !source.BOM || !source.Mixed || source.DominantNewline != NewlineCRLF || source.TrailingNewline {
		t.Fatalf("unexpected metadata: %#v", source)
	}
	if len(source.Lines) != 3 || string(source.Lines[1].Content) != "b" || string(source.Lines[1].Terminator) != "\n" {
		t.Fatalf("unexpected lines: %#v", source.Lines)
	}
	rendered, err := source.Render([]Replacement{{Start: source.Lines[2].Start, End: source.Lines[2].End, Bytes: []byte("c\r\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rendered, append([]byte{0xef, 0xbb, 0xbf}, []byte("a\r\nb\nc\r\n")...)) {
		t.Fatalf("unexpected render: %q", rendered)
	}
}

func TestParseTimeStrictCentiseconds(t *testing.T) {
	value, err := ParseTime("1:02:03.45")
	if err != nil || value.String() != "1:02:03.45" {
		t.Fatalf("unexpected time: %v %v", value, err)
	}
	for _, invalid := range []string{"1:2:03.45", "1:02:60.00", "1:02:03.4"} {
		if _, err := ParseTime(invalid); err == nil {
			t.Errorf("ParseTime(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestParserKeepsCommasInEventText(t *testing.T) {
	data := []byte("[Script Info]\nScriptType: v4.00+\n[ V4+ Styles ]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: Default,Arial,20,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,one,two,three\n")
	source, err := ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	events := doc.Section(SectionEvents).Events
	if len(events) != 1 || events[0].Text != "one,two,three" {
		t.Fatalf("unexpected event: %#v", events)
	}
}
