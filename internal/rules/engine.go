package rules

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"asstools/internal/ass"
)

type MatrixCandidate struct {
	Value  string
	Source string
	Detail string
}

type MatrixValue string

const (
	MatrixAuto   MatrixValue = "auto"
	MatrixNone   MatrixValue = "None"
	MatrixTV601  MatrixValue = "TV.601"
	MatrixTV709  MatrixValue = "TV.709"
	MatrixTV240M MatrixValue = "TV.240M"
	MatrixTVFCC  MatrixValue = "TV.FCC"
	MatrixPC601  MatrixValue = "PC.601"
	MatrixPC709  MatrixValue = "PC.709"
	MatrixPC240M MatrixValue = "PC.240M"
	MatrixPCFCC  MatrixValue = "PC.FCC"
)

type Context struct {
	Document       *ass.Document
	MatrixOverride MatrixOverride
}

type MatrixOverride struct {
	Explicit bool
	Value    string
}

var matrixValues = map[string]string{
	"auto":    "auto",
	"none":    "None",
	"tv.601":  "TV.601",
	"tv.709":  "TV.709",
	"tv.240m": "TV.240M",
	"tv.fcc":  "TV.FCC",
	"pc.601":  "PC.601",
	"pc.709":  "PC.709",
	"pc.240m": "PC.240M",
	"pc.fcc":  "PC.FCC",
}

func NormalizeMatrixValue(value string) (string, bool) {
	canonical, ok := matrixValues[strings.ToLower(strings.TrimSpace(value))]
	return canonical, ok
}

func InferMatrix(doc *ass.Document) (*MatrixCandidate, string) {
	if doc == nil {
		return nil, ""
	}
	properties := map[string][]ass.Property{}
	for key, property := range doc.ScriptProperties() {
		properties[key] = []ass.Property{property}
	}
	return inferMatrix(properties)
}

func Run(input interface{}, matrixModes ...string) Result {
	var doc *ass.Document
	matrixMode := "auto"
	switch value := input.(type) {
	case *ass.Document:
		doc = value
	case *Context:
		if value != nil {
			doc = value.Document
			if value.MatrixOverride.Value != "" {
				matrixMode = value.MatrixOverride.Value
			}
		}
	case Context:
		doc = value.Document
		if value.MatrixOverride.Value != "" {
			matrixMode = value.MatrixOverride.Value
		}
	case nil:
		return Result{}
	default:
		return Result{}
	}
	if len(matrixModes) > 0 && matrixModes[0] != "" {
		matrixMode = matrixModes[0]
	}
	result := Result{}
	if doc == nil || doc.Source == nil {
		return result
	}
	if matrixMode == "" {
		matrixMode = "auto"
	}
	order := 0
	add := func(d Diagnostic) {
		d.RuleOrder = order
		order++
		result.Diagnostics = append(result.Diagnostics, d)
		if d.Edit != nil && d.Edit.Safe {
			result.Edits = append(result.Edits, *d.Edit)
		}
	}

	runStructure(doc, add)
	runScriptInfo(doc, matrixMode, add)
	runStyles(doc, add)
	runEvents(doc, add)
	runOverrides(doc, add)

	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		if result.Diagnostics[i].Line != result.Diagnostics[j].Line {
			return result.Diagnostics[i].Line < result.Diagnostics[j].Line
		}
		return result.Diagnostics[i].RuleOrder < result.Diagnostics[j].RuleOrder
	})
	result.Edits = suppressOverlappingEdits(result.Edits)
	return result
}

