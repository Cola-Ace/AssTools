package commands

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"asstools/internal/ass"
	"asstools/internal/rules"
)

func Info(path string, out, errOut io.Writer) int {
	doc, result, err := load(path, "auto")
	if err != nil {
		fmt.Fprintf(errOut, "asst: %s\n", err)
		return 2
	}
	source := doc.Source
	fmt.Fprintln(out, "== File ==")
	fmt.Fprintf(out, "Path: %q\n", path)
	fmt.Fprintf(out, "Size: %d bytes\n", len(source.Original))
	fmt.Fprintln(out, "Encoding: UTF-8")
	fmt.Fprintf(out, "BOM: %s\n", yesNo(source.BOM))
	lf, crlf := newlineCounts(source)
	fmt.Fprintf(out, "Line endings: CRLF (%d), LF (%d), mixed: %s\n", crlf, lf, yesNo(source.Mixed))
	fmt.Fprintf(out, "Trailing newline: %s\n", yesNo(source.TrailingNewline))

	fmt.Fprintln(out, "\n== Structure ==")
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
	fmt.Fprintln(out, "\n== Styles ==")
	fmt.Fprintf(out, "Definitions: %d\n", len(styles))
	for _, style := range styles {
		fmt.Fprintf(out, "  %s\n", style)
	}
	fmt.Fprintf(out, "Fonts used: %s\n", joinOrNone(fonts))
	fmt.Fprintf(out, "Undefined style references: %d\n", undefined)

	dialogues, comments, earliest, latest, minLayer, maxLayer := eventSummary(doc)
	fmt.Fprintln(out, "\n== Events ==")
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

	fmt.Fprintln(out, "\n== Compliance ==")
	printSummary(out, result)
	return 0
}

func Check(path string, out, errOut io.Writer) int {
	_, result, err := load(path, "auto")
	if err != nil {
		fmt.Fprintf(errOut, "asst: %s\n", err)
		return 2
	}
	if len(result.Diagnostics) == 0 {
		fmt.Fprintln(out, "No diagnostics.")
		fmt.Fprintln(out)
	} else {
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintf(out, "%s:%d: %s[%s]: %s\n", path, diagnostic.Line, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
		}
		fmt.Fprintln(out)
	}
	printSummary(out, result)
	if result.ErrorCount() > 0 {
		return 1
	}
	return 0
}

