package ass

import (
	"strings"
	"unicode"
)

type OverrideBlock struct {
	Span Span
	Tags []OverrideTag
}

type OverrideTag struct {
	Name      string
	Arguments string
	Span      Span
	Known     bool
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
		known := knownOverride(strings.ToLower(name))
		tags = append(tags, OverrideTag{
			Name:      name,
			Arguments: body[argStart:i],
			Span:      Span{Start: absoluteStart + start, End: absoluteStart + i},
			Known:     known,
		})
	}
	return tags
}

func matchKnownName(value string) string {
	known := []string{"alpha", "blur", "move", "clip", "iclip", "fade", "pos", "org", "bord", "shad", "frx", "fry", "frz", "fscx", "fscy", "fsp", "fax", "fay", "pbo", "xbord", "ybord", "xshad", "yshad", "an", "fn", "fs", "be", "t", "q", "r", "a", "b", "i", "u", "s", "p", "k", "K", "kf", "ko", "kt", "c", "1c", "2c", "3c", "4c", "1a", "2a", "3a", "4a"}
	for _, name := range known {
		if len(value) >= len(name) && strings.EqualFold(value[:len(name)], name) {
			return value[:len(name)]
		}
	}
	return ""
}

func knownOverride(name string) bool {
	switch name {
	case "a", "an", "b", "be", "blur", "bord", "c", "1c", "2c", "3c", "4c", "1a", "2a", "3a", "4a", "alpha", "clip", "iclip", "fad", "fade", "fax", "fay", "fn", "fr", "frx", "fry", "frz", "fs", "fscx", "fscy", "fsp", "k", "K", "kf", "ko", "kt", "q", "r", "shad", "s", "t", "u", "pos", "move", "org", "p", "pbo", "xbord", "ybord", "xshad", "yshad":
		return true
	default:
		return false
	}
}