func runStructure(doc *ass.Document, add func(Diagnostic)) {
	if !doc.Source.BOM {
		add(Diagnostic{Line: 1, Severity: SeverityWarning, Code: "utf8-bom", Message: "UTF-8 BOM is missing", Edit: &Edit{Line: 1, Start: 0, End: 0, Replacement: []byte{0xef, 0xbb, 0xbf}, Code: "utf8-bom", Description: "add UTF-8 BOM", Before: "<missing>", After: "UTF-8 BOM", Safe: true}})
	}
	if doc.Source.Mixed {
		add(Diagnostic{Line: 1, Severity: SeverityWarning, Code: "newline-mixed", Message: "CRLF and LF line endings are mixed; dominant style will be used for normalization"})
	}
	seen := map[ass.SectionKind]int{}
	lastRank := 0
	for index := range doc.Sections {
		section := &doc.Sections[index]
		if section.Kind == ass.SectionProjectGarbage || section.Kind == ass.SectionFonts || section.Kind == ass.SectionExtradata {
			message := fmt.Sprintf("[%s] can be removed safely", section.RawName)
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "obsolete-section", Message: message, Edit: sectionDeleteEdit(doc, section, "obsolete-section", message)})
		}
		if section.Kind == ass.SectionUnknown {
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "unknown-section", Message: fmt.Sprintf("unknown section [%s] is preserved", section.RawName), Manual: true})
		}
		if section.Kind == ass.SectionScriptInfo || section.Kind == ass.SectionStyles || section.Kind == ass.SectionEvents {
			seen[section.Kind]++
			if seen[section.Kind] > 1 {
				add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "duplicate-section", Message: fmt.Sprintf("duplicate [%s] section", section.RawName)})
			}
			rank := sectionRank(section.Kind)
			if rank > 0 && lastRank > 0 && rank < lastRank {
				add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "section-order", Message: "standard sections are out of order"})
			}
			if rank > lastRank {
				lastRank = rank
			}
		}
	}
	for i, section := range doc.Sections {
		if section.Kind == ass.SectionEvents && i != len(doc.Sections)-1 {
			for _, following := range doc.Sections[i+1:] {
				if following.Kind != ass.SectionProjectGarbage && following.Kind != ass.SectionFonts && following.Kind != ass.SectionExtradata {
					add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "events-not-last", Message: "[Events] must be the final standard section"})
					break
				}
			}
		}
		if section.Kind == ass.SectionStyles && len(section.Formats) == 0 {
			message := "styles section is missing a Format line"
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "styles-format-missing", Message: message, Edit: formatInsertionEdit(doc, &section, "styles", message)})
		}
		if section.Kind == ass.SectionEvents && len(section.Formats) == 0 {
			message := "events section is missing a Format line"
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "events-format-missing", Message: message, Edit: formatInsertionEdit(doc, &section, "events", message)})
		}
	}
	for _, line := range doc.Source.Lines {
		if len(bytes.TrimSpace(line.Content)) == 0 {
			continue
		}
		for _, section := range doc.Sections {
			if section.Kind != ass.SectionScriptInfo || line.Number < section.StartLine || line.Number > section.EndLine {
				continue
			}
			trimmed := strings.TrimSpace(string(line.Content))
			if strings.HasPrefix(trimmed, ";") {
				message := "semicolon comment is present in Script Info"
				add(Diagnostic{Line: line.Number, Severity: SeverityWarning, Code: "script-info-comment", Message: message, Edit: lineDeleteEdit(doc, line.Number, "script-info-comment", message)})
			}
		}
	}
}

func sectionRank(kind ass.SectionKind) int {
	switch kind {
	case ass.SectionScriptInfo:
		return 1
	case ass.SectionStyles:
		return 2
	case ass.SectionEvents:
		return 3
	default:
		return 0
	}
}

