package commands

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"asstools/internal/ass"
	"asstools/internal/output"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func Info(path string, out, errOut io.Writer) int {
	return InfoWithJSONOptions(path, false, false, out, errOut)
}

func InfoWithOptions(path string, strict bool, out, errOut io.Writer) int {
	return InfoWithJSONOptions(path, strict, false, out, errOut)
}

func InfoReader(path string, in io.Reader, out, errOut io.Writer) int {
	return InfoReaderWithJSONOptions(path, in, false, false, out, errOut)
}

func InfoReaderWithOptions(path string, in io.Reader, strict bool, out, errOut io.Writer) int {
	return InfoReaderWithJSONOptions(path, in, strict, false, out, errOut)
}

func InfoWithJSONOptions(path string, strict, jsonOutput bool, out, errOut io.Writer) int {
	return info(path, nil, strict, jsonOutput, out, errOut)
}

func InfoReaderWithJSONOptions(path string, in io.Reader, strict, jsonOutput bool, out, errOut io.Writer) int {
	return info(path, in, strict, jsonOutput, out, errOut)
}

func info(path string, in io.Reader, strict, jsonOutput bool, out, errOut io.Writer) (code int) {
	path = cleanPath(path)
	trackedOut := output.Track(out)
	trackedErrOut := output.Track(errOut)
	out = trackedOut
	errOut = trackedErrOut
	defer func() {
		if trackedOut.Err() != nil || trackedErrOut.Err() != nil {
			code = 2
		}
	}()
	var doc *ass.Document
	var result rules.Result
	var err error
	if in == nil {
		doc, result, err = load(path, "auto")
	} else {
		doc, result, err = loadReader(in, "auto")
	}
	if err != nil {
		if jsonOutput {
			if jsonErr := writeJSONError(errOut, "info", err); jsonErr != nil {
				return 2
			}
			return 2
		}
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: %s", err)))
		return 2
	}
	if jsonOutput {
		if err := encodeJSON(out, infoJSONPayload(path, doc, result)); err != nil {
			return 2
		}
		if strict && (result.ErrorCount() > 0 || result.ManualCount() > 0) {
			return 1
		}
		return 0
	}
	source := doc.Source
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold+terminal.Cyan, "== File =="))
	fmt.Fprintf(out, "Path: %s\n", formatEditValue(path))
	fmt.Fprintf(out, "Size: %d bytes\n", len(source.Original))
	fmt.Fprintln(out, "Encoding: UTF-8")
	fmt.Fprintf(out, "BOM: %s\n", yesNo(source.BOM))
	lf, crlf := newlineCounts(source)
	fmt.Fprintf(out, "Line endings: CRLF (%d), LF (%d), mixed: %s\n", crlf, lf, yesNo(source.Mixed))
	fmt.Fprintf(out, "Trailing newline: %s\n", yesNo(source.TrailingNewline))

	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Cyan, "== Structure =="))
	fmt.Fprintf(out, "Sections: %d\n", len(doc.Sections))
	for _, section := range doc.Sections {
		fmt.Fprintf(out, "  [%s] lines %d-%d\n", section.RawName, section.StartLine, section.EndLine)
	}
	properties := doc.ScriptProperties()
	for _, key := range []string{"scripttype", "wrapstyle", "scaledborderandshadow", "ycbcr matrix", "layoutresx", "layoutresy", "playresx", "playresy"} {
		if property, ok := properties[key]; ok {
			label := property.RawKey
			if label == "" {
				label = key
			}
			fmt.Fprintf(out, "%s: %s\n", label, property.Value)
		}
	}
	if candidate, conflict := rules.InferMatrix(doc); candidate != nil {
		fmt.Fprintf(out, "Matrix candidate: %s\n", candidate.Detail)
		if conflict != "" {
			fmt.Fprintf(out, "Matrix resolution note: %s\n", conflict)
		}
	} else {
		fmt.Fprintln(out, "Matrix candidate: unavailable")
	}

	styles, fonts, undefined := styleRowsSummary(doc)
	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Cyan, "== Styles =="))
	fmt.Fprintf(out, "Definitions: %d\n", len(styles))
	if err := printStyleTable(out, styles); err != nil {
		return 2
	}
	fmt.Fprintf(out, "Fonts used: %s\n", joinOrNone(fonts))
	fmt.Fprintf(out, "Undefined style references: %d\n", undefined)

	dialogues, comments, earliest, latest, minLayer, maxLayer := eventSummary(doc)
	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Cyan, "== Events =="))
	fmt.Fprintf(out, "Dialogue: %d\n", dialogues)
	fmt.Fprintf(out, "Comment: %d\n", comments)
	if earliest >= 0 {
		fmt.Fprintf(out, "Time span: %s - %s (%s)\n", earliest.String(), latest.String(), ass.Time(int64(latest)-int64(earliest)).String())
	} else {
		fmt.Fprintln(out, "Time span: none")
	}
	if minLayer <= maxLayer {
		fmt.Fprintf(out, "Layer range: %d-%d\n", minLayer, maxLayer)
	} else {
		fmt.Fprintln(out, "Layer range: none")
	}

	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Cyan, "== Compliance =="))
	if err := printSummary(out, result); err != nil {
		return 2
	}
	if err := printComplianceDetails(out, result); err != nil {
		return 2
	}
	if strict && (result.ErrorCount() > 0 || result.ManualCount() > 0) {
		return 1
	}
	return 0
}

