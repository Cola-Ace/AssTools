package ass

import (
	"fmt"
	"strconv"
	"strings"
)

func Parse(source *Source) (*Document, error) {
	if source == nil {
		return nil, fmt.Errorf("nil source")
	}
	doc := &Document{Source: source}
	sectionIndexes := make([]int, 0)
	for i, line := range source.Lines {
		name, ok := sectionHeader(string(line.Content))
		if ok {
			sectionIndexes = append(sectionIndexes, i)
			section := Section{
				RawName:    name,
				Kind:       ClassifySection(name),
				HeaderLine: line.Number,
				StartLine:  line.Number,
				EndLine:    line.Number,
			}
			doc.Sections = append(doc.Sections, section)
		}
	}
	for i := range doc.Sections {
		start := sectionIndexes[i]
		end := len(source.Lines)
		if i+1 < len(sectionIndexes) {
			end = sectionIndexes[i+1]
		}
		lastContent := end - 1
		for lastContent > start && strings.TrimSpace(string(source.Lines[lastContent].Content)) == "" {
			lastContent--
		}
		doc.Sections[i].EndLine = source.Lines[lastContent].Number
		parseSection(doc, &doc.Sections[i], start+1, end)
	}
	return doc, nil
}

func sectionHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 2 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), true
}

func parseSection(doc *Document, section *Section, start, end int) {
	for i := start; i < end; i++ {
		line := doc.Source.Lines[i]
		text := string(line.Content)
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, ";") {
			continue
		}
		if hasPrefixFold(trimmed, "Format:") {
			payload, _ := afterColon(text, "Format:")
			fields := splitSimpleFields(payload)
			section.Formats = append(section.Formats, Format{Line: line.Number, Fields: fields, Kind: section.Kind})
			continue
		}
		switch section.Kind {
		case SectionScriptInfo:
			if property, ok := parseProperty(line); ok {
				section.Properties = append(section.Properties, property)
			}
		case SectionStyles:
			if prefix, payload, prefixStart, ok := recordPayload(text, "Style"); ok {
				format := latestFormat(section)
				style := parseStyle(line, prefix, payload, prefixStart, format)
				section.Styles = append(section.Styles, style)
				if format != nil && len(style.Values) != len(format.Fields) {
					doc.Issues = append(doc.Issues, ParseIssue{Line: line.Number, Code: "style-field-count", Message: fmt.Sprintf("Style has %d fields but Format has %d", len(style.Values), len(format.Fields))})
				}
			}
		case SectionEvents:
			if prefix, payload, prefixStart, ok := recordPayload(text, "Dialogue"); ok {
				format := latestFormat(section)
				event := parseEvent(line, prefix, payload, prefixStart, format, "Dialogue")
				section.Events = append(section.Events, event)
				if format != nil && len(event.Values) != len(format.Fields) {
					doc.Issues = append(doc.Issues, ParseIssue{Line: line.Number, Code: "event-field-count", Message: fmt.Sprintf("Dialogue has %d fields but Format has %d", len(event.Values), len(format.Fields))})
				}
			} else if prefix, payload, prefixStart, ok := recordPayload(text, "Comment"); ok {
				format := latestFormat(section)
				event := parseEvent(line, prefix, payload, prefixStart, format, "Comment")
				section.Events = append(section.Events, event)
				if format != nil && len(event.Values) != len(format.Fields) {
					doc.Issues = append(doc.Issues, ParseIssue{Line: line.Number, Code: "event-field-count", Message: fmt.Sprintf("Comment has %d fields but Format has %d", len(event.Values), len(format.Fields))})
				}
			}
		}
	}
}

func parseProperty(line Line) (Property, bool) {
	text := string(line.Content)
	colon := strings.IndexByte(text, ':')
	if colon < 0 {
		return Property{}, false
	}
	keyStart := 0
	for keyStart < colon && (text[keyStart] == ' ' || text[keyStart] == '\t') {
		keyStart++
	}
	keyEnd := colon
	for keyEnd > keyStart && (text[keyEnd-1] == ' ' || text[keyEnd-1] == '\t') {
		keyEnd--
	}
	valueStart := colon + 1
	for valueStart < len(text) && (text[valueStart] == ' ' || text[valueStart] == '\t') {
		valueStart++
	}
	valueEnd := len(text)
	for valueEnd > valueStart && (text[valueEnd-1] == ' ' || text[valueEnd-1] == '\t') {
		valueEnd--
	}
	return Property{
		Line:      line.Number,
		RawKey:    text[keyStart:keyEnd],
		RawValue:  text[valueStart:valueEnd],
		Key:       strings.TrimSpace(text[keyStart:keyEnd]),
		Value:     strings.TrimSpace(text[valueStart:valueEnd]),
		KeySpan:   Span{Start: line.Start + keyStart, End: line.Start + keyEnd},
		ValueSpan: Span{Start: line.Start + valueStart, End: line.Start + valueEnd},
	}, true
}

