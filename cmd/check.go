package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Lillevang/hx-ready/internal/helix"
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

	p := newPrinter(stdout)
	canonical, recipe, err := recipes.Resolve(language)
	if err != nil && !errors.Is(err, recipes.ErrNoRecipe) {
		// A bundled recipe that fails to load is a packaging bug, not a
		// user error; say so and still show the health.
		fmt.Fprintf(stderr, "warning: %v\n", err)
		recipe = nil
	}
	if canonical != language {
		// D-009: aliases are resolved before anything reaches hx.
		p.line("%q is Helix's %q language; checking %s.", language, canonical, canonical)
		p.blank()
		language = canonical
	}

	h, code := healthFor(language, stderr)
	if code != ExitOK {
		return code
	}

	renderCheck(p, h, recipe)
	if h.Ready() {
		return ExitOK
	}
	return ExitError
}

// renderCheck prints the health report in the layout from AGENTS.md
// ("Primary UX"), followed by the installation suggestion when the language
// is not ready.
//
// Readiness is what Helix lists (helix.Health.Ready). Whether a recipe's
// narrower "requires" should define readiness instead is open (Q-02).
func renderCheck(p *printer, h *helix.Health, recipe *recipes.Recipe) {
	name := h.Language
	if recipe != nil {
		name = recipe.DisplayName
	}
	p.title(name)

	if h.Ready() {
		p.blank()
		for _, ls := range h.LanguageServers {
			p.ok(toolLabel(ls))
		}
		if h.DebugAdapter.Status == helix.StatusOK {
			p.ok(toolLabel(h.DebugAdapter))
		}
		if h.Formatter.Status == helix.StatusOK {
			p.ok(toolLabel(h.Formatter))
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
		renderTool(p, ls)
	}
	p.section("Debug adapter")
	renderTool(p, h.DebugAdapter)
	p.section("Formatter")
	renderTool(p, h.Formatter)
	if h.Parser != helix.StatusOK || h.Highlight != helix.StatusOK ||
		h.Textobjects != helix.StatusOK || h.Indent != helix.StatusOK {
		p.section("Editor support")
		renderQueries(p, h)
	}

	if recipe == nil {
		renderNoRecipe(p, h)
	} else {
		p.section("Suggested installation")
		p.blank()
		for _, l := range suggestedCommands(recipe) {
			p.indented(l)
		}
	}

	p.section("Verify")
	p.blank()
	p.indented("hx --health " + h.Language)
}

// renderNoRecipe is the fallback for a language Helix knows but hx-ready
// has no recipe for: point at dnf so the user can look for the tools by
// the names Helix wants.
func renderNoRecipe(p *printer, h *helix.Health) {
	p.blank()
	p.line("No hx-ready installation recipe exists yet for %s.", h.Language)
	missing := h.Missing()
	if len(missing) == 0 {
		return
	}
	p.section("Try")
	p.blank()
	for _, bin := range missing {
		p.indented("dnf search " + bin)
	}
}

// renderTool prints one tool line. Slots Helix has not configured count as
// fine (D-005); a missing tool with no command name cannot be installed and
// is shown as a warning (D-015).
func renderTool(p *printer, t helix.Tool) {
	switch {
	case t.Status == helix.StatusOK:
		p.ok(toolLabel(t))
	case t.Status == helix.StatusNone:
		p.ok("none configured")
	case t.Status == helix.StatusMissing && t.Binary == "":
		p.warn("no command configured in Helix")
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

// suggestedCommands renders the whole recipe as shell lines. This is the
// read-only preview; the filtered, executable plan is the installer's job
// (docs/TASKS.md, T-04).
func suggestedCommands(r *recipes.Recipe) []string {
	var out []string
	if len(r.Fedora.Packages) > 0 {
		names := make([]string, 0, len(r.Fedora.Packages))
		for _, pkg := range r.Fedora.Packages {
			names = append(names, pkg.Name)
		}
		out = append(out, "sudo dnf install "+strings.Join(names, " "))
	}
	for _, c := range r.Fedora.Commands {
		out = append(out, renderCommand(c))
	}
	return out
}

// renderCommand shows a recipe command the way a user would type it.
func renderCommand(c recipes.Command) string {
	var parts []string
	if c.Sudo {
		parts = append(parts, "sudo")
	}
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+os.ExpandEnv(c.Env[k]))
	}
	if c.Shell != "" {
		parts = append(parts, c.Shell)
	} else {
		parts = append(parts, c.Args...)
	}
	return strings.Join(parts, " ")
}