func runScriptInfo(doc *ass.Document, matrixMode string, add func(Diagnostic)) {
	section := doc.Section(ass.SectionScriptInfo)
	if section == nil {
		add(Diagnostic{Line: 1, Severity: SeverityError, Code: "script-info-missing", Message: "[Script Info] section is missing"})
		return
	}
	properties := map[string][]ass.Property{}
	for _, property := range section.Properties {
		key := strings.ToLower(property.Key)
		properties[key] = append(properties[key], property)
	}
	propertyKeys := make([]string, 0, len(properties))
	for key := range properties {
		propertyKeys = append(propertyKeys, key)
	}
	sort.Strings(propertyKeys)
	for _, key := range propertyKeys {
		values := properties[key]
		if len(values) > 1 {
			add(Diagnostic{Line: values[1].Line, Severity: SeverityWarning, Code: "duplicate-property", Message: fmt.Sprintf("Script Info property %q appears more than once", key), Manual: true})
		}
	}
	ensure := func(key, expected, code string) {
		values := properties[strings.ToLower(key)]
		if len(values) == 0 {
			message := fmt.Sprintf("%s is missing; expected %s", key, expected)
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: code, Message: message, Edit: insertPropertyEdit(doc, section, key, expected, code, message)})
			return
		}
		property := values[0]
		if property.Value == expected {
			return
		}
		message := fmt.Sprintf("%s should be %s", key, expected)
		edit := valueEdit(doc, property, expected, code, message)
		add(Diagnostic{Line: property.Line, Severity: SeverityError, Code: code, Message: message, Edit: edit})
	}
	ensure("ScriptType", "v4.00+", "script-type")
	ensure("WrapStyle", "2", "wrap-style")
	ensure("ScaledBorderAndShadow", "yes", "scaled-border-shadow")

	for _, key := range []string{"layoutresx", "layoutresy", "playresx", "playresy"} {
		values := properties[key]
		if len(values) == 0 {
			continue
		}
		if _, err := positiveInt(values[0].Value); err != nil {
			add(Diagnostic{Line: values[0].Line, Severity: SeverityError, Code: "resolution-invalid", Message: fmt.Sprintf("%s must be a positive integer", values[0].RawKey)})
		}
	}
	playX, playXOK := resolution(properties, "playresx")
	playY, playYOK := resolution(properties, "playresy")
	if playXOK && playYOK {
		for _, item := range []struct {
			key   string
			value int
		}{
			{key: "LayoutResX", value: playX},
			{key: "LayoutResY", value: playY},
		} {
			if len(properties[strings.ToLower(item.key)]) == 0 {
				message := fmt.Sprintf("%s is copied from PlayRes", item.key)
				add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "layout-resolution-copy", Message: message, Edit: insertPropertyEdit(doc, section, item.key, strconv.Itoa(item.value), "layout-resolution-copy", message)})
			}
		}
	}

	candidate, conflict := inferMatrix(properties)
	if conflict != "" {
		add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "matrix-resolution-conflict", Message: conflict})
	}
	matrixProperty := firstProperty(properties["ycbcr matrix"])
	current := ""
	if matrixProperty != nil {
		current = matrixProperty.Value
	}
	currentCanonical, currentValid := NormalizeMatrixValue(current)
	if currentValid && matrixProperty != nil && current != currentCanonical && strings.EqualFold(current, currentCanonical) {
		message := fmt.Sprintf("YCbCr Matrix uses non-canonical spelling; use %s", currentCanonical)
		add(Diagnostic{Line: matrixProperty.Line, Severity: SeverityWarning, Code: "matrix-case", Message: message, Edit: valueEdit(doc, *matrixProperty, currentCanonical, "matrix-case", message)})
	}
	if strings.EqualFold(strings.TrimSpace(matrixMode), "auto") {
		if currentValid && candidate != nil && currentCanonical != candidate.Value {
			add(Diagnostic{Line: matrixProperty.Line, Severity: SeverityWarning, Code: "matrix-resolution-mismatch", Message: fmt.Sprintf("existing YCbCr Matrix %s differs from %s", currentCanonical, candidate.Detail)})
		} else if !currentValid && candidate != nil {
			message := fmt.Sprintf("YCbCr Matrix should be %s", candidate.Detail)
			add(Diagnostic{Line: propertyLine(section, matrixProperty), Severity: SeverityError, Code: "matrix-missing", Message: message, Edit: matrixEdit(doc, section, matrixProperty, candidate.Value, candidate.Detail, "matrix-missing", message)})
		} else if !currentValid && candidate == nil {
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "matrix-manual", Message: "YCbCr Matrix is missing or invalid and cannot be inferred", Manual: true})
		}
	} else {
		explicit, ok := NormalizeMatrixValue(matrixMode)
		if !ok {
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityError, Code: "matrix-value", Message: fmt.Sprintf("invalid matrix value %q", matrixMode)})
			return
		}
		if currentCanonical != explicit || !currentValid {
			message := fmt.Sprintf("YCbCr Matrix should be %s (explicit override)", explicit)
			add(Diagnostic{Line: propertyLine(section, matrixProperty), Severity: SeverityWarning, Code: "matrix-explicit", Message: message, Edit: matrixEdit(doc, section, matrixProperty, explicit, "explicit override", "matrix-explicit", message)})
		}
		if candidate != nil && explicit != candidate.Value {
			add(Diagnostic{Line: section.HeaderLine, Severity: SeverityWarning, Code: "matrix-resolution-mismatch", Message: fmt.Sprintf("explicit Matrix %s differs from %s", explicit, candidate.Detail)})
		}
	}
}

