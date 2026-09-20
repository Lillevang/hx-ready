package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Lillevang/hx-ready/internal/helix"
	"github.com/Lillevang/hx-ready/internal/installer"
	"github.com/Lillevang/hx-ready/internal/recipes"
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
	plat := platformFor(p)
	plan, code := planFor(stderr, plat, recipe, v)
	if code != ExitOK {
		return code
	}
	renderCheck(p, plat, h, recipe, v, plan)
	if v.ready {
		return ExitOK
	}
	return ExitError
}

// resolveLanguage follows recipe aliases (D-009), announcing the canonical
// name when one was followed, and returns the recipe if any.
func resolveLanguage(p *printer, stderr io.Writer, language string) (string, *recipes.Recipe) {
	canonical, recipe, err := recipes.Resolve(language)
	if err != nil && !errors.Is(err, recipes.ErrNoRecipe) {
		// A bundled recipe that fails to load is a packaging bug, not a
		// user error; say so and still show the health.
		fmt.Fprintf(stderr, "warning: %v\n", err)
		recipe = nil
	}
	if canonical != language {
		p.line("%q is Helix's %q language; checking %s.", language, canonical, canonical)
		p.blank()
	}
	return canonical, recipe
}

// onPath reports whether an executable is available, so plans skip
// toolchains the user already has. A variable so tests are machine
// independent.
var onPath = func(exe string) bool {
	_, err := exec.LookPath(exe)
	return err == nil
}

// platformFor picks the platform whose recipe block to use. check works
// anywhere Helix runs (D-008), so on an unrecognised system it falls back
// to Fedora and says so; install refuses instead.
func platformFor(p *printer) installer.Platform {
	plat, err := installer.Detect()
	if err != nil {
		p.line("This system is not recognised (%v); showing %s commands.", err, installer.Fedora.Display)
		p.blank()
		return installer.Fedora
	}
	return plat
}

// planFor builds the installation plan for what is missing. A nil recipe
// yields a nil plan; a recipe without a block for the platform yields an
// empty plan with everything uncovered, which the renderer explains.
func planFor(stderr io.Writer, plat installer.Platform, recipe *recipes.Recipe, v verdict) (*installer.Plan, int) {
	if recipe == nil {
		return nil, ExitOK
	}
	plan, err := installer.Build(plat, recipe, v.missing, onPath)
	if errors.Is(err, installer.ErrNoPlatformBlock) {
		return &installer.Plan{Uncovered: v.missing}, ExitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "The bundled recipe for %s is broken: %v\n", recipe.Language, err)
		return nil, ExitError
	}
	return plan, ExitOK
}

// renderCheck prints the health report in the layout from AGENTS.md
// ("Primary UX"), followed by the installation suggestion when the language
// is not ready.
//
// Readiness is the verdict's (D-017): the recipe's requires when there is
// one, otherwise everything Helix lists.
func renderCheck(p *printer, plat installer.Platform, h *helix.Health, recipe *recipes.Recipe, v verdict, plan *installer.Plan) {
	name := h.Language
	if recipe != nil {
		name = recipe.DisplayName
	}
	p.title(name)

	if v.ready {
		p.blank()
		for _, ls := range h.LanguageServers {
			if ls.Status == helix.StatusOK {
				p.ok(toolLabel(ls))
			} else {
				renderTool(p, ls, v)
			}
		}
		for _, t := range []helix.Tool{h.DebugAdapter, h.Formatter} {
			if t.Status == helix.StatusOK {
				p.ok(toolLabel(t))
			} else if t.Status == helix.StatusMissing {
				renderTool(p, t, v)
			}
		}
		renderQueries(p, h)
		p.blank()
		p.line("Ready.")
		return
	}

	p.section("Language servers")
	if len(h.LanguageServers) == 0 {
		p.ok("none configured")
	}
	for _, ls := range h.LanguageServers {
		renderTool(p, ls, v)
	}
	p.section("Debug adapter")
	renderTool(p, h.DebugAdapter, v)
	p.section("Formatter")
	renderTool(p, h.Formatter, v)
	if h.Parser != helix.StatusOK || h.Highlight != helix.StatusOK ||
		h.Textobjects != helix.StatusOK || h.Indent != helix.StatusOK {
		p.section("Editor support")
		renderQueries(p, h)
	}

	if recipe == nil {
		renderNoRecipe(p, plat, h)
	} else if plat.Block(recipe) == nil {
		p.blank()
		p.line("The %s recipe has no %s block yet.", recipe.DisplayName, plat.Display)
		renderSearchHints(p, plat, v.missing)
	} else {
		p.blank()
		renderPlan(p, "Suggested installation", plan)
	}

	renderPathHint(p, h)

	p.section("Verify")
	p.blank()
	p.indented("hx --health " + h.Language)
}

