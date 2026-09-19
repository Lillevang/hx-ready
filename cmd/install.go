package cmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/Lillevang/hx-ready/internal/helix"
)

const installUsage = `Usage: hx-ready install <language> [--dry-run] [--yes]

Runs the same health check as "check", then installs only the missing
tooling using the bundled Fedora recipe. Every command is shown before it
runs, and privileged commands require confirmation.

Flags:
  --dry-run   print the commands that would run and exit
  --yes       do not ask for confirmation (for scripts)
`

func runInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, installUsage) }
	dryRun := fs.Bool("dry-run", false, "print the commands that would run and exit")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
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
	_, _ = *dryRun, *yes

	return notImplemented("install", stderr)
}