const (
	styleTableMaxWidth = 120
	styleCellMaxWidth  = 32
)

type styleColumn struct {
	key   string
	label string
}

type styleRow struct {
	name   string
	font   string
	size   string
	fields []styleColumn
	values map[string]string
}

func styleSummary(doc *ass.Document) ([]string, []string, int) {
	rows, fonts, undefined := styleRowsSummary(doc)
	styles := make([]string, 0, len(rows))
	for _, row := range rows {
		styles = append(styles, fmt.Sprintf("%s  %s  %s", row.name, row.font, row.size))
	}
	return styles, fonts, undefined
}

func styleRowsSummary(doc *ass.Document) ([]styleRow, []string, int) {
	styles := make([]styleRow, 0)
	fonts := map[string]bool{}
	defined := map[string]bool{}
	definedFolded := map[string]int{}
	for _, section := range doc.Sections {
		if section.Kind != ass.SectionStyles {
			continue
		}
		for _, style := range section.Styles {
			defined[style.Name] = true
			definedFolded[strings.ToLower(style.Name)]++
			font := styleValue(style, "fontname")
			if font != "" {
				fonts[font] = true
			}
			values := map[string]string{"name": style.Name}
			fields := styleFieldsFor(section, style)
			for _, field := range fields {
				values[field.key] = styleValue(style, field.key)
			}
			values["name"] = style.Name
			styles = append(styles, styleRow{name: style.Name, font: font, size: styleValue(style, "fontsize"), fields: fields, values: values})
		}
	}
	undefined := 0
	for _, section := range doc.Sections {
		if section.Kind != ass.SectionEvents {
			continue
		}
		for _, event := range section.Events {
			if event.Style != "" && !defined[event.Style] && definedFolded[strings.ToLower(event.Style)] != 1 {
				undefined++
			}
			for _, block := range ass.ScanOverrides(event) {
				for _, tag := range block.Tags {
					if strings.EqualFold(tag.Name, "fn") {
						font := strings.TrimSpace(strings.TrimPrefix(tag.Arguments, "("))
						font = strings.TrimSuffix(font, ")")
						if font != "" {
							fonts[font] = true
						}
					}
				}
			}
		}
	}
	fontList := fontsSlice(fonts)
	sort.Strings(fontList)
	return styles, fontList, undefined
}

func printStyleTable(out io.Writer, styles []styleRow) error {
	columns := styleColumns(styles)
	for index, group := range splitStyleColumns(columns, styles) {
		if index > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		var rendered bytes.Buffer
		table := tabwriter.NewWriter(&rendered, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, styleTableLine(group, nil)); err != nil {
			return err
		}
		for _, style := range styles {
			if _, err := fmt.Fprintln(table, styleTableLine(group, &style)); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
		if err := writeStyleTable(out, rendered.String()); err != nil {
			return err
		}
	}
	return nil
}

func styleFieldsFor(section ass.Section, style ass.Style) []styleColumn {
	var fields []string
	if style.Line == 0 && len(section.Formats) > 0 {
		fields = section.Formats[len(section.Formats)-1].Fields
	}
	for _, format := range section.Formats {
		if format.Line > style.Line {
			break
		}
		fields = format.Fields
	}
	result := make([]styleColumn, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		label := strings.TrimSpace(field)
		key := strings.ToLower(label)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, styleColumn{key: key, label: label})
	}
	return result
}

