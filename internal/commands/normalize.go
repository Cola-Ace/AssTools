package commands

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"asstools/internal/ass"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

var normalizeWriteMu sync.Mutex

var errNormalizeInputChanged = errors.New("input changed during normalization")

func Normalize(path, matrixMode string, in io.Reader, out, errOut io.Writer) int {
	return NormalizeWithOptions(path, matrixMode, false, false, in, out, errOut)
}

func NormalizeWithBackup(path, matrixMode string, in io.Reader, out, errOut io.Writer) int {
	return NormalizeWithOptions(path, matrixMode, true, false, in, out, errOut)
}

func NormalizeWithOptions(path, matrixMode string, backupEnabled, skipConfirmation bool, in io.Reader, out, errOut io.Writer) int {
	canonical, ok := rules.NormalizeMatrixValue(matrixMode)
	if !ok {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: invalid matrix value %q", matrixMode)))
		return 2
	}
	return normalize(path, canonical, backupEnabled, skipConfirmation, in, out, errOut)
}

func normalize(path, matrixMode string, backupEnabled, skipConfirmation bool, in io.Reader, out, errOut io.Writer) int {
	if canonical, ok := rules.NormalizeMatrixValue(matrixMode); ok {
		matrixMode = canonical
	}
	doc, result, err := load(path, matrixMode)
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: %s", err)))
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
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot render normalization candidate: %s", err)))
		return 2
	}
	_, candidateResult, err := checkBytes(candidate, matrixMode)
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: normalization candidate is invalid: %s", err)))
		return 2
	}
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold+terminal.Cyan, "== Normalize preview =="))
	fmt.Fprintf(out, "Input: %s\n", formatEditValue(path))
	fmt.Fprintf(out, "Matrix mode: %s\n", matrixMode)
	printMatrixDecision(out, doc, matrixMode)
	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold, "Changes:"))
	if len(edits) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		printEdits(out, edits)
	}
	fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Magenta, "Manual items:"))
	manuals := manualDiagnostics(result)
	if len(manuals) == 0 {
		fmt.Fprintln(out, "  none")
	} else {
		for _, diagnostic := range manuals {
			fmt.Fprintf(out, "  line %d [%s] %s\n", diagnostic.Line, terminal.Color(out, terminal.Magenta, diagnostic.Code), diagnostic.Message)
		}
	}

	if len(edits) == 0 {
		fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Green, "No changes required."))
		printSummary(out, candidateResult)
		if candidateResult.ErrorCount() > 0 || candidateResult.ManualCount() > 0 {
			return 1
		}
		return 0
	}
	backup := ""
	if backupEnabled {
		backup = path + ".bak"
		if _, statErr := os.Stat(backup); statErr == nil {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: backup already exists: %s", backup)))
			return 2
		} else if !os.IsNotExist(statErr) {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot inspect backup path %s: %s", backup, statErr)))
			return 2
		}
	}
	if !skipConfirmation {
		fmt.Fprintln(out, "\n"+terminal.Color(out, terminal.Bold+terminal.Yellow, fmt.Sprintf("Apply %d %s to %s?", len(edits), plural(len(edits), "change", "changes"), formatEditValue(path))))
		if backupEnabled {
			fmt.Fprintf(out, "%s ", terminal.Color(out, terminal.Cyan, fmt.Sprintf("Backup: %s [y/N]", formatEditValue(backup))))
		} else {
			fmt.Fprintf(out, "%s ", terminal.Color(out, terminal.Cyan, "Confirm [y/N]"))
		}
		reader := bufio.NewReader(in)
		answer, readErr := reader.ReadString('\n')
		if readErr != nil && len(answer) == 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, terminal.Color(out, terminal.Yellow, "Cancelled; no files changed."))
			return 0
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(out, terminal.Color(out, terminal.Yellow, "Cancelled; no files changed."))
			return 0
		}
	}
	normalizeWriteMu.Lock()
	defer normalizeWriteMu.Unlock()
	current, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot re-read input: %s", err)))
		return 2
	}
	if !bytes.Equal(current, doc.Source.Original) {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, "asst: input changed while waiting for confirmation"))
		return 2
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot stat input: %s", err)))
		return 2
	}
	replacement, err := newReplacementTransaction(path, current, candidate, info.Mode().Perm())
	if err != nil {
		if errors.Is(err, errNormalizeInputChanged) {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, "asst: input changed before replacement"))
		} else {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot prepare replacement for %s: %s", path, err)))
		}
		return 2
	}
	backupWritten := false
	if backupEnabled {
		if err := writeBackup(backup, current, info.Mode().Perm()); err != nil {
			cleanupErr := replacement.abort()
			if cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("cleanup failed: %w", cleanupErr))
			}
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot write backup %s: %s", backup, err)))
			return 2
		}
		backupWritten = true
	}
	if err := replacement.install(); err != nil {
		cleanupErr := replacement.abort()
		if backupWritten && !replacement.preservationFailed() {
			if removeErr := os.Remove(backup); removeErr != nil && !os.IsNotExist(removeErr) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove backup after failed replacement: %w", removeErr))
			}
		}
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup failed: %w", cleanupErr))
		}
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: cannot replace %s: %s", path, err)))
		return 2
	}
	_, after, err := load(path, matrixMode)
	if err == nil {
		applied, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
		} else if !bytes.Equal(applied, candidate) {
			err = errNormalizeInputChanged
		}
	}
	if err != nil {
		rollbackErr := replacement.rollback()
		if rollbackErr != nil {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: normalized file cannot be checked: %s; rollback failed: %s", err, rollbackErr)))
		} else {
			fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: normalized file cannot be checked: %s; original restored", err)))
		}
		return 2
	}
	if err := replacement.commit(); err != nil {
		fmt.Fprintln(errOut, terminal.Color(errOut, terminal.Red, fmt.Sprintf("asst: normalized file was applied but temporary cleanup failed: %s", err)))
		return 2
	}
	fmt.Fprintln(out, terminal.Color(out, terminal.Green, fmt.Sprintf("Applied %d %s.", len(edits), plural(len(edits), "change", "changes"))))
	if backupEnabled {
		fmt.Fprintln(out, terminal.Color(out, terminal.Green, fmt.Sprintf("Backup written: %s", formatEditValue(backup))))
	}
	recheckStyle := terminal.Green
	if after.ErrorCount() > 0 {
		recheckStyle = terminal.Red
	} else if after.ManualCount() > 0 {
		recheckStyle = terminal.Magenta
	} else if after.WarningCount() > 0 {
		recheckStyle = terminal.Yellow
	}
	fmt.Fprintln(out, terminal.Color(out, recheckStyle, fmt.Sprintf("Recheck: %d errors, %d warnings, %d manual items", after.ErrorCount(), after.WarningCount(), after.ManualCount())))
	if after.ErrorCount() == 0 && after.ManualCount() == 0 {
		fmt.Fprintln(out, terminal.Color(out, terminal.Green, "Status: normalized successfully"))
		return 0
	}
	fmt.Fprintln(out, terminal.Color(out, terminal.Magenta, "Status: normalized with manual items"))
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
	code := terminal.Color(out, terminal.Cyan, "["+edit.Code+"]")
	before, after := formatEditDiff(out, edit.Before, edit.After)
	if edit.Start == edit.End && edit.Before == "<missing>" {
		fmt.Fprintf(out, "  line %d  %s %s\n", edit.Line, code, edit.Description)
		fmt.Fprintf(out, "    before: %s\n    after:  %s\n", before, after)
		return
	}
	if strings.HasPrefix(edit.Before, "lines ") && edit.After == "" {
		fmt.Fprintf(out, "  %s  %s %s\n", edit.Before, code, edit.Description)
		return
	}
	fmt.Fprintf(out, "  line %d  %s %s\n", edit.Line, code, edit.Description)
	fmt.Fprintf(out, "    before: %s\n    after:  %s\n", before, after)
}

