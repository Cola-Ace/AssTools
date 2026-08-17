package rules

import (
	"strconv"
	"strings"

	"asstools/internal/ass"
)

type overrideSyntaxRepair struct {
	message     string
	span        ass.Span
	replacement []byte
}

type overrideSyntaxCandidate struct {
	span        ass.Span
	replacement string
}

func overrideSyntaxRepairs(event ass.Event) []overrideSyntaxRepair {
	originalIssues := ass.ScanOverrideSyntax(event)
	if len(originalIssues) == 0 {
		return nil
	}
	baseline := issueCounts(originalIssues)
	seen := map[string]bool{}
	repairs := make([]overrideSyntaxRepair, 0)
	for _, candidate := range overrideSyntaxCandidates(event) {
		key := editCandidateKey(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		candidateText, ok := replaceEventSpan(event, candidate.span, candidate.replacement)
		if !ok {
			continue
		}
		candidateEvent := event
		candidateEvent.Text = candidateText
		remainingIssues := ass.ScanOverrideSyntax(candidateEvent)
		if len(remainingIssues) >= len(originalIssues) || hasNewIssue(baseline, remainingIssues) {
			continue
		}
		message := reducedIssue(baseline, remainingIssues, originalIssues)
		if message == "" {
			continue
		}
		repairs = append(repairs, overrideSyntaxRepair{message: message, span: candidate.span, replacement: []byte(candidate.replacement)})
	}
	return repairs
}

func overrideSyntaxCandidates(event ass.Event) []overrideSyntaxCandidate {
	text := event.Text
	candidates := make([]overrideSyntaxCandidate, 0)
	seen := map[string]bool{}
	add := func(span ass.Span, replacement string) {
		if span.Start < event.TextSpan.Start || span.End > event.TextSpan.End || span.End < span.Start {
			return
		}
		key := editCandidateKey(overrideSyntaxCandidate{span: span, replacement: replacement})
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, overrideSyntaxCandidate{span: span, replacement: replacement})
	}
	for _, block := range ass.ScanOverrides(event) {
		for _, tag := range block.Tags {
			start := tag.Span.Start - event.TextSpan.Start
			end := tag.Span.End - event.TextSpan.Start
			if start < 0 || end < start || end > len(text) || len(tag.Arguments) > end-start {
				continue
			}
			raw := text[start:end]
			prefix := raw[:len(raw)-len(tag.Arguments)]
			args := tag.Arguments
			trimmed := strings.TrimSpace(args)
			if len(trimmed) >= 2 && trimmed[0] == '(' && trimmed[len(trimmed)-1] == ')' {
				inner := trimmed[1 : len(trimmed)-1]
				add(tag.Span, prefix+preserveArgumentWhitespace(args, inner))
			}
			if withoutUnmatched, changed := removeUnmatchedClosingParentheses(trimmed); changed {
				add(tag.Span, prefix+preserveArgumentWhitespace(args, withoutUnmatched))
			}
			if missing := missingClosingParentheses(trimmed); missing > 0 {
				add(tag.Span, prefix+preserveArgumentWhitespace(args, trimmed+strings.Repeat(")", missing)))
			}
			if flattened, changed := flattenNestedArguments(trimmed); changed {
				add(tag.Span, prefix+preserveArgumentWhitespace(args, flattened))
			}
		}
	}
	return candidates
}

func issueCounts(issues []string) map[string]int {
	counts := make(map[string]int, len(issues))
	for _, issue := range issues {
		counts[issue]++
	}
	return counts
}

func hasNewIssue(baseline map[string]int, issues []string) bool {
	for issue := range issueCounts(issues) {
		if baseline[issue] == 0 {
			return true
		}
	}
	return false
}

func reducedIssue(baseline map[string]int, remaining []string, original []string) string {
	counts := issueCounts(remaining)
	for _, issue := range original {
		if counts[issue] < baseline[issue] {
			return issue
		}
	}
	return ""
}

func replaceEventSpan(event ass.Event, span ass.Span, replacement string) (string, bool) {
	start := span.Start - event.TextSpan.Start
	end := span.End - event.TextSpan.Start
	if start < 0 || end < start || end > len(event.Text) {
		return "", false
	}
	return event.Text[:start] + replacement + event.Text[end:], true
}

func editCandidateKey(candidate overrideSyntaxCandidate) string {
	return strings.Join([]string{strconv.Itoa(candidate.span.Start), strconv.Itoa(candidate.span.End), candidate.replacement}, ":")
}

func preserveArgumentWhitespace(original, replacement string) string {
	left := len(original) - len(strings.TrimLeft(original, " \t"))
	right := len(original) - len(strings.TrimRight(original, " \t"))
	if left+right > len(original) {
		return replacement
	}
	return original[:left] + replacement + original[len(original)-right:]
}

func removeUnmatchedClosingParentheses(value string) (string, bool) {
	depth := 0
	changed := false
	var builder strings.Builder
	for _, character := range value {
		switch character {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				changed = true
				continue
			}
			depth--
		}
		builder.WriteRune(character)
	}
	return builder.String(), changed
}

func missingClosingParentheses(value string) int {
	depth := 0
	for _, character := range value {
		switch character {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func flattenNestedArguments(value string) (string, bool) {
	if len(value) < 4 || value[0] != '(' || value[len(value)-1] != ')' {
		return "", false
	}
	depth := 0
	nestedStart := -1
	for index, character := range value {
		switch character {
		case '(':
			if depth > 0 && nestedStart < 0 {
				nestedStart = index
			}
			depth++
		case ')':
			if depth == 0 {
				return "", false
			}
			depth--
			if depth == 1 && nestedStart >= 0 {
				inner := value[nestedStart+1 : index]
				if inner != "" && !strings.ContainsAny(inner, "()") {
					return value[:nestedStart] + inner + value[index+1:], true
				}
				nestedStart = -1
			}
		}
	}
	return "", false
}