func styleColumns(styles []styleRow) []styleColumn {
	columns := []styleColumn{{key: "name", label: "Name"}}
	seen := map[string]bool{"name": true}
	for _, style := range styles {
		for _, field := range style.fields {
			if seen[field.key] {
				continue
			}
			seen[field.key] = true
			columns = append(columns, field)
		}
	}
	for _, field := range []styleColumn{{key: "fontname", label: "Fontname"}, {key: "fontsize", label: "Fontsize"}} {
		if seen[field.key] {
			continue
		}
		seen[field.key] = true
		columns = append(columns, field)
	}
	return columns
}

func splitStyleColumns(columns []styleColumn, styles []styleRow) [][]styleColumn {
	if len(columns) == 0 {
		return nil
	}
	name := columns[0]
	nameWidth := 2 + styleColumnWidth(name, styles)
	groups := make([][]styleColumn, 0, 1)
	current := []styleColumn{name}
	currentWidth := nameWidth
	for _, column := range columns[1:] {
		columnWidth := styleColumnWidth(column, styles)
		if len(current) > 1 && currentWidth+2+columnWidth > styleTableMaxWidth {
			groups = append(groups, current)
			current = []styleColumn{name}
			currentWidth = nameWidth
		}
		current = append(current, column)
		currentWidth += 2 + columnWidth
	}
	return append(groups, current)
}

func styleColumnWidth(column styleColumn, styles []styleRow) int {
	width := len([]rune(styleCell(column.label)))
	for _, style := range styles {
		if valueWidth := len([]rune(styleCell(styleRowValue(style, column.key)))); valueWidth > width {
			width = valueWidth
		}
	}
	return width
}

func styleTableLine(columns []styleColumn, style *styleRow) string {
	cells := make([]string, 0, len(columns))
	for index, column := range columns {
		value := column.label
		if style != nil {
			value = styleRowValue(*style, column.key)
		}
		value = styleCell(value)
		if index == 0 {
			value = "  " + value
		}
		cells = append(cells, value)
	}
	return strings.Join(cells, "\t")
}

func styleRowValue(style styleRow, key string) string {
	value := style.values[key]
	if value != "" {
		return value
	}
	switch key {
	case "name":
		return style.name
	case "fontname":
		return style.font
	case "fontsize":
		return style.size
	default:
		return ""
	}
}

func styleCell(value string) string {
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	runes := []rune(value)
	if len(runes) <= styleCellMaxWidth {
		return value
	}
	return string(runes[:styleCellMaxWidth-3]) + "..."
}

func writeStyleTable(out io.Writer, rendered string) error {
	newline := strings.IndexByte(rendered, '\n')
	if newline < 0 {
		written, err := io.WriteString(out, rendered)
		if err == nil && written != len(rendered) {
			return io.ErrShortWrite
		}
		return err
	}
	header := terminal.Color(out, terminal.Bold+terminal.Cyan, rendered[:newline])
	written, err := io.WriteString(out, header)
	if err != nil {
		return err
	}
	if written != len(header) {
		return io.ErrShortWrite
	}
	written, err = io.WriteString(out, rendered[newline:])
	if err == nil && written != len(rendered[newline:]) {
		return io.ErrShortWrite
	}
	return err
}

func eventSummary(doc *ass.Document) (int, int, ass.Time, ass.Time, int, int) {
	dialogues, comments := 0, 0
	var earliest, latest ass.Time
	haveTime := false
	minLayer, maxLayer := 0, -1
	haveLayer := false
	for _, section := range doc.Sections {
		if section.Kind != ass.SectionEvents {
			continue
		}
		for _, event := range section.Events {
			if event.Kind == "Dialogue" {
				dialogues++
			} else {
				comments++
			}
			if !haveLayer || event.Layer < minLayer {
				minLayer = event.Layer
			}
			if !haveLayer || event.Layer > maxLayer {
				maxLayer = event.Layer
			}
			haveLayer = true
			start, startErr := ass.ParseTime(event.StartRaw)
			end, endErr := ass.ParseTime(event.EndRaw)
			if startErr == nil && endErr == nil {
				if !haveTime || start < earliest {
					earliest = start
				}
				if !haveTime || end > latest {
					latest = end
				}
				haveTime = true
			}
		}
	}
	if !haveTime {
		return dialogues, comments, -1, -1, minLayer, maxLayer
	}
	return dialogues, comments, earliest, latest, minLayer, maxLayer
}