func printEdits(out io.Writer, edits []rules.Edit) {
	for index, edit := range edits {
		printEdit(out, edit)
		if index < len(edits)-1 {
			fmt.Fprintln(out)
		}
	}
}

func formatEditDiff(out io.Writer, beforeValue, afterValue string) (string, string) {
	before := formatEditValue(beforeValue)
	after := formatEditValue(afterValue)
	oldDiff, newDiff := editValueDiff(before, after)
	return colorEditValue(out, terminal.Red, terminal.Yellow, oldDiff), colorEditValue(out, terminal.Green, terminal.Cyan, newDiff)
}

type editValueParts struct {
	prefix  string
	changed string
	suffix  string
}

func editValueDiff(before, after string) (editValueParts, editValueParts) {
	beforeRunes := []rune(before)
	afterRunes := []rune(after)
	prefixLength := 0
	for prefixLength < len(beforeRunes) && prefixLength < len(afterRunes) && beforeRunes[prefixLength] == afterRunes[prefixLength] {
		prefixLength++
	}
	suffixLength := 0
	for prefixLength+suffixLength < len(beforeRunes) && prefixLength+suffixLength < len(afterRunes) && beforeRunes[len(beforeRunes)-1-suffixLength] == afterRunes[len(afterRunes)-1-suffixLength] {
		suffixLength++
	}
	return editValueParts{
			prefix:  string(beforeRunes[:prefixLength]),
			changed: string(beforeRunes[prefixLength : len(beforeRunes)-suffixLength]),
			suffix:  string(beforeRunes[len(beforeRunes)-suffixLength:]),
		}, editValueParts{
			prefix:  string(afterRunes[:prefixLength]),
			changed: string(afterRunes[prefixLength : len(afterRunes)-suffixLength]),
			suffix:  string(afterRunes[len(afterRunes)-suffixLength:]),
		}
}

