package ass

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

type OverrideBlock struct {
	Span Span
	Tags []OverrideTag
}

type OverrideTag struct {
	Name        string
	Arguments   string
	Span        Span
	Known       bool
	VSFilterMod bool
}

// ScanOverrideSyntax returns syntax errors found in an event's override blocks.
// It intentionally does not report unknown tag names; those are handled as a
// separate manual-review diagnostic by the rules engine.
func ScanOverrideSyntax(event Event) []string {
	text := event.Text
	issues := make([]string, 0)
	depth := 0
	blockStart := -1
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{':
			if depth == 0 {
				blockStart = i
			} else {
				issues = append(issues, "override block contains nested {")
			}
			depth++
		case '}':
			if depth == 0 {
				issues = append(issues, "closing } has no matching {")
				continue
			}
			depth--
			if depth == 0 {
				issues = append(issues, validateOverrideBody(text[blockStart+1:i])...)
			}
		}
	}
	if depth > 0 {
		issues = append(issues, "override block is missing closing }")
	}
	return issues
}

func validateOverrideBody(body string) []string {
	issues := make([]string, 0)
	for i := 0; i < len(body); {
		if body[i] != '\\' {
			i++
			continue
		}
		nameStart := i + 1
		i = nameStart
		if i >= len(body) || (!isOverrideTagLetter(body[i]) && !isOverrideTagDigit(body[i])) {
			issues = append(issues, "override tag is missing a name after \\")
			if i < len(body) {
				i++
			}
			continue
		}
		if matched := matchKnownName(body[i:]); matched != "" {
			i += len(matched)
		} else if isOverrideTagDigit(body[i]) {
			i++
			if i >= len(body) || !isOverrideTagLetter(body[i]) {
				issues = append(issues, "override tag name is invalid")
				for i < len(body) && body[i] != '\\' {
					i++
				}
				continue
			}
			i++
		} else {
			for i < len(body) && isOverrideTagLetter(body[i]) {
				i++
			}
		}
		name := body[nameStart:i]
		argStart := i
		parenDepth := 0
		unmatchedClose := false
		for i < len(body) {
			switch body[i] {
			case '(':
				parenDepth++
			case ')':
				if parenDepth == 0 {
					unmatchedClose = true
				} else {
					parenDepth--
				}
			case '\\':
				if parenDepth == 0 {
					goto tagDone
				}
			}
			i++
		}
	tagDone:
		args := body[argStart:i]
		if unmatchedClose || parenDepth != 0 {
			issues = append(issues, fmtOverrideParens(name))
			continue
		}
		if issue := invalidOverrideArguments(name, args); issue != "" {
			issues = append(issues, issue)
		}
	}
	return issues
}

func isOverrideTagLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isOverrideTagDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func fmtOverrideParens(name string) string {
	return "override tag \\" + name + " has unbalanced parentheses"
}

