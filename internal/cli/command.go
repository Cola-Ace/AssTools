package cli

import (
	"io"
	"strings"
)

type commandHandler func(args []string, in io.Reader, out, errOut io.Writer) int

type commandSpec struct {
	name    string
	usage   string
	summary string
	handler commandHandler
	help    func(io.Writer)
}

func commandRegistry() []commandSpec {
	return []commandSpec{
		{
			name:    "info",
			usage:   "asst info <input.ass>",
			summary: "summarize structure, timing, fonts, and compliance",
			handler: runInfo,
			help:    printInfoHelp,
		},
		{
			name:    "check",
			usage:   "asst check [--ignore-vsfiltermod] <input.ass>",
			summary: "print stable compliance diagnostics",
			handler: runCheck,
			help:    printCheckHelp,
		},
		{
			name:    "normalize",
			usage:   "asst normalize [--backup] [--yes] [--matrix <auto|value>] <input.ass>",
			summary: "preview safe edits, then optionally apply them",
			handler: runNormalize,
			help:    printNormalizeHelp,
		},
	}
}

func commandFor(name string) (commandHandler, bool) {
	command, ok := commandNamed(name)
	if ok {
		return command.handler, true
	}
	return nil, false
}

func commandNamed(name string) (commandSpec, bool) {
	for _, command := range commandRegistry() {
		if strings.EqualFold(command.name, name) {
			return command, true
		}
	}
	return commandSpec{}, false
}