func parseStyle(line Line, prefix, payload string, prefixStart int, format *Format) Style {
	values, spans := splitFields(payload, line.Start+prefixStart+len(prefix)+1, false)
	fields := map[string]int{}
	if format != nil {
		for i, field := range format.Fields {
			fields[strings.ToLower(strings.TrimSpace(field))] = i
		}
	}
	name := ""
	nameSpan := Span{}
	if index, ok := fields["name"]; ok && index < len(values) {
		name = values[index]
		nameSpan = spans[index]
	} else if len(values) > 0 {
		name = values[0]
		nameSpan = spans[0]
	}
	return Style{Line: line.Number, Prefix: prefix, Values: values, Fields: fields, Spans: spans, Name: name, NameSpan: nameSpan, Kind: SectionStyles}
}

func parseEvent(line Line, prefix, payload string, prefixStart int, format *Format, kind string) Event {
	values, spans := splitFields(payload, line.Start+prefixStart+len(prefix)+1, true)
	fields := map[string]int{}
	if format != nil {
		for i, field := range format.Fields {
			fields[strings.ToLower(strings.TrimSpace(field))] = i
		}
	}
	event := Event{Line: line.Number, Prefix: prefix, Kind: kind, Values: values, Fields: fields, Spans: spans, KindSpan: Span{Start: line.Start + prefixStart, End: line.Start + prefixStart + len(prefix)}}
	if index, ok := fields["layer"]; ok && index < len(values) {
		event.Layer, _ = strconv.Atoi(strings.TrimSpace(values[index]))
	}
	if index, ok := fields["start"]; ok && index < len(values) {
		event.StartRaw = values[index]
		event.Start, _ = ParseTime(values[index])
	}
	if index, ok := fields["end"]; ok && index < len(values) {
		event.EndRaw = values[index]
		event.End, _ = ParseTime(values[index])
	}
	if index, ok := fields["style"]; ok && index < len(values) {
		event.Style = values[index]
	}
	if index, ok := fields["name"]; ok && index < len(values) {
		event.Name = values[index]
	}
	if index, ok := fields["text"]; ok && index < len(values) {
		event.Text = values[index]
		event.TextSpan = spans[index]
	}
	return event
}

func ParseTime(raw string) (Time, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 8 {
		return 0, fmt.Errorf("invalid time %q", raw)
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 3 || len(parts[1]) != 2 || len(parts[2]) != 5 || parts[2][2] != '.' || len(parts[0]) == 0 {
		return 0, fmt.Errorf("invalid time %q", raw)
	}
	hours, err1 := strconv.Atoi(parts[0])
	minutes, err2 := strconv.Atoi(parts[1])
	seconds, err3 := strconv.Atoi(parts[2][:2])
	centiseconds, err4 := strconv.Atoi(parts[2][3:])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || minutes > 59 || seconds > 59 || centiseconds > 99 || hours < 0 || minutes < 0 || seconds < 0 || centiseconds < 0 {
		return 0, fmt.Errorf("invalid time %q", raw)
	}
	return Time(int64(hours)*360000 + int64(minutes)*6000 + int64(seconds)*100 + int64(centiseconds)), nil
}

func latestFormat(section *Section) *Format {
	if len(section.Formats) == 0 {
		return nil
	}
	return &section.Formats[len(section.Formats)-1]
}

func afterColon(text, prefix string) (string, int) {
	index := strings.Index(strings.ToLower(text), strings.ToLower(prefix))
	if index < 0 {
		return "", len(text)
	}
	start := index + strings.Index(prefix, ":") + 1
	return text[start:], start
}

func recordPayload(text, wanted string) (prefix, payload string, prefixStart int, ok bool) {
	colon := strings.IndexByte(text, ':')
	if colon < 0 {
		return "", "", 0, false
	}
	prefix = strings.TrimSpace(text[:colon])
	if !strings.EqualFold(prefix, wanted) {
		return "", "", 0, false
	}
	start := colon + 1
	return prefix, text[start:], strings.Index(text, prefix), true
}

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}

func splitSimpleFields(payload string) []string {
	parts := strings.Split(payload, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func splitFields(payload string, absoluteStart int, splitN10 bool) ([]string, []Span) {
	values := make([]string, 0, 10)
	spans := make([]Span, 0, 10)
	start := 0
	fieldLimit := -1
	if splitN10 {
		fieldLimit = 9
	}
	for i := 0; i <= len(payload); i++ {
		if i != len(payload) && payload[i] != ',' {
			continue
		}
		if fieldLimit >= 0 && len(values) == fieldLimit {
			i = len(payload)
		}
		rawStart, rawEnd := start, i
		valueStart, valueEnd := rawStart, rawEnd
		for valueStart < valueEnd && (payload[valueStart] == ' ' || payload[valueStart] == '\t') {
			valueStart++
		}
		for valueEnd > valueStart && (payload[valueEnd-1] == ' ' || payload[valueEnd-1] == '\t') {
			valueEnd--
		}
		values = append(values, payload[valueStart:valueEnd])
		spans = append(spans, Span{Start: absoluteStart + valueStart, End: absoluteStart + valueEnd})
		if i == len(payload) {
			break
		}
		start = i + 1
	}
	return values, spans
}