func newlineCounts(source *ass.Source) (int, int) {
	lf, crlf := 0, 0
	for _, line := range source.Lines {
		if bytes.Equal(line.Terminator, []byte("\r\n")) {
			crlf++
		} else if bytes.Equal(line.Terminator, []byte("\n")) {
			lf++
		}
	}
	return lf, crlf
}

func styleValue(style ass.Style, name string) string {
	index, ok := style.Fields[name]
	if !ok || index >= len(style.Values) {
		return ""
	}
	return style.Values[index]
}

func fontsSlice(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

type infoJSON struct {
	Command     string            `json:"command"`
	Status      string            `json:"status"`
	Path        string            `json:"path"`
	File        infoFileJSON      `json:"file"`
	Structure   infoStructureJSON `json:"structure"`
	Styles      infoStylesJSON    `json:"styles"`
	Events      infoEventsJSON    `json:"events"`
	Summary     jsonSummary       `json:"summary"`
	Diagnostics []jsonDiagnostic  `json:"diagnostics"`
}

type infoFileJSON struct {
	SizeBytes       int                 `json:"size_bytes"`
	Encoding        string              `json:"encoding"`
	BOM             bool                `json:"bom"`
	LineEndings     infoLineEndingsJSON `json:"line_endings"`
	TrailingNewline bool                `json:"trailing_newline"`
}

type infoLineEndingsJSON struct {
	CRLF  int  `json:"crlf"`
	LF    int  `json:"lf"`
	Mixed bool `json:"mixed"`
}

type infoStructureJSON struct {
	Sections              []infoSectionJSON `json:"sections"`
	Properties            map[string]string `json:"properties"`
	MatrixCandidate       string            `json:"matrix_candidate"`
	MatrixCandidateReason string            `json:"matrix_candidate_reason"`
	MatrixResolutionNote  string            `json:"matrix_resolution_note"`
}

type infoSectionJSON struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type infoStylesJSON struct {
	Definitions              []map[string]string `json:"definitions"`
	Fields                   []string            `json:"fields,omitempty"`
	FontsUsed                []string            `json:"fonts_used"`
	UndefinedStyleReferences int                 `json:"undefined_style_references"`
}

type infoEventsJSON struct {
	Dialogue   int                 `json:"dialogue"`
	Comment    int                 `json:"comment"`
	TimeSpan   *infoTimeSpanJSON   `json:"time_span"`
	LayerRange *infoLayerRangeJSON `json:"layer_range"`
}

type infoTimeSpanJSON struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Duration string `json:"duration"`
}

type infoLayerRangeJSON struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