func runStyles(doc *ass.Document, add func(Diagnostic)) {
	styles := make([]ass.Style, 0)
	for _, section := range doc.Sections {
		if section.Kind == ass.SectionStyles {
			styles = append(styles, section.Styles...)
		}
	}
	seen := map[string]ass.Style{}
	for _, style := range styles {
		key := strings.TrimSpace(style.Name)
		if key == "" {
			add(Diagnostic{Line: style.Line, Severity: SeverityWarning, Code: "style-name-empty", Message: "style name must not be empty", Manual: true})
		} else if previous, ok := seen[key]; ok {
			add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "style-duplicate", Message: fmt.Sprintf("style %q duplicates line %d", key, previous.Line)})
		} else {
			seen[key] = style
		}
		if len(style.Values) != len(style.Fields) && len(style.Fields) > 0 {
			add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "style-field-count", Message: "style field count does not match Format"})
			continue
		}
		field := func(name string) (string, ass.Span, bool) {
			index, ok := style.Fields[name]
			if !ok || index >= len(style.Values) || index >= len(style.Spans) {
				return "", ass.Span{}, false
			}
			return style.Values[index], style.Spans[index], true
		}
		if font, _, ok := field("fontname"); ok && utf8.RuneCountInString(font) > 31 {
			add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "font-name-length", Message: "Fontname must not exceed 31 characters"})
		}
		if size, _, ok := field("fontsize"); ok {
			value, err := strconv.ParseFloat(size, 64)
			if err != nil || value < 0 || value > 511 {
				add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "font-size", Message: "Fontsize must be between 0 and 511"})
			}
		}
		if border, _, ok := field("borderstyle"); ok && border != "1" && border != "3" {
			add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "border-style", Message: "BorderStyle must be 1 or 3"})
		}
		if alignment, _, ok := field("alignment"); ok {
			value, err := strconv.Atoi(alignment)
			if err != nil || value < 1 || value > 9 {
				add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "alignment", Message: "Alignment must be between 1 and 9"})
			}
		}
		for _, name := range []string{"primarycolour", "secondarycolour", "outlinecolour", "backcolour"} {
			value, span, ok := field(name)
			if !ok {
				continue
			}
			if strings.HasPrefix(strings.ToLower(value), "&h") {
				canonical := strings.ToUpper(value)
				if canonical != value {
					message := fmt.Sprintf("%s should use uppercase &H", name)
					add(Diagnostic{Line: style.Line, Severity: SeverityWarning, Code: "color-case", Message: message, Edit: spanEdit(doc, span, canonical, "color-case", message)})
				}
				if !validColor(value) {
					add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "color-value", Message: fmt.Sprintf("%s is not a valid ASS color", name)})
				}
			} else {
				add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "color-value", Message: fmt.Sprintf("%s is not a valid ASS color", name)})
			}
		}
		for _, name := range []string{"bold", "italic", "underline", "strikeout"} {
			value, span, ok := field(name)
			if !ok {
				continue
			}
			lower := strings.ToLower(value)
			canonical := ""
			switch lower {
			case "true", "yes":
				canonical = "-1"
			case "false", "no":
				canonical = "0"
			}
			if canonical != "" {
				message := fmt.Sprintf("%s should be %s", name, canonical)
				add(Diagnostic{Line: style.Line, Severity: SeverityWarning, Code: "boolean-value", Message: message, Edit: spanEdit(doc, span, canonical, "boolean-value", message)})
			} else if value != "0" && value != "-1" {
				add(Diagnostic{Line: style.Line, Severity: SeverityError, Code: "boolean-value", Message: fmt.Sprintf("%s must be 0 or -1", name)})
			}
		}
		if encoding, span, ok := field("encoding"); ok && encoding != "1" {
			message := "Encoding should be 1"
			add(Diagnostic{Line: style.Line, Severity: SeverityWarning, Code: "encoding", Message: message, Edit: spanEdit(doc, span, "1", "encoding", message)})
		}
	}
	if _, ok := seen["Default"]; !ok {
		add(Diagnostic{Line: firstStylesLine(doc), Severity: SeverityWarning, Code: "default-style-missing", Message: "a case-sensitive Default style is required", Manual: true})
	}
}

