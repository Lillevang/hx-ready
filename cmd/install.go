package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Lillevang/hx-ready/internal/helix"
	"github.com/Lillevang/hx-ready/internal/installer"
)

const installUsage = `Usage: hx-ready install <language> [--dry-run] [--yes]

Runs the same health check as "check", then installs only the missing
tooling using the bundled Fedora recipe. Every command is shown before it
runs, and privileged commands require confirmation.

Flags:
  --dry-run   print the commands that would run and exit
  --yes       do not ask for confirmation (for scripts)
`

// stdin is where the confirmation prompt reads from. Tests replace it.
var stdin io.Reader = os.Stdin

// newExecutor builds the executor that runs a plan for real. Tests replace
// it so nothing is ever executed.
var newExecutor = func(stdout, stderr io.Writer) installer.Executor {
	return installer.Exec{Stdout: stdout, Stderr: stderr, Stdin: stdin}
}

func runInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, installUsage) }
	dryRun := fs.Bool("dry-run", false, "print the commands that would run and exit")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	if err := fs.Parse(flagsFirst(args)); err != nil {
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

	p := newPrinter(stdout)
	language, recipe := resolveLanguage(p, stderr, language)
	h, code := healthFor(language, stderr)
	if code != ExitOK {
		return code
	}
	v := assess(h, recipe)
	// D-008, D-019: only run recipes on a platform we know.
	plat, platErr := installer.Detect()
	if platErr != nil {
		plat = installer.Fedora
	}
	if v.ready {
		renderCheck(p, plat, h, recipe, v, nil)
		p.line("Nothing to install.")
		return ExitOK
	}
	if recipe == nil {
		renderCheck(p, plat, h, nil, v, nil)
		return ExitError
	}
	if platErr != nil {
		fmt.Fprintf(stderr, "hx-ready install only supports %s for now.\n\n%v\n\n\"hx-ready check\" still works here; it only reads.\n", supportedPlatforms(), platErr)
		return ExitError
	}
	plan, code := planFor(stderr, plat, recipe, v)
	if code != ExitOK {
		return code
	}
	if plat.Block(recipe) == nil {
		fmt.Fprintf(stderr, "The %s recipe has no %s block yet, so there is nothing hx-ready can run here.\n", recipe.DisplayName, plat.Display)
		return ExitError
	}

	if *dryRun {
		renderPlan(p, "Would run:", plan)
		if plan.Empty() {
			return ExitError
		}
		return ExitOK
	}

	renderPlan(p, "Will run:", plan)
	if plan.Empty() {
		return ExitError
	}
	if !*yes && !confirm(p, stdin) {
		p.line("Aborted. Nothing was installed.")
		return ExitError
	}

	if err := plan.Run(newExecutor(stdout, stderr)); err != nil {
		fmt.Fprintf(stderr, "\nInstallation stopped: %v\n\nSteps after the failed one were not run. Fix the problem and run\n  hx-ready install %s\nagain; steps that already succeeded are skipped.\n", err, language)
		return ExitError
	}

	// Let Helix judge the result (AGENTS.md: "Verify with Helix").
	p.blank()
	p.line("Verifying with hx --health %s", language)
	p.blank()
	after, code := healthFor(language, stderr)
	if code != ExitOK {
		return code
	}
	afterV := assess(after, recipe)
	afterPlan, code := planFor(stderr, plat, recipe, afterV)
	if code != ExitOK {
		return code
	}
	renderCheck(p, plat, after, recipe, afterV, afterPlan)
	if afterV.ready {
		return ExitOK
	}
	return ExitError
}

func supportedPlatforms() string {
	var names []string
	for _, p := range installer.Platforms {
		names = append(names, p.Display)
	}
	return strings.Join(names, " and ")
}

// confirm asks once (D-011) and returns true only on an explicit yes.
func confirm(p *printer, in io.Reader) bool {
	p.blank()
	fmt.Fprint(p.w, "Proceed? [y/N] ")
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