func Normalize(path, matrixMode string, in io.Reader, out, errOut io.Writer) int {
	if canonical, ok := rules.NormalizeMatrixValue(matrixMode); ok {
		matrixMode = canonical
	}
	doc, result, err := load(path, matrixMode)
	if err != nil {
		fmt.Fprintf(errOut, "asst: %s\n", err)
		return 2
	}
	edits := append([]rules.Edit(nil), result.Edits...)
	edits = append(edits, sourceFormatEdits(doc)...)
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Start != edits[j].Start {
			return edits[i].Start < edits[j].Start
		}
		return edits[i].End > edits[j].End
	})
	edits = mergeNormalizationEdits(uniqueEdits(edits))
	candidate, err := doc.Source.Render(rules.ToReplacements(edits))
	if err != nil {
		fmt.Fprintf(errOut, "asst: cannot render normalization candidate: %s\n", err)
		return 2
	}
	_, candidateResult, err := checkBytes(candidate, matrixMode)
	if err != nil {
		fmt.Fprintf(errOut, "asst: normalization candidate is invalid: %s\n", err)
		return 2
	}
	fmt.Fprintln(out, "== Normalize preview ==")
	fmt.Fprintf(out, "Input: %q\n", path)
	fmt.Fprintf(out, "Matrix mode: %s\n", matrixMode)
	printMatrixDecision(out, doc, matrixMode)
	fmt.Fprintln(out, "\nChanges:")
	if len(edits) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for _, edit := range edits {
			printEdit(out, edit)
		}
	}
	fmt.Fprintln(out, "\nManual items:")
	manuals := manualDiagnostics(result)
	if len(manuals) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for _, diagnostic := range manuals {
			fmt.Fprintf(out, "  line %d [%s] %s\n", diagnostic.Line, diagnostic.Code, diagnostic.Message)
		}
	}

	if len(edits) == 0 {
		fmt.Fprintln(out, "\nNo changes required.")
		printSummary(out, candidateResult)
		if candidateResult.ErrorCount() > 0 || candidateResult.ManualCount() > 0 {
			return 1
		}
		return 0
	}
	backup := path + ".bak"
	if _, statErr := os.Stat(backup); statErr == nil {
		fmt.Fprintf(errOut, "asst: backup already exists: %s\n", backup)
		return 2
	} else if !os.IsNotExist(statErr) {
		fmt.Fprintf(errOut, "asst: cannot inspect backup path %s: %s\n", backup, statErr)
		return 2
	}
	fmt.Fprintf(out, "\nApply %d %s to %q?\n", len(edits), plural(len(edits), "change", "changes"), path)
	fmt.Fprintf(out, "Backup: %q [y/N] ", backup)
	reader := bufio.NewReader(in)
	answer, readErr := reader.ReadString('\n')
	if readErr != nil && len(answer) == 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Cancelled; no files changed.")
		return 0
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(out, "Cancelled; no files changed.")
		return 0
	}
	current, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errOut, "asst: cannot re-read input: %s\n", err)
		return 2
	}
	if !bytes.Equal(current, doc.Source.Original) {
		fmt.Fprintln(errOut, "asst: input changed while waiting for confirmation")
		return 2
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(errOut, "asst: cannot stat input: %s\n", err)
		return 2
	}
	if err := writeBackup(backup, current, info.Mode().Perm()); err != nil {
		fmt.Fprintf(errOut, "asst: cannot write backup %s: %s\n", backup, err)
		return 2
	}
	if err := replaceFile(path, candidate, info.Mode().Perm()); err != nil {
		fmt.Fprintf(errOut, "asst: cannot replace %s: %s\n", path, err)
		return 2
	}
	_, after, err := load(path, matrixMode)
	if err != nil {
		fmt.Fprintf(errOut, "asst: normalized file cannot be checked: %s\n", err)
		return 2
	}
	fmt.Fprintf(out, "Applied %d %s.\n", len(edits), plural(len(edits), "change", "changes"))
	fmt.Fprintf(out, "Backup written: %q\n", backup)
	fmt.Fprintf(out, "Recheck: %d errors, %d warnings, %d manual items\n", after.ErrorCount(), after.WarningCount(), after.ManualCount())
	if after.ErrorCount() == 0 && after.ManualCount() == 0 {
		fmt.Fprintln(out, "Status: normalized successfully")
		return 0
	}
	fmt.Fprintln(out, "Status: normalized with manual items")
	return 1
}

func load(path, matrixMode string) (*ass.Document, rules.Result, error) {
	if !strings.EqualFold(filepath.Ext(path), ".ass") {
		return nil, rules.Result{}, fmt.Errorf("input must have a .ass extension")
	}
	source, _, err := ass.Load(path)
	if err != nil {
		return nil, rules.Result{}, err
	}
	doc, err := ass.Parse(source)
	if err != nil {
		return nil, rules.Result{}, err
	}
	return doc, rules.Run(doc, matrixMode), nil
}

func checkBytes(data []byte, matrixMode string) (*ass.Document, rules.Result, error) {
	source, err := ass.ParseBytes(data)
	if err != nil {
		return nil, rules.Result{}, err
	}
	doc, err := ass.Parse(source)
	if err != nil {
		return nil, rules.Result{}, err
	}
	return doc, rules.Run(doc, matrixMode), nil
}

func sourceFormatEdits(doc *ass.Document) []rules.Edit {
	edits := make([]rules.Edit, 0)
	if !doc.Source.BOM {
		edits = append(edits, rules.Edit{Line: 1, Start: 0, End: 0, Replacement: []byte{0xef, 0xbb, 0xbf}, Code: "utf8-bom", Description: "add UTF-8 BOM", Before: "<missing>", After: "UTF-8 BOM", Safe: true})
	}
	if !doc.Source.Mixed {
		return edits
	}
	wantCRLF := doc.Source.DominantNewline == ass.NewlineCRLF
	for _, line := range doc.Source.Lines {
		if len(line.Terminator) == 0 {
			continue
		}
		isCRLF := bytes.Equal(line.Terminator, []byte("\r\n"))
		if isCRLF == wantCRLF {
			continue
		}
		start := line.End - len(line.Terminator)
		replacement := []byte("\n")
		if wantCRLF {
			replacement = []byte("\r\n")
		}
		edits = append(edits, rules.Edit{Line: line.Number, Start: start, End: line.End, Replacement: replacement, Code: "newline-style", Description: "use dominant newline style", Before: string(line.Terminator), After: string(replacement), Safe: true})
	}
	return edits
}