func runEvents(doc *ass.Document, add func(Diagnostic)) {
	styles := make([]ass.Style, 0)
	for _, section := range doc.Sections {
		if section.Kind == ass.SectionStyles {
			styles = append(styles, section.Styles...)
		}
	}
	exact := map[string]bool{}
	folded := map[string][]string{}
	for _, style := range styles {
		exact[style.Name] = true
		folded[strings.ToLower(style.Name)] = append(folded[strings.ToLower(style.Name)], style.Name)
	}
	for _, section := range doc.Sections {
		if section.Kind != ass.SectionEvents {
			continue
		}
		format := latestFormat(&section)
		for _, event := range section.Events {
			if format != nil && len(event.Values) != len(format.Fields) {
				add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "event-field-count", Message: "event field count does not match Format"})
				continue
			}
			if event.StartRaw != "" {
				if _, err := ass.ParseTime(event.StartRaw); err != nil {
					add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "time-invalid", Message: fmt.Sprintf("invalid start time %q", event.StartRaw)})
				}
			}
			if event.EndRaw != "" {
				if _, err := ass.ParseTime(event.EndRaw); err != nil {
					add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "time-invalid", Message: fmt.Sprintf("invalid end time %q", event.EndRaw)})
				}
			}
			if event.StartRaw != "" && event.EndRaw != "" {
				start, startErr := ass.ParseTime(event.StartRaw)
				end, endErr := ass.ParseTime(event.EndRaw)
				if startErr == nil && endErr == nil && start > end {
					add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "time-order", Message: "event start time is after end time"})
				}
			}
			if layerIndex, ok := event.Fields["layer"]; ok && layerIndex < len(event.Values) {
				if value, err := strconv.Atoi(strings.TrimSpace(event.Values[layerIndex])); err != nil || value < 0 {
					add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "layer-invalid", Message: "Layer must be a non-negative integer"})
				}
			}
			for _, marginName := range []string{"marginl", "marginr", "marginv"} {
				if index, ok := event.Fields[marginName]; ok && index < len(event.Values) {
					if value, err := strconv.Atoi(strings.TrimSpace(event.Values[index])); err != nil || value < 0 {
						add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "margin-invalid", Message: fmt.Sprintf("%s must be a non-negative integer", marginName)})
					}
				}
			}
			if effectIndex, ok := event.Fields["effect"]; ok && effectIndex < len(event.Values) && strings.TrimSpace(event.Values[effectIndex]) != "" {
				add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "event-effect", Message: "non-empty Effect is preserved for manual review", Manual: true})
			}
			if event.Style != "" {
				if !exact[event.Style] {
					candidates := folded[strings.ToLower(event.Style)]
					if len(candidates) == 1 {
						message := fmt.Sprintf("style %q should be %q", event.Style, candidates[0])
						span := eventFieldSpan(event, "style")
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "style-case", Message: message, Edit: spanEdit(doc, span, candidates[0], "style-case", message)})
					} else {
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "style-undefined", Message: fmt.Sprintf("style %q is not defined", event.Style), Manual: true})
					}
				}
			} else {
				if _, exists := event.Fields["style"]; exists {
					span := eventFieldSpan(event, "style")
					if exact["Default"] {
						message := "empty style reference should use Default"
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "style-empty", Message: message, Edit: spanEdit(doc, span, "Default", "style-empty", message)})
					} else {
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "style-empty", Message: "empty style reference cannot be resolved without Default", Manual: true})
					}
				}
			}
			if event.Name != "" {
				span := eventFieldSpan(event, "name")
				message := "event Name is cleared for normalized delivery"
				add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "event-name", Message: message, Edit: spanEdit(doc, span, "", "event-name", message)})
			}
			if event.Kind == "Dialogue" && event.StartRaw != "" && event.EndRaw != "" {
				start, startErr := ass.ParseTime(event.StartRaw)
				end, endErr := ass.ParseTime(event.EndRaw)
				if startErr == nil && endErr == nil && start == end {
					message := "zero-duration Dialogue is converted to Comment"
					add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "zero-duration", Message: message, Edit: EditForPrefix(doc, event, "Comment", "zero-duration", message)})
				}
			}
		}
	}
}