// userBinDirs lists where user-level installs land: ~/.local/bin (D-007)
// and rustup's ~/.cargo/bin (D-024). Each entry is the directory and the
// shell line that would add it.
func userBinDirs() [][2]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return [][2]string{
		{filepath.Join(home, ".local", "bin"), `export PATH="$HOME/.local/bin:$PATH"`},
		{filepath.Join(home, ".cargo", "bin"), `export PATH="$HOME/.cargo/bin:$PATH"`},
	}
}

// renderPathHint prints a hint when a missing tool already exists in a
// user bin directory: it was installed, Helix just cannot see it (D-007).
// Shell rc files are never edited.
func renderPathHint(p *printer, h *helix.Health) {
	first := true
	for _, d := range userBinDirs() {
		var found []string
		for _, bin := range h.Missing() {
			if fi, err := os.Stat(filepath.Join(d[0], bin)); err == nil && !fi.IsDir() {
				found = append(found, bin)
			}
		}
		if len(found) == 0 {
			continue
		}
		if first {
			p.section("PATH")
			first = false
		}
		p.blank()
		p.indented(fmt.Sprintf("%s exists in %s, but that directory is not on Helix's PATH.", strings.Join(found, ", "), d[0]))
		p.indented("Add it to your shell and restart Helix:")
		p.blank()
		p.indented(d[1])
	}
}

// renderPlan prints the steps under a heading, then anything the recipe
// cannot provide so nothing is hidden (Q-02).
func renderPlan(p *printer, heading string, plan *installer.Plan) {
	p.line("%s", heading)
	if plan.Empty() {
		p.blank()
		p.indented("nothing: the recipe covers none of the missing tools")
	} else {
		_ = plan.Run(installer.DryRun{Out: p.w})
	}
	if len(plan.Uncovered) > 0 {
		p.section("Not covered by the recipe")
		for _, u := range plan.Uncovered {
			p.warn(u)
		}
	}
}

// renderNoRecipe is the fallback for a language Helix knows but hx-ready
// has no recipe for: point at the package manager so the user can look for
// the tools by the names Helix wants.
func renderNoRecipe(p *printer, plat installer.Platform, h *helix.Health) {
	p.blank()
	p.line("No hx-ready installation recipe exists yet for %s.", h.Language)
	renderSearchHints(p, plat, h.Missing())
}

func renderSearchHints(p *printer, plat installer.Platform, missing []string) {
	if len(missing) == 0 {
		return
	}
	p.section("Try")
	p.blank()
	for _, bin := range missing {
		p.indented(plat.PackageSearch(bin))
	}
}

// renderTool prints one tool line. Slots Helix has not configured count as
// fine (D-005); a missing tool with no command name cannot be installed and
// is shown as a warning (D-015); a missing tool the recipe does not require
// is a warning too, not a failure (D-017).
func renderTool(p *printer, t helix.Tool, v verdict) {
	switch {
	case t.Status == helix.StatusOK:
		p.ok(toolLabel(t))
	case t.Status == helix.StatusNone:
		p.ok("none configured")
	case t.Status == helix.StatusMissing && t.Binary == "":
		p.warn("no command configured in Helix")
	case t.Status == helix.StatusMissing && !v.covered(t):
		p.warn(toolLabel(t) + " (not covered by the recipe)")
	case t.Status == helix.StatusMissing:
		p.missing(toolLabel(t))
	default:
		p.warn(toolLabel(t) + " (status unknown)")
	}
}

// renderQueries prints the tree-sitter rows. They are not installable, so a
// missing one is a warning rather than a failure.
func renderQueries(p *printer, h *helix.Health) {
	for _, q := range []struct {
		label  string
		status helix.Status
	}{
		{"highlighting", h.Highlight},
		{"textobjects", h.Textobjects},
		{"indentation", h.Indent},
	} {
		switch q.status {
		case helix.StatusOK:
			p.ok(q.label)
		case helix.StatusMissing:
			p.warn(q.label + " (no tree-sitter queries)")
		default:
			p.warn(q.label + " (status unknown)")
		}
	}
}

// toolLabel is the name shown for a tool: Helix's label for language servers
// (golangci-lint-lsp), the executable for debug adapters and formatters.
func toolLabel(t helix.Tool) string {
	if t.Name != "" {
		return t.Name
	}
	return t.Binary
}
