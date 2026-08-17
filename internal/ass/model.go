package ass

import (
	"fmt"
	"strings"
)

type SectionKind string

const (
	SectionScriptInfo     SectionKind = "script-info"
	SectionStyles         SectionKind = "styles"
	SectionEvents         SectionKind = "events"
	SectionFonts          SectionKind = "fonts"
	SectionProjectGarbage SectionKind = "project-garbage"
	SectionExtradata      SectionKind = "extradata"
	SectionUnknown        SectionKind = "unknown"
)

type Span struct {
	Start int
	End   int
}

type Property struct {
	Line      int
	RawKey    string
	RawValue  string
	Key       string
	Value     string
	KeySpan   Span
	ValueSpan Span
}

type Format struct {
	Line   int
	Fields []string
	Kind   SectionKind
}

type Style struct {
	Line     int
	Prefix   string
	Values   []string
	Fields   map[string]int
	Spans    []Span
	Name     string
	NameSpan Span
	Kind     SectionKind
}

type Event struct {
	Line     int
	Prefix   string
	Kind     string
	Values   []string
	Fields   map[string]int
	Spans    []Span
	Layer    int
	Start    Time
	End      Time
	StartRaw string
	EndRaw   string
	Style    string
	Name     string
	Text     string
	TextSpan Span
	KindSpan Span
}

type Time int64

type ParseIssue struct {
	Line    int
	Code    string
	Message string
}

type Section struct {
	RawName    string
	Kind       SectionKind
	HeaderLine int
	StartLine  int
	EndLine    int
	Properties []Property
	Formats    []Format
	Styles     []Style
	Events     []Event
}

type Document struct {
	Source   *Source
	Sections []Section
	Issues   []ParseIssue
}

func ClassifySection(name string) SectionKind {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "script info":
		return SectionScriptInfo
	case "v4+ styles", "v4 styles":
		return SectionStyles
	case "events":
		return SectionEvents
	case "fonts":
		return SectionFonts
	case "aegisub project garbage":
		return SectionProjectGarbage
	case "aegisub extradata":
		return SectionExtradata
	default:
		return SectionUnknown
	}
}

func (d *Document) Section(kind SectionKind) *Section {
	for i := range d.Sections {
		if d.Sections[i].Kind == kind {
			return &d.Sections[i]
		}
	}
	return nil
}

func (d *Document) ScriptProperties() map[string]Property {
	result := map[string]Property{}
	section := d.Section(SectionScriptInfo)
	if section == nil {
		return result
	}
	for _, property := range section.Properties {
		result[strings.ToLower(property.Key)] = property
	}
	return result
}

func (t Time) String() string {
	hours := int64(t) / 360000
	minutes := (int64(t) / 6000) % 60
	seconds := (int64(t) / 100) % 60
	centiseconds := int64(t) % 100
	return formatTime(hours, minutes, seconds, centiseconds)
}

func formatTime(hours, minutes, seconds, centiseconds int64) string {
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
}