func runOverrides(doc *ass.Document, add func(Diagnostic)) {
	wrapStyle := ""
	if properties := doc.ScriptProperties(); properties["wrapstyle"].Value != "" {
		wrapStyle = properties["wrapstyle"].Value
	}
	for _, section := range doc.Sections {
		if section.Kind != ass.SectionEvents {
			continue
		}
		for _, event := range section.Events {
			format := latestFormat(&section)
			if format == nil || len(event.Values) != len(format.Fields) {
				continue
			}
			if event.Kind == "Dialogue" {
				for _, issue := range ass.ScanOverrideSyntax(event) {
					add(Diagnostic{Line: event.Line, Severity: SeverityError, Code: "override-syntax", Message: issue})
				}
			}
			qConflict := false
			drawingMode := false
			for _, block := range ass.ScanOverrides(event) {
				for _, tag := range block.Tags {
					if !tag.Known {
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "unknown-override", Message: fmt.Sprintf("unknown override tag \\%s is preserved", tag.Name), Manual: true})
					}
					if strings.EqualFold(tag.Name, "q") {
						value := strings.TrimSpace(strings.TrimPrefix(tag.Arguments, "("))
						value = strings.TrimSuffix(value, ")")
						if value != "" && value != "2" {
							qConflict = true
						}
					}
					if strings.EqualFold(tag.Name, "p") {
						value := strings.TrimSpace(tag.Arguments)
						if value != "" {
							if number, err := strconv.Atoi(value); err == nil && number > 0 {
								drawingMode = true
							}
						}
					}
					if strings.EqualFold(tag.Name, "a") {
						value := strings.TrimSpace(tag.Arguments)
						if number, err := strconv.Atoi(value); err == nil && number >= 1 && number <= 9 {
							message := fmt.Sprintf("legacy \\a%s is mapped to \\an%s", value, value)
							add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "legacy-alignment", Message: message, Edit: spanEdit(doc, tag.Span, "\\an"+value, "legacy-alignment", message)})
						} else {
							add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "legacy-alignment", Message: "legacy \\a alignment is ambiguous and is preserved", Manual: true})
						}
					}
				}
			}
			if strings.EqualFold(wrapStyle, "2") && strings.Contains(event.Text, `\n`) {
				if drawingMode {
					add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "lowercase-break", Message: "lowercase \\n is preserved inside drawing data", Manual: true})
				} else if qConflict {
					add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "lowercase-break", Message: "lowercase \\n is preserved because a conflicting \\q tag is present", Manual: true})
				} else {
					replacement := replaceOutsideBraces(event.Text, `\n`, `\N`)
					if replacement != event.Text {
						message := "lowercase \\n is normalized to \\N under WrapStyle 2"
						add(Diagnostic{Line: event.Line, Severity: SeverityWarning, Code: "lowercase-break", Message: message, Edit: spanEdit(doc, event.TextSpan, replacement, "lowercase-break", message)})
					}
				}
			}
		}
	}
}

func inferMatrix(properties map[string][]ass.Property) (*MatrixCandidate, string) {
	lx, lxOK := resolution(properties, "layoutresx")
	ly, lyOK := resolution(properties, "layoutresy")
	px, pxOK := resolution(properties, "playresx")
	py, pyOK := resolution(properties, "playresy")
	layout, layoutOK := classifyResolution(lx, ly, lxOK && lyOK, "LayoutRes")
	play, playOK := classifyResolution(px, py, pxOK && pyOK, "PlayRes")
	if layoutOK && playOK && layout.Value != play.Value {
		return &layout, fmt.Sprintf("LayoutRes %dx%d suggests %s but PlayRes %dx%d suggests %s", lx, ly, layout.Value, px, py, play.Value)
	}
	if layoutOK {
		return &layout, ""
	}
	if playOK {
		return &play, ""
	}
	return nil, ""
}

