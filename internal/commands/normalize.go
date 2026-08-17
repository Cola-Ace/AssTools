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
