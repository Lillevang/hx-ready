// Package cmd contains the hx-ready command-line entry points.
//
// Dispatch is deliberately plain: a name-to-function table and the standard
// library flag package. Cobra can be introduced later if the command surface
// outgrows this (see docs/DECISIONS.md, D-002).
package cmd

import (
	"fmt"
	"io"
)

// Exit codes. Keep them stable; scripts may depend on them.
const (
	ExitOK    = 0
	ExitError = 1 // something went wrong, or the language is not ready
	ExitUsage = 2 // bad arguments
)

// Version is set at build time via -ldflags "-X github.com/Lillevang/hx-ready/cmd.Version=v1.2.3".
var Version = "dev"

const usage = `hx-ready prepares Helix Editor for a programming language.

Usage:
  hx-ready check <language>              show what is missing and how to install it
  hx-ready install <language> [flags]    install missing tooling (asks first)
  hx-ready doctor                        summarise Helix language health
  hx-ready version

Run "hx-ready <command> --help" for command flags.
`

type command struct {
	name string
	run  func(args []string, stdout, stderr io.Writer) int
}

var commands = []command{
	{"check", runCheck},
	{"install", runInstall},
	{"doctor", runDoctor},
	{"version", runVersion},
}

// Run executes hx-ready with the given arguments and returns an exit code.
// It never calls os.Exit, so tests can drive it directly.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	for _, c := range commands {
		if c.name == args[0] {
			return c.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "hx-ready: unknown command %q\n\n%s", args[0], usage)
	return ExitUsage
}

func runVersion(_ []string, stdout, _ io.Writer) int {
	fmt.Fprintf(stdout, "hx-ready %s\n", Version)
	return ExitOK
}

// notImplemented is the placeholder body for commands that have a task in
// docs/TASKS.md but no code yet.
func notImplemented(name string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "hx-ready %s: not implemented yet. See docs/TASKS.md.\n", name)
	return ExitError
}