func classifyResolution(x, y int, ok bool, source string) (MatrixCandidate, bool) {
	if !ok {
		return MatrixCandidate{}, false
	}
	switch y {
	case 1080:
		return MatrixCandidate{Value: "TV.709", Source: source, Detail: fmt.Sprintf("TV.709 (inferred from %s %dx%d)", source, x, y)}, true
	case 720:
		return MatrixCandidate{Value: "TV.601", Source: source, Detail: fmt.Sprintf("TV.601 (inferred from %s %dx%d)", source, x, y)}, true
	default:
		return MatrixCandidate{}, false
	}
}

func resolution(properties map[string][]ass.Property, key string) (int, bool) {
	property := firstProperty(properties[key])
	if property == nil {
		return 0, false
	}
	value, err := positiveInt(property.Value)
	return value, err == nil
}

func positiveInt(value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not positive")
	}
	return n, nil
}

func validColor(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 || !strings.EqualFold(trimmed[:2], "&H") {
		return false
	}
	digits := strings.TrimSuffix(trimmed[2:], "&")
	if len(digits) == 0 || len(digits) > 8 {
		return false
	}
	_, err := strconv.ParseUint(digits, 16, 32)
	return err == nil
}

func firstProperty(properties []ass.Property) *ass.Property {
	if len(properties) == 0 {
		return nil
	}
	property := properties[0]
	return &property
}

func propertyLine(section *ass.Section, property *ass.Property) int {
	if property != nil {
		return property.Line
	}
	return section.HeaderLine
}

func matrixEdit(doc *ass.Document, section *ass.Section, property *ass.Property, value, detail, code, message string) *Edit {
	if property != nil {
		return valueEdit(doc, *property, value, code, message)
	}
	position := insertionPosition(doc, section)
	lineEnding := dominantNewline(doc)
	return &Edit{Line: section.HeaderLine, Start: position, End: position, Replacement: []byte(insertionPrefix(doc, section) + "YCbCr Matrix: " + value + lineEnding), Code: code, Description: detail, Before: "<missing>", After: "YCbCr Matrix: " + value, Safe: true}
}

func insertPropertyEdit(doc *ass.Document, section *ass.Section, key, value, code, message string) *Edit {
	position := insertionPosition(doc, section)
	lineEnding := dominantNewline(doc)
	return &Edit{Line: section.HeaderLine, Start: position, End: position, Replacement: []byte(insertionPrefix(doc, section) + key + ": " + value + lineEnding), Code: code, Description: message, Before: "<missing>", After: key + ": " + value, Safe: true}
}

func formatInsertionEdit(doc *ass.Document, section *ass.Section, kind, message string) *Edit {
	format := ""
	if kind == "styles" {
		format = "Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding"
	} else {
		format = "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text"
	}
	position := insertionPosition(doc, section)
	if header, ok := doc.Source.Line(section.HeaderLine); ok {
		position = header.End
	}
	lineEnding := dominantNewline(doc)
	prefix := ""
	if header, ok := doc.Source.Line(section.HeaderLine); ok && len(header.Terminator) == 0 {
		prefix = lineEnding
	}
	return &Edit{Line: section.HeaderLine, Start: position, End: position, Replacement: []byte(prefix + format + lineEnding), Code: kind + "-format-missing", Description: message, Before: "<missing>", After: format, Safe: true}
}

func valueEdit(doc *ass.Document, property ass.Property, value, code, message string) *Edit {
	return &Edit{Line: property.Line, Start: property.ValueSpan.Start, End: property.ValueSpan.End, Replacement: []byte(value), Code: code, Description: message, Before: property.Value, After: value, Safe: true}
}

func spanEdit(doc *ass.Document, span ass.Span, value, code, message string) *Edit {
	if span.End < span.Start {
		span.End = span.Start
	}
	before := ""
	if span.Start >= 0 && span.End <= len(doc.Source.Original) {
		before = string(doc.Source.Original[span.Start:span.End])
	}
	line := lineForOffset(doc.Source, span.Start)
	return &Edit{Line: line, Start: span.Start, End: span.End, Replacement: []byte(value), Code: code, Description: message, Before: before, After: value, Safe: true}
}

