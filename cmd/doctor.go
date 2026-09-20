package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Lillevang/hx-ready/internal/helix"
	"github.com/Lillevang/hx-ready/internal/recipes"
)

const doctorUsage = `Usage: hx-ready doctor [--all]

Summarises Helix language health: which languages are ready and which are
missing external tooling. By default only languages with an hx-ready recipe
or with some tooling already installed are shown.

Flags:
  --all   include every language Helix knows about
`

// doctorEntry is one language in the doctor report.
type doctorEntry struct {
	language string
	recipe   *recipes.Recipe // nil when none is bundled
	health   *helix.Health   // nil when the per-language check failed
	verdict  verdict
	err      error
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, doctorUsage) }
	all := fs.Bool("all", false, "include every language Helix knows about")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return ExitUsage
	}

	runner, err := newRunner()
	if err != nil {
		printHelixError(stderr, "", "", err)
		return ExitError
	}
	out, err := runner.Health("languages")
	if err != nil {
		printHelixError(stderr, "languages", out, err)
		return ExitError
	}
	rows, err := helix.ParseTable(out)
	if err != nil {
		printHelixError(stderr, "languages", out, err)
		return ExitError
	}

	known, err := recipes.Languages()
	if err != nil {
		fmt.Fprintf(stderr, "could not list bundled recipes: %v\n", err)
		return ExitError
	}
	byRecipe := map[string]*recipes.Recipe{}
	for _, l := range known {
		r, err := recipes.Load(l)
		if err != nil {
			fmt.Fprintf(stderr, "bundled recipe %s is broken: %v\n", l, err)
			return ExitError
		}
		byRecipe[l] = r
	}

	// D-006: a language is worth reporting when hx-ready can do something
	// about it, or the user has clearly started setting it up.
	var entries []doctorEntry
	for _, row := range rows {
		if !*all && byRecipe[row.Language] == nil && !row.HasInstalledTool() {
			continue
		}
		e := doctorEntry{language: row.Language, recipe: byRecipe[row.Language]}
		if strings.HasSuffix(row.Language, helix.Truncated) {
			e.err = errors.New("name truncated by Helix; run doctor in a wider terminal")
		} else if raw, err := runner.Health(row.Language); err != nil {
			e.err = err
		} else if h, err := helix.Parse(row.Language, raw); err != nil {
			e.err = err
		} else {
			e.health = h
			e.verdict = assess(h, e.recipe)
		}
		entries = append(entries, e)
	}

	return renderDoctor(newPrinter(stdout), entries)
}

// renderDoctor prints the grouped summary from AGENTS.md ("doctor command")
// and returns the exit code: 0 only when every listed language is ready.
func renderDoctor(p *printer, entries []doctorEntry) int {
	var ready, partial, failed []doctorEntry
	for _, e := range entries {
		switch {
		case e.err != nil:
			failed = append(failed, e)
		case e.verdict.ready:
			ready = append(ready, e)
		default:
			partial = append(partial, e)
		}
	}

	if len(entries) == 0 {
		p.line("Nothing to report: no language has an hx-ready recipe or any tooling installed.")
		p.line("Run \"hx-ready doctor --all\" to list every language Helix knows.")
		return ExitOK
	}

	first := true
	group := func(title string) {
		if !first {
			p.blank()
		}
		first = false
		p.line("%s", title)
		p.blank()
	}

	if len(ready) > 0 {
		group("Ready")
		for _, e := range ready {
			p.ok(e.language)
		}
	}
	if len(partial) > 0 {
		group("Partially configured")
		for _, e := range partial {
			p.warn(e.language)
			for _, t := range missingLines(e.health, e.verdict) {
				p.line("      %s", t)
			}
			if e.recipe != nil {
				p.line("      hx-ready install %s", e.language)
			}
		}
	}
	if len(failed) > 0 {
		group("Could not check")
		for _, e := range failed {
			p.missing(fmt.Sprintf("%s: %v", e.language, e.err))
		}
	}

	if len(partial) == 0 && len(failed) == 0 {
		return ExitOK
	}
	return ExitError
}

// missingLines lists what Helix wants and cannot find, using Helix's labels.
// A missing tool with no command name is reported as such (D-015); one the
// recipe does not require is marked rather than counted (D-017).
func missingLines(h *helix.Health, v verdict) []string {
	var out []string
	add := func(t helix.Tool, slot string) {
		if t.Status != helix.StatusMissing {
			return
		}
		switch {
		case t.Binary == "":
			out = append(out, slot+" has no command configured")
		case !v.covered(t):
			out = append(out, toolLabel(t)+" not covered by the recipe")
		default:
			out = append(out, "missing "+toolLabel(t))
		}
	}
	for _, ls := range h.LanguageServers {
		add(ls, "language server")
	}
	add(h.DebugAdapter, "debug adapter")
	add(h.Formatter, "formatter")
	return out
}
