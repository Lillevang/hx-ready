package cmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/Lillevang/hx-ready/internal/helix"
)

const checkUsage = `Usage: hx-ready check <language>

Runs "hx --health <language>", reports which tools are present or missing,
and prints the Fedora installation recipe if one exists.

This command is read-only. It never changes the system.
`

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, checkUsage) }
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return ExitUsage
	}
	language := fs.Arg(0)
	if err := helix.ValidateLanguageName(language); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitUsage
	}

	return notImplemented("check", stderr)
}