func invalidOverrideArguments(name, raw string) string {
	tag := strings.ToLower(name)
	args := strings.TrimSpace(raw)
	if requiresParentheses(tag) {
		inside, ok := parenthesizedArguments(args)
		if !ok {
			return "override tag \\" + name + " requires parenthesized arguments"
		}
		switch tag {
		case "pos", "org":
			if !validNumberList(inside, 2) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "move":
			if !validNumberList(inside, 4, 6) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "fad":
			if !validNumberList(inside, 2) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "fade":
			if !validNumberList(inside, 7) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "clip", "iclip":
			if strings.TrimSpace(inside) == "" {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "distort":
			if !validNumberList(inside, 6) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "jitter":
			if !validNumberList(inside, 5, 6) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "mover":
			if !validNumberList(inside, 8, 10) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "moves3":
			if !validNumberList(inside, 6, 8) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "moves4":
			if !validNumberList(inside, 8, 10) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "movevc":
			if !validNumberList(inside, 2, 4, 6) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "vc", "1vc", "2vc", "3vc", "4vc":
			if !validOverrideColorList(inside, false) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "va", "1va", "2va", "3va", "4va":
			if !validOverrideColorList(inside, true) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "img", "1img", "2img", "3img", "4img":
			if !validImageArguments(inside) {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "lua":
			if strings.TrimSpace(inside) == "" {
				return "override tag \\" + name + " has invalid arguments"
			}
		case "t":
			if strings.TrimSpace(inside) == "" {
				return "override tag \\" + name + " has invalid arguments"
			}
			if nested := validateOverrideBody(inside); len(nested) > 0 {
				return nested[0]
			}
		}
		return ""
	}
	switch tag {
	case "an":
		value, err := strconv.Atoi(args)
		if err != nil || value < 1 || value > 9 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "a":
		value, err := strconv.Atoi(args)
		if err != nil || value < 1 || value > 9 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "q":
		value, err := strconv.Atoi(args)
		if err != nil || value < 0 || value > 3 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "p":
		value, err := strconv.Atoi(args)
		if err != nil || value < 0 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "i", "u", "s":
		value, err := strconv.Atoi(args)
		if err != nil || value < 0 || value > 1 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "b", "be", "blur", "bord", "xbord", "ybord", "shad", "xshad", "yshad", "fr", "frx", "fry", "frz", "fsc", "fscx", "fscy", "fsp", "fax", "fay", "fe", "fs", "fsvp", "fshp", "frs", "xblur", "yblur", "z", "rnd", "rndx", "rndy", "rndz", "k", "kf", "ko", "kt":
		if !validNumber(args) {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "ortho":
		value, err := strconv.Atoi(args)
		if err != nil || value < 0 || value > 1 {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "blend":
		if !validBlendArguments(args) {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "c", "1c", "2c", "3c", "4c", "alpha", "1a", "2a", "3a", "4a":
		if args != "" && !validOverrideColor(args) {
			return "override tag \\" + name + " has invalid arguments"
		}
	case "fn":
		if args == "" {
			return "override tag \\" + name + " has invalid arguments"
		}
	}
	return ""
}

func requiresParentheses(tag string) bool {
	switch tag {
	case "pos", "move", "org", "clip", "iclip", "fad", "fade", "t", "distort", "jitter", "mover", "moves3", "moves4", "movevc", "vc", "va", "1vc", "2vc", "3vc", "4vc", "1va", "2va", "3va", "4va", "img", "1img", "2img", "3img", "4img", "lua":
		return true
	default:
		return false
	}
}

func parenthesizedArguments(value string) (string, bool) {
	if len(value) < 2 || value[0] != '(' || value[len(value)-1] != ')' {
		return "", false
	}
	return value[1 : len(value)-1], true
}

func validNumberList(value string, allowed ...int) bool {
	parts := strings.Split(value, ",")
	validCount := false
	for _, count := range allowed {
		if len(parts) == count {
			validCount = true
			break
		}
	}
	if !validCount {
		return false
	}
	for _, part := range parts {
		if !validNumber(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

func validNumber(value string) bool {
	if value == "" {
		return false
	}
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
}

func validOverrideColor(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 4 || !strings.EqualFold(trimmed[:2], "&H") || trimmed[len(trimmed)-1] != '&' {
		return false
	}
	digits := trimmed[2 : len(trimmed)-1]
	if len(digits) == 0 || len(digits) > 8 {
		return false
	}
	_, err := strconv.ParseUint(digits, 16, 32)
	return err == nil
}

func validOverrideColorList(value string, alpha bool) bool {
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		valid := validOverrideColor
		if alpha {
			valid = validOverrideAlpha
		}
		if !valid(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

func validOverrideAlpha(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 4 || !strings.EqualFold(trimmed[:2], "&H") || trimmed[len(trimmed)-1] != '&' {
		return false
	}
	digits := trimmed[2 : len(trimmed)-1]
	if len(digits) == 0 || len(digits) > 2 {
		return false
	}
	_, err := strconv.ParseUint(digits, 16, 8)
	return err == nil
}

func validImageArguments(value string) bool {
	parts := strings.Split(value, ",")
	if len(parts) < 1 || len(parts) > 3 || strings.TrimSpace(parts[0]) == "" {
		return false
	}
	for _, part := range parts[1:] {
		if !validNumber(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

func validBlendArguments(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if inside, ok := parenthesizedArguments(trimmed); ok {
		inside = strings.TrimSpace(inside)
		if blend, err := strconv.Atoi(inside); err == nil {
			return blend >= 0 && blend <= 6
		}
		allowed := map[string]bool{
			"normal": true, "over": true, "overlay": true, "add": true,
			"sub": true, "subtract": true, "substract": true,
			"mult": true, "multiply": true, "scr": true, "screen": true,
			"diff": true, "difference": true,
		}
		return allowed[strings.ToLower(inside)]
	}
	blend, err := strconv.Atoi(trimmed)
	return err == nil && blend >= 0 && blend <= 6
}

func ScanOverrides(event Event) []OverrideBlock {
	text := event.Text
	blocks := make([]OverrideBlock, 0)
	for i := 0; i < len(text); {
		if text[i] != '{' {
			i++
			continue
		}
		start := i
		depth := 1
		i++
		for i < len(text) && depth > 0 {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth != 0 {
			break
		}
		end := i
		bodyEnd := end - 1
		body := text[start+1 : bodyEnd]
		blocks = append(blocks, OverrideBlock{
			Span: Span{Start: event.TextSpan.Start + start, End: event.TextSpan.Start + end},
			Tags: scanTags(body, event.TextSpan.Start+start+1),
		})
	}
	return blocks
}

func scanTags(body string, absoluteStart int) []OverrideTag {
	tags := make([]OverrideTag, 0)
	for i := 0; i < len(body); {
		if body[i] != '\\' {
			i++
			continue
		}
		start := i
		i++
		nameStart := i
		if matched := matchKnownName(body[i:]); matched != "" {
			i += len(matched)
		} else if i < len(body) && unicode.IsDigit(rune(body[i])) {
			i++
			if i < len(body) && unicode.IsLetter(rune(body[i])) {
				i++
			}
		} else {
			for i < len(body) && unicode.IsLetter(rune(body[i])) {
				i++
			}
		}
		if i == nameStart {
			continue
		}
		name := body[nameStart:i]
		argStart := i
		depth := 0
		for i < len(body) {
			switch body[i] {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case '\\':
				if depth == 0 && i > argStart {
					goto done
				}
			}
			i++
		}
	done:
		lowerName := strings.ToLower(name)
		vsfiltermod := vsfiltermodOverride(lowerName)
		tags = append(tags, OverrideTag{
			Name:        name,
			Arguments:   body[argStart:i],
			Span:        Span{Start: absoluteStart + start, End: absoluteStart + i},
			Known:       knownOverride(lowerName),
			VSFilterMod: vsfiltermod,
		})
		if strings.EqualFold(name, "t") {
			rawArguments := body[argStart:i]
			if nested, ok := parenthesizedArguments(strings.TrimSpace(rawArguments)); ok {
				open := strings.Index(rawArguments, "(")
				if open < 0 {
					continue
				}
				for _, nestedTag := range scanTags(nested, absoluteStart+argStart+open+1) {
					if nestedTag.VSFilterMod {
						tags = append(tags, nestedTag)
					}
				}
			}
		}
	}
	return tags
}

func matchKnownName(value string) string {
	known := []string{"alpha", "blur", "move", "moves3", "moves4", "mover", "clip", "iclip", "fade", "fad", "pos", "org", "bord", "shad", "frx", "fry", "frz", "fr", "fsc", "fscx", "fscy", "fsp", "fax", "fay", "pbo", "xbord", "ybord", "xshad", "yshad", "an", "fn", "fs", "be", "fsvp", "fshp", "frs", "xblur", "yblur", "z", "rnd", "rndx", "rndy", "rndz", "distort", "jitter", "movevc", "ortho", "blend", "img", "1img", "2img", "3img", "4img", "vc", "va", "1vc", "2vc", "3vc", "4vc", "1va", "2va", "3va", "4va", "lua", "t", "q", "r", "a", "b", "i", "u", "s", "p", "k", "K", "kf", "ko", "kt", "c", "1c", "2c", "3c", "4c", "1a", "2a", "3a", "4a", "fe"}
	matched := ""
	for _, name := range known {
		if len(name) > len(matched) && len(value) >= len(name) && strings.EqualFold(value[:len(name)], name) {
			matched = value[:len(name)]
		}
	}
	return matched
}

func knownOverride(name string) bool {
	switch name {
	case "a", "an", "b", "be", "blur", "bord", "c", "1c", "2c", "3c", "4c", "1a", "2a", "3a", "4a", "alpha", "clip", "iclip", "fad", "fade", "fax", "fay", "fe", "fn", "fr", "frx", "fry", "frz", "fs", "fscx", "fscy", "fsp", "k", "K", "kf", "ko", "kt", "q", "r", "shad", "s", "t", "u", "pos", "move", "org", "p", "pbo", "xbord", "ybord", "xshad", "yshad":
		return true
	default:
		return false
	}
}

func vsfiltermodOverride(name string) bool {
	switch strings.ToLower(name) {
	case "fsc", "fsvp", "frs", "z", "distort", "rnd", "rndx", "rndy", "rndz", "mover", "moves3", "moves4", "jitter", "movevc", "xblur", "yblur", "fshp", "ortho", "blend", "img", "1img", "2img", "3img", "4img", "vc", "va", "1vc", "2vc", "3vc", "4vc", "1va", "2va", "3va", "4va", "lua":
		return true
	default:
		return false
	}
}
