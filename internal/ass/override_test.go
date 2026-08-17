package ass

import "testing"

func TestScanOverrideSyntax(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "valid position", text: `{\pos(100,200)}`, want: false},
		{name: "valid numeric tag", text: `{\fs20}`, want: false},
		{name: "valid karaoke tag", text: `{\kf20}`, want: false},
		{name: "valid transform", text: `{\t(0,100,\fs20)}`, want: false},
		{name: "invalid transform tag", text: `{\t(0,100,\fs)}`, want: true},
		{name: "block comment", text: `{comment}`, want: false},
		{name: "missing parenthesis", text: `{\pos(100,200}`, want: true},
		{name: "missing closing brace", text: `{\pos(100,200)`, want: true},
		{name: "missing tag name", text: `{\}`, want: true},
		{name: "repeated slash", text: `{\\}`, want: true},
		{name: "invalid tag name", text: `{\(100,200)}`, want: true},
		{name: "unmatched close", text: `text}`, want: true},
		{name: "nested block", text: `{{\pos(100,200)}}`, want: true},
		{name: "invalid arguments", text: `{\move(0,0,100)}`, want: true},
		{name: "invalid numeric tag", text: `{\fsabc}`, want: true},
		{name: "invalid alignment", text: `{\an10}`, want: true},
		{name: "non-finite number", text: `{\pos(NaN,1)}`, want: true},
		{name: "invalid color", text: `{\c&HFF0000}`, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ScanOverrideSyntax(Event{Text: test.text})
			if (len(got) > 0) != test.want {
				t.Fatalf("ScanOverrideSyntax(%q) = %#v", test.text, got)
			}
		})
	}
}

func TestScanOverrideSyntaxVSFilterMod(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "valid fsc", text: `{\fsc200}`, want: false},
		{name: "valid mover", text: `{\mover(0,0,100,100,0,0,10,10)}`, want: false},
		{name: "valid gradient", text: `{\1vc(&H000000&,&HFFFFFF&,&H000000&,&HFFFFFF&)}`, want: false},
		{name: "invalid fsvp", text: `{\fsvpbad}`, want: true},
		{name: "invalid mover arguments", text: `{\mover(0,0,100)}`, want: true},
		{name: "invalid gradient arguments", text: `{\1vc(&H000000&,&HFFFFFF&)}`, want: true},
		{name: "invalid alpha gradient", text: `{\1va(&H0000&,&HFF&,&H00&,&HFF&)}`, want: true},
		{name: "valid blend", text: `{\blend(mult)}`, want: false},
		{name: "invalid blend", text: `{\blend(unknown)}`, want: true},
		{name: "valid lua extension", text: `{\lua(method,args)}`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ScanOverrideSyntax(Event{Text: test.text})
			if (len(got) > 0) != test.want {
				t.Fatalf("ScanOverrideSyntax(%q) = %#v", test.text, got)
			}
		})
	}
}

func TestScanOverridesClassifiesVSFilterMod(t *testing.T) {
	blocks := ScanOverrides(Event{Text: `{\t(0,100,\fsc200)\fsvp10\1vc(&H000000&,&HFFFFFF&,&H000000&,&HFFFFFF&)}`})
	if len(blocks) != 1 {
		t.Fatalf("expected one override block, got %#v", blocks)
	}
	seen := map[string]bool{}
	for _, tag := range blocks[0].Tags {
		if tag.VSFilterMod && tag.Known {
			t.Fatalf("VSFilterMod tag should not be marked as libass-known: %#v", tag)
		}
		seen[tag.Name] = tag.VSFilterMod
	}
	for _, name := range []string{"fsc", "fsvp", "1vc"} {
		if !seen[name] {
			t.Fatalf("expected VSFilterMod tag %q, got %#v", name, blocks[0].Tags)
		}
	}
}
