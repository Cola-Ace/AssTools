package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
	"asstools/internal/terminal"
)

func runNormalize(args []string, in io.Reader, out, errOut io.Writer) int {
	matrix := "auto"
	path := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--matrix" {
			if i+1 >= len(args) {
				return usageForCommand(errOut, "normalize", "--matrix requires a value")
			}
			matrix = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--matrix=") {
			matrix = strings.TrimPrefix(arg, "--matrix=")
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return usageForCommand(errOut, "normalize", fmt.Sprintf("unknown option %q", arg))
		}
		if path != "" {
			return usageForCommand(errOut, "normalize", "normalize accepts one .ass file")
		}
		path = arg
	}
	if path == "" {
		return usageForCommand(errOut, "normalize", "normalize requires one .ass file")
	}
	if !strings.EqualFold(matrix, "auto") {
		if _, ok := canonicalMatrix(matrix); !ok {
			return usageForCommand(errOut, "normalize", fmt.Sprintf("invalid matrix value %q", matrix))
		}
	} else {
		matrix = "auto"
	}
	return commands.Normalize(path, matrix, in, out, errOut)
}

func canonicalMatrix(value string) (string, bool) {
	values := map[string]string{
		"none": "None", "tv.601": "TV.601", "tv.709": "TV.709", "tv.240m": "TV.240M", "tv.fcc": "TV.FCC",
		"pc.601": "PC.601", "pc.709": "PC.709", "pc.240m": "PC.240M", "pc.fcc": "PC.FCC",
	}
	canonical, ok := values[strings.ToLower(strings.TrimSpace(value))]
	return canonical, ok
}

func printNormalizeHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst normalize [--matrix <auto|value>] <input.ass>"))
	fmt.Fprintln(out, "Preview safe edits and apply them only after a y/yes confirmation.")
	fmt.Fprintln(out, "The default matrix mode is auto; explicit values use canonical spelling.")
}