func lineForOffset(source *ass.Source, offset int) int {
	for _, line := range source.Lines {
		if offset >= line.Start && offset <= line.End {
			return line.Number
		}
	}
	if len(source.Lines) == 0 {
		return 1
	}
	return source.Lines[len(source.Lines)-1].Number
}

func eventFieldSpan(event ass.Event, name string) ass.Span {
	index, ok := event.Fields[name]
	if !ok || index >= len(event.Spans) {
		return ass.Span{}
	}
	return event.Spans[index]
}

func EditForPrefix(doc *ass.Document, event ass.Event, prefix, code, message string) *Edit {
	return &Edit{Line: event.Line, Start: event.KindSpan.Start, End: event.KindSpan.End, Replacement: []byte(prefix), Code: code, Description: message, Before: event.Prefix, After: prefix, Safe: true}
}

func lineDeleteEdit(doc *ass.Document, lineNumber int, code, message string) *Edit {
	line, ok := doc.Source.Line(lineNumber)
	if !ok {
		return nil
	}
	return &Edit{Line: lineNumber, Start: line.Start, End: line.End, Replacement: nil, Code: code, Description: message, Before: string(line.Content), After: "", Safe: true}
}

func sectionDeleteEdit(doc *ass.Document, section *ass.Section, code, message string) *Edit {
	first, ok1 := doc.Source.Line(section.StartLine)
	last, ok2 := doc.Source.Line(section.EndLine)
	if !ok1 || !ok2 {
		return nil
	}
	return &Edit{Line: section.StartLine, Start: first.Start, End: last.End, Replacement: nil, Code: code, Description: message, Before: fmt.Sprintf("lines %d-%d", section.StartLine, section.EndLine), After: "", Safe: true}
}

func firstStylesLine(doc *ass.Document) int {
	if section := doc.Section(ass.SectionStyles); section != nil {
		return section.HeaderLine
	}
	return 1
}

func insertionPosition(doc *ass.Document, section *ass.Section) int {
	line, ok := doc.Source.Line(section.EndLine)
	if !ok {
		return len(doc.Source.Original)
	}
	return line.End
}

func insertionPrefix(doc *ass.Document, section *ass.Section) string {
	line, ok := doc.Source.Line(section.EndLine)
	if ok && len(line.Terminator) == 0 && line.End == len(doc.Source.Original) {
		return dominantNewline(doc)
	}
	return ""
}

func dominantNewline(doc *ass.Document) string {
	if doc.Source.DominantNewline == ass.NewlineCRLF {
		return "\r\n"
	}
	return "\n"
}

func replaceOutsideBraces(text, from, to string) string {
	var builder strings.Builder
	depth := 0
	for i := 0; i < len(text); {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' && depth > 0 {
			depth--
		}
		if depth == 0 && strings.HasPrefix(text[i:], from) {
			builder.WriteString(to)
			i += len(from)
			continue
		}
		builder.WriteByte(text[i])
		i++
	}
	return builder.String()
}

func suppressOverlappingEdits(edits []Edit) []Edit {
	if len(edits) == 0 {
		return nil
	}
	ordered := append([]Edit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start != ordered[j].Start {
			return ordered[i].Start < ordered[j].Start
		}
		return ordered[i].End > ordered[j].End
	})
	result := make([]Edit, 0, len(ordered))
	for _, edit := range ordered {
		suppressed := false
		for _, kept := range result {
			if edit.Start >= kept.Start && edit.End <= kept.End {
				suppressed = true
				break
			}
			if edit.Start < kept.End && kept.Start < edit.End {
				suppressed = true
				break
			}
		}
		if !suppressed {
			result = append(result, edit)
		}
	}
	return result
}

func latestFormat(section *ass.Section) *ass.Format {
	if section == nil || len(section.Formats) == 0 {
		return nil
	}
	return &section.Formats[len(section.Formats)-1]
}

func ToReplacements(edits []Edit) []ass.Replacement {
	replacements := make([]ass.Replacement, 0, len(edits))
	for _, edit := range edits {
		replacements = append(replacements, replacementFromEdit(edit))
	}
	return replacements
}