func infoJSONPayload(path string, doc *ass.Document, result rules.Result) infoJSON {
	source := doc.Source
	lf, crlf := newlineCounts(source)
	sections := make([]infoSectionJSON, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		sections = append(sections, infoSectionJSON{
			Name:      section.RawName,
			Kind:      string(section.Kind),
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
		})
	}
	properties := make(map[string]string)
	for key, property := range doc.ScriptProperties() {
		label := property.RawKey
		if label == "" {
			label = key
		}
		properties[label] = property.Value
	}
	matrixCandidate := ""
	matrixCandidateReason := ""
	matrixResolutionNote := ""
	if candidate, conflict := rules.InferMatrix(doc); candidate != nil {
		matrixCandidate = candidate.Value
		matrixCandidateReason = matrixCandidateReasonJSON(candidate)
		matrixResolutionNote = conflict
	}
	styles, fonts, undefined := styleRowsSummary(doc)
	definitions := make([]map[string]string, 0, len(styles))
	for _, style := range styles {
		values := make(map[string]string, len(style.values))
		seenKeys := make(map[string]bool, len(style.fields))
		for _, field := range style.fields {
			if value, ok := style.values[field.key]; ok {
				values[styleJSONFieldName(field.label)] = value
				seenKeys[field.key] = true
			}
		}
		for key, value := range style.values {
			if !seenKeys[key] {
				values[styleJSONFieldName(key)] = value
			}
		}
		definitions = append(definitions, values)
	}
	commonFields := commonStyleFields(styles)
	dialogues, comments, earliest, latest, minLayer, maxLayer := eventSummary(doc)
	var timeSpan *infoTimeSpanJSON
	if earliest >= 0 {
		timeSpan = &infoTimeSpanJSON{
			Start:    earliest.String(),
			End:      latest.String(),
			Duration: ass.Time(int64(latest) - int64(earliest)).String(),
		}
	}
	var layerRange *infoLayerRangeJSON
	if minLayer <= maxLayer {
		layerRange = &infoLayerRangeJSON{Min: minLayer, Max: maxLayer}
	}
	return infoJSON{
		Command: "info",
		Status:  resultStatus(result),
		Path:    path,
		File: infoFileJSON{
			SizeBytes:       len(source.Original),
			Encoding:        "UTF-8",
			BOM:             source.BOM,
			LineEndings:     infoLineEndingsJSON{CRLF: crlf, LF: lf, Mixed: source.Mixed},
			TrailingNewline: source.TrailingNewline,
		},
		Structure: infoStructureJSON{
			Sections:              sections,
			Properties:            properties,
			MatrixCandidate:       matrixCandidate,
			MatrixCandidateReason: matrixCandidateReason,
			MatrixResolutionNote:  matrixResolutionNote,
		},
		Styles: infoStylesJSON{
			Definitions:              definitions,
			Fields:                   commonFields,
			FontsUsed:                fonts,
			UndefinedStyleReferences: undefined,
		},
		Events: infoEventsJSON{
			Dialogue:   dialogues,
			Comment:    comments,
			TimeSpan:   timeSpan,
			LayerRange: layerRange,
		},
		Summary:     summaryJSON(result),
		Diagnostics: diagnosticsJSON(result.Diagnostics),
	}
}

func matrixCandidateReasonJSON(candidate *rules.MatrixCandidate) string {
	if candidate == nil {
		return ""
	}
	reason := strings.TrimSpace(strings.TrimPrefix(candidate.Detail, candidate.Value))
	reason = strings.TrimPrefix(reason, "(")
	reason = strings.TrimSuffix(reason, ")")
	return strings.TrimSpace(reason)
}

func commonStyleFields(styles []styleRow) []string {
	if len(styles) == 0 {
		return nil
	}
	common := styleFieldLabels(styles[0].fields)
	for _, style := range styles[1:] {
		fields := styleFieldLabels(style.fields)
		if len(fields) != len(common) {
			return nil
		}
		for index := range common {
			if fields[index] != common[index] {
				return nil
			}
		}
	}
	return common
}

func styleFieldLabels(fields []styleColumn) []string {
	labels := make([]string, 0, len(fields))
	for _, field := range fields {
		labels = append(labels, styleJSONFieldName(field.label))
	}
	return labels
}

func styleJSONFieldName(key string) string {
	trimmed := strings.TrimSpace(key)
	switch strings.ToLower(trimmed) {
	case "name":
		return "name"
	case "fontname":
		return "font_name"
	case "fontsize":
		return "font_size"
	case "primarycolour":
		return "primary_colour"
	case "secondarycolour":
		return "secondary_colour"
	case "outlinecolour":
		return "outline_colour"
	case "backcolour":
		return "back_colour"
	case "borderstyle":
		return "border_style"
	case "scalex":
		return "scale_x"
	case "scaley":
		return "scale_y"
	case "strikeout":
		return "strike_out"
	case "marginl":
		return "margin_l"
	case "marginr":
		return "margin_r"
	case "marginv":
		return "margin_v"
	}
	var result strings.Builder
	var previous rune
	for index, r := range trimmed {
		if index > 0 && unicode.IsUpper(r) {
			if unicode.IsLower(previous) || unicode.IsDigit(previous) {
				result.WriteByte('_')
			}
		}
		if r == ' ' || r == '\t' || r == '-' || r == '.' {
			result.WriteByte('_')
			previous = r
			continue
		}
		result.WriteRune(unicode.ToLower(r))
		previous = r
	}
	return result.String()
}
