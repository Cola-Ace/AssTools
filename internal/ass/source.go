package ass

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"
)

type NewlineKind string

const (
	NewlineLF   NewlineKind = "LF"
	NewlineCRLF NewlineKind = "CRLF"
)

type Line struct {
	Number     int
	Content    []byte
	Terminator []byte
	Start      int
	End        int
}

type Source struct {
	Original        []byte
	BOM             bool
	Lines           []Line
	DominantNewline NewlineKind
	Mixed           bool
	TrailingNewline bool
}

type Replacement struct {
	Start int
	End   int
	Bytes []byte
}

func Load(path string) (*Source, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("input is a directory: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	source, err := ParseBytes(data)
	if err != nil {
		return nil, nil, err
	}
	return source, info, nil
}

func ParseBytes(data []byte) (*Source, error) {
	s := &Source{Original: append([]byte(nil), data...), DominantNewline: NewlineLF}
	semantic := s.Original
	bomOffset := 0
	if len(semantic) >= 3 && bytes.Equal(semantic[:3], []byte{0xef, 0xbb, 0xbf}) {
		s.BOM = true
		semantic = semantic[3:]
		bomOffset = 3
	}
	if !utf8.Valid(semantic) {
		return nil, errors.New("input is not valid UTF-8")
	}

	lfCount, crlfCount := 0, 0
	firstNewline := NewlineLF
	lineStart := 0
	lineNumber := 1
	for i := 0; i < len(semantic); i++ {
		if semantic[i] != '\n' {
			continue
		}
		termStart := i
		if i > lineStart && semantic[i-1] == '\r' {
			termStart = i - 1
			crlfCount++
			if lfCount == 0 && crlfCount == 1 {
				firstNewline = NewlineCRLF
			}
		} else {
			lfCount++
			if lfCount == 1 && crlfCount == 0 {
				firstNewline = NewlineLF
			}
		}
		absoluteStart := lineStart + bomOffset
		contentEnd := termStart
		termEnd := i + 1
		s.Lines = append(s.Lines, Line{
			Number:     lineNumber,
			Content:    semantic[lineStart:contentEnd],
			Terminator: semantic[termStart:termEnd],
			Start:      absoluteStart,
			End:        termEnd + bomOffset,
		})
		lineStart = termEnd
		lineNumber++
	}
	if lineStart < len(semantic) || len(semantic) == 0 {
		s.Lines = append(s.Lines, Line{
			Number:     lineNumber,
			Content:    semantic[lineStart:],
			Terminator: nil,
			Start:      lineStart + bomOffset,
			End:        len(s.Original),
		})
	}
	if len(s.Lines) > 0 {
		last := s.Lines[len(s.Lines)-1]
		s.TrailingNewline = len(last.Terminator) > 0
	}
	if lfCount > 0 || crlfCount > 0 {
		if crlfCount > lfCount {
			s.DominantNewline = NewlineCRLF
		} else if crlfCount == lfCount {
			s.DominantNewline = firstNewline
		}
		s.Mixed = lfCount > 0 && crlfCount > 0
	}
	return s, nil
}

func (s *Source) Render(replacements []Replacement) ([]byte, error) {
	if len(replacements) == 0 {
		return append([]byte(nil), s.Original...), nil
	}
	ordered := append([]Replacement(nil), replacements...)
	for i := 0; i < len(ordered); i++ {
		if ordered[i].Start < 0 || ordered[i].End < ordered[i].Start || ordered[i].End > len(s.Original) {
			return nil, fmt.Errorf("invalid replacement range %d:%d", ordered[i].Start, ordered[i].End)
		}
		for j := i + 1; j < len(ordered); j++ {
			if ordered[j].Start < ordered[i].Start || (ordered[j].Start == ordered[i].Start && ordered[j].Start == ordered[j].End && ordered[i].Start != ordered[i].End) {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Start != ordered[i].End && ordered[i-1].Start != ordered[i-1].End && ordered[i].Start < ordered[i-1].End {
			return nil, fmt.Errorf("overlapping replacements at %d", ordered[i].Start)
		}
	}
	for _, insertion := range ordered {
		if insertion.Start != insertion.End {
			continue
		}
		for _, replacement := range ordered {
			if replacement.Start != replacement.End && insertion.Start > replacement.Start && insertion.Start < replacement.End {
				return nil, fmt.Errorf("insertion inside replacement at %d", insertion.Start)
			}
		}
	}
	var out bytes.Buffer
	previous := 0
	for _, replacement := range ordered {
		out.Write(s.Original[previous:replacement.Start])
		out.Write(replacement.Bytes)
		previous = replacement.End
	}
	out.Write(s.Original[previous:])
	return out.Bytes(), nil
}

func (s *Source) Line(number int) (Line, bool) {
	if number < 1 || number > len(s.Lines) {
		return Line{}, false
	}
	return s.Lines[number-1], true
}
