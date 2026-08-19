package cli

import (
	"fmt"
	"io"
	"strings"

	"asstools/internal/commands"
	"asstools/internal/rules"
	"asstools/internal/terminal"
)

func runNormalize(args []string, in io.Reader, out, errOut io.Writer) int {
	matrix := "auto"
	backup := false
	skipConfirmation := false
	jsonOutput := hasJSONFlag(args)
	outputPath := ""
	path := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--backup" {
			backup = true
			continue
		}
		if arg == "--yes" {
			skipConfirmation = true
			continue
		}
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--output" {
			if i+1 >= len(args) || args[i+1] == "" || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
				return usageForCommandOutput(out, errOut, "normalize", "--output requires a value", jsonOutput)
			}
			outputPath = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--output=") {
			outputPath = strings.TrimPrefix(arg, "--output=")
			if outputPath == "" {
				return usageForCommandOutput(out, errOut, "normalize", "--output requires a value", jsonOutput)
			}
			continue
		}
		if arg == "--matrix" {
			if i+1 >= len(args) {
				return usageForCommandOutput(out, errOut, "normalize", "--matrix requires a value", jsonOutput)
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
			return usageForCommandOutput(out, errOut, "normalize", fmt.Sprintf("unknown option %q", arg), jsonOutput)
		}
		if path != "" {
			return usageForCommandOutput(out, errOut, "normalize", "normalize accepts one .ass file", jsonOutput)
		}
		path = arg
	}
	if path == "" {
		return usageForCommandOutput(out, errOut, "normalize", "normalize requires one .ass file", jsonOutput)
	}
	canonical, ok := canonicalMatrix(matrix)
	if !ok {
		return usageForCommandOutput(out, errOut, "normalize", fmt.Sprintf("invalid matrix value %q", matrix), jsonOutput)
	}
	matrix = canonical
	if outputPath == "" {
		outputPath = path
	}
	return commands.NormalizeWithJSONOutput(path, outputPath, matrix, backup, skipConfirmation, jsonOutput, in, out, errOut)
}

func canonicalMatrix(value string) (string, bool) {
	return rules.NormalizeMatrixValue(value)
}

func printNormalizeHelp(out io.Writer) {
	fmt.Fprintln(out, terminal.Color(out, terminal.Bold, "Usage: asst normalize [--backup] [--output <path>] [--yes] [--json] [--matrix <auto|value>] <input.ass>"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Preview safe edits and apply them only after a y/yes confirmation; use --yes to skip confirmation. No backup is created by default.")
	fmt.Fprintln(out, "Use --output to write the normalized candidate to another path; without it the input is replaced.")
	fmt.Fprintln(out, "Use --backup to write a byte-identical <input.ass>.bak before replacing the original.")
	fmt.Fprintln(out, "Use --json for a single machine-readable JSON document; without --yes it returns a preview without prompting.")
	fmt.Fprintln(out, "The default matrix mode is auto; explicit values use canonical spelling.")
}
