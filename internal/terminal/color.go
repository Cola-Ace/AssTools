package terminal

import (
	"io"
	"os"
	"strings"
)

const (
	Reset   = "\x1b[0m"
	Bold    = "\x1b[1m"
	Dim     = "\x1b[2m"
	Red     = "\x1b[31m"
	Yellow  = "\x1b[33m"
	Green   = "\x1b[32m"
	Cyan    = "\x1b[36m"
	Magenta = "\x1b[35m"
)

func Enabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func Color(w io.Writer, style, text string) string {
	if text == "" || !Enabled(w) {
		return text
	}
	return style + text + Reset
}