func colorEditValue(out io.Writer, baseStyle, highlightStyle string, parts editValueParts) string {
	if !terminal.Enabled(out) {
		return parts.prefix + parts.changed + parts.suffix
	}
	if parts.changed == "" {
		return terminal.Color(out, baseStyle, parts.prefix+parts.suffix)
	}
	var value strings.Builder
	value.WriteString(baseStyle)
	value.WriteString(parts.prefix)
	value.WriteString(highlightStyle)
	value.WriteString(parts.changed)
	value.WriteString(terminal.Reset)
	if parts.suffix != "" {
		value.WriteString(baseStyle)
		value.WriteString(parts.suffix)
		value.WriteString(terminal.Reset)
	}
	return value.String()
}

func formatEditValue(value string) string {
	return strings.ReplaceAll(fmt.Sprintf("%q", value), `\\`, `\`)
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
	directory := filepath.Dir(path)
	base := filepath.Base(path)
	tempPath, err := writeTempFile(directory, "."+base+".asst-backup-*", data, mode)
	if err != nil {
		return err
	}
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := os.Link(tempPath, path); err != nil {
		return err
	}
	keepTemp = false
	_ = os.Remove(tempPath)
	return nil
}

func replaceFile(path string, data []byte, mode os.FileMode) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	replacement, err := newReplacementTransaction(path, original, data, mode)
	if err != nil {
		return err
	}
	if err := replacement.install(); err != nil {
		return errors.Join(err, replacement.abort())
	}
	return replacement.commit()
}

type replacementTransaction struct {
	path          string
	candidatePath string
	rollbackPath  string
	cleanupPath   string
	expected      []byte
	installed     bool
	keepRollback  bool
}

func newReplacementTransaction(path string, expected, data []byte, mode os.FileMode) (*replacementTransaction, error) {
	current, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(current, expected) {
		return nil, errNormalizeInputChanged
	}
	directory := filepath.Dir(path)
	base := filepath.Base(path)
	candidatePath, err := writeTempFile(directory, "."+base+".asst-candidate-*", data, mode)
	if err != nil {
		return nil, err
	}
	rollbackPath, err := writeTempFile(directory, "."+base+".asst-original-*", expected, mode)
	if err != nil {
		_ = os.Remove(candidatePath)
		return nil, err
	}
	latest, err := os.ReadFile(path)
	if err != nil {
		_ = os.Remove(candidatePath)
		_ = os.Remove(rollbackPath)
		return nil, err
	}
	if !bytes.Equal(latest, expected) {
		_ = os.Remove(candidatePath)
		_ = os.Remove(rollbackPath)
		return nil, errNormalizeInputChanged
	}
	return &replacementTransaction{
		path:          path,
		candidatePath: candidatePath,
		rollbackPath:  rollbackPath,
		expected:      append([]byte(nil), expected...),
	}, nil
}

func (replacement *replacementTransaction) install() error {
	current, err := os.ReadFile(replacement.path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, replacement.expected) {
		return errNormalizeInputChanged
	}
	renameErr := os.Rename(replacement.candidatePath, replacement.path)
	if renameErr == nil {
		replacement.candidatePath = ""
		replacement.installed = true
		return nil
	}
	displacedPath, err := newTempPath(filepath.Dir(replacement.path), "."+filepath.Base(replacement.path)+".asst-displaced-*")
	if err != nil {
		return errors.Join(renameErr, fmt.Errorf("prepare fallback replacement: %w", err))
	}
	if err := os.Rename(replacement.path, displacedPath); err != nil {
		_ = os.Remove(displacedPath)
		return errors.Join(renameErr, fmt.Errorf("preserve original before fallback replacement: %w", err))
	}
	if err := os.Rename(replacement.candidatePath, replacement.path); err != nil {
		replacement.keepRollback = true
		replacement.cleanupPath = displacedPath
		restoreErr := restoreFromTemp(displacedPath, replacement.path)
		if restoreErr == nil {
			replacement.keepRollback = false
			replacement.cleanupPath = ""
		}
		return errors.Join(renameErr, err, restoreErr)
	}
	replacement.candidatePath = ""
	replacement.installed = true
	replacement.cleanupPath = replacement.rollbackPath
	replacement.rollbackPath = displacedPath
	return nil
}

func (replacement *replacementTransaction) rollback() error {
	if !replacement.installed {
		return replacement.abort()
	}
	if err := restoreFromTemp(replacement.rollbackPath, replacement.path); err != nil {
		replacement.keepRollback = true
		return err
	}
	replacement.installed = false
	replacement.rollbackPath = ""
	return replacement.cleanup()
}

func (replacement *replacementTransaction) commit() error {
	if !replacement.installed {
		return fmt.Errorf("replacement is not installed")
	}
	replacement.installed = false
	return replacement.cleanup()
}

func (replacement *replacementTransaction) abort() error {
	if replacement.keepRollback {
		preservedPath := replacement.cleanupPath
		if preservedPath == "" {
			preservedPath = replacement.rollbackPath
		}
		return fmt.Errorf("original preserved at %s", preservedPath)
	}
	return replacement.cleanup()
}

func (replacement *replacementTransaction) cleanup() error {
	var cleanupErr error
	paths := []*string{&replacement.candidatePath, &replacement.rollbackPath, &replacement.cleanupPath}
	for _, path := range paths {
		if *path == "" {
			continue
		}
		if err := os.Remove(*path); err != nil && !os.IsNotExist(err) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary file %s: %w", *path, err))
			continue
		}
		*path = ""
	}
	return cleanupErr
}

func (replacement *replacementTransaction) preservationFailed() bool {
	return replacement.keepRollback
}

func writeTempFile(directory, pattern string, data []byte, mode os.FileMode) (string, error) {
	temp, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = temp.Close()
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(mode); err != nil {
		return "", err
	}
	if err := writeAndSync(temp, data); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	keepTemp = false
	return tempPath, nil
}

func writeAndSync(file *os.File, data []byte) error {
	written, err := file.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return file.Sync()
}

func newTempPath(directory, pattern string) (string, error) {
	temp, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Remove(tempPath); err != nil {
		return "", err
	}
	return tempPath, nil
}

func restoreFromTemp(sourcePath, destinationPath string) error {
	if err := os.Rename(sourcePath, destinationPath); err == nil {
		return nil
	} else {
		firstErr := err
		currentPath, tempErr := newTempPath(filepath.Dir(destinationPath), "."+filepath.Base(destinationPath)+".asst-current-*")
		if tempErr != nil {
			return errors.Join(firstErr, fmt.Errorf("prepare restoration: %w", tempErr))
		}
		if err := os.Rename(destinationPath, currentPath); err != nil {
			_ = os.Remove(currentPath)
			return errors.Join(firstErr, fmt.Errorf("preserve replaced file during restoration: %w", err))
		}
		if err := os.Rename(sourcePath, destinationPath); err != nil {
			restoreCurrentErr := os.Rename(currentPath, destinationPath)
			if restoreCurrentErr != nil {
				return errors.Join(firstErr, fmt.Errorf("restore original: %w", err), fmt.Errorf("restore replaced file: %w", restoreCurrentErr))
			}
			return errors.Join(firstErr, fmt.Errorf("restore original: %w", err))
		}
		if err := os.Remove(currentPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove replaced file after restoration: %w", err)
		}
		return nil
	}
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
