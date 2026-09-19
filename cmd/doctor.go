package cmd

import (
	"flag"
	"fmt"
	"io"
)

const doctorUsage = `Usage: hx-ready doctor [--all]

Summarises Helix language health: which languages are ready and which are
missing external tooling. By default only languages with an hx-ready recipe
or with some tooling already installed are shown.

Flags:
  --all   include every language Helix knows about
`

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, doctorUsage) }
	all := fs.Bool("all", false, "include every language Helix knows about")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return ExitUsage
	}
	_ = *all

	return notImplemented("doctor", stderr)
}
