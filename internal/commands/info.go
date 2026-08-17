package commands

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"asstools/internal/ass"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func Info(path string, out, errOut io.Writer) int {
	doc, result, err := load(path, "auto")
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: %s", err)))
		return 2
	}
	source := doc.Source
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold+terminal.Cyan, "== File =="))
	fmt.Fprintf(out, "Path: %q\n", path)
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

	styles, fonts, undefined := styleSummary(doc)
	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Cyan, "== Styles =="))
	fmt.Fprintf(out, "Definitions: %d\n", len(styles))
	for _, style := range styles {
		fmt.Fprintf(out, "  %s\n", style)
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
	printSummary(out, result)
	return 0
}

func styleSummary(doc *ass.Document) ([]string, []string, int) {
	styles := make([]string, 0)
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
			styles = append(styles, fmt.Sprintf("%s  %s  %s", style.Name, font, styleValue(style, "fontsize")))
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
			if _, startErr := ass.ParseTime(event.StartRaw); startErr == nil {
				if !haveTime || event.Start < earliest {
					earliest = event.Start
				}
				if !haveTime || event.End > latest {
					latest = event.End
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
