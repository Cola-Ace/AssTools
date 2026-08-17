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