func printEdit(out io.Writer, edit rules.Edit) {
	if edit.Start == edit.End && edit.Before == "<missing>" {
		fmt.Fprintf(out, "  line %d  [%s] %s\n", edit.Line, edit.Code, edit.Description)
		fmt.Fprintf(out, "    before: %q\n    after:  %q\n", edit.Before, edit.After)
		return
	}
	if strings.HasPrefix(edit.Before, "lines ") && edit.After == "" {
		fmt.Fprintf(out, "  %s  [%s] %s\n", edit.Before, edit.Code, edit.Description)
		return
	}
	fmt.Fprintf(out, "  line %d  [%s] %s\n", edit.Line, edit.Code, edit.Description)
	fmt.Fprintf(out, "    before: %q\n    after:  %q\n", edit.Before, edit.After)
}

func printMatrixDecision(out io.Writer, doc *ass.Document, matrixMode string) {
	if strings.EqualFold(matrixMode, "auto") {
		property := doc.ScriptProperties()["ycbcr matrix"]
		if canonical, ok := rules.NormalizeMatrixValue(property.Value); ok {
			if candidate, _ := rules.InferMatrix(doc); candidate != nil {
				fmt.Fprintf(out, "Matrix decision: retain existing %s (valid; matches candidate %s)\n", canonical, candidate.Detail)
			} else {
				fmt.Fprintf(out, "Matrix decision: retain existing %s (valid)\n", canonical)
			}
		} else if candidate, _ := rules.InferMatrix(doc); candidate != nil {
			fmt.Fprintf(out, "Matrix decision: %s\n", candidate.Detail)
		} else {
			fmt.Fprintln(out, "Matrix decision: manual review required")
		}
		return
	}
	if canonical, ok := rules.NormalizeMatrixValue(matrixMode); ok {
		fmt.Fprintf(out, "Matrix decision: %s (explicit override)\n", canonical)
	}
}

func printSummary(out io.Writer, result rules.Result) {
	status := "compliant"
	if result.ErrorCount() > 0 {
		status = "errors found"
	} else if result.ManualCount() > 0 {
		status = "manual review required"
	} else if result.WarningCount() > 0 {
		status = "compliant with warnings"
	}
	fmt.Fprintf(out, "Summary: %d errors, %d warnings, %d manual items\n", result.ErrorCount(), result.WarningCount(), result.ManualCount())
	fmt.Fprintf(out, "Status: %s\n", status)
}

func manualDiagnostics(result rules.Result) []rules.Diagnostic {
	items := make([]rules.Diagnostic, 0)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Manual {
			items = append(items, diagnostic)
		}
	}
	return items
}

func writeBackup(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = file.Close()
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func replaceFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	base := filepath.Base(path)
	temp, err := os.CreateTemp(directory, "."+base+".asst-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err == nil {
		return nil
	}
	original, readErr := os.ReadFile(path)
	if readErr != nil {
		return readErr
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.WriteFile(path, original, mode)
		return err
	}
	return nil
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

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func uniqueEdits(edits []rules.Edit) []rules.Edit {
	result := make([]rules.Edit, 0, len(edits))
	seen := map[string]bool{}
	for _, edit := range edits {
		key := fmt.Sprintf("%d:%d:%x", edit.Start, edit.End, edit.Replacement)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, edit)
	}
	return result
}

func mergeNormalizationEdits(edits []rules.Edit) []rules.Edit {
	if len(edits) < 2 {
		return edits
	}
	ordered := append([]rules.Edit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start != ordered[j].Start {
			return ordered[i].Start < ordered[j].Start
		}
		return ordered[i].End > ordered[j].End
	})
	result := make([]rules.Edit, 0, len(ordered))
	for _, edit := range ordered {
		suppressed := false
		for _, kept := range result {
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
