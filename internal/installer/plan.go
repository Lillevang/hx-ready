// Package installer turns a recipe and a health report into a plan of
// commands, and runs that plan through an Executor.
//
// Plans are always shown before they run. Commands come only from bundled
// recipes, never from Helix output or user input.
package installer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Lillevang/hx-ready/internal/recipes"
)

// Step is one command in a plan.
type Step struct {
	// Provides lists the executables this step installs.
	Provides []string
	// Args is the argv to run. Args[0] is the program. Empty when Shell is set.
	Args []string
	// Shell, when non-empty, is run via "sh -c" instead of Args.
	Shell string
	// Env holds extra KEY=VALUE pairs, already expanded.
	Env []string
	// Privileged steps are prefixed with sudo and require confirmation.
	Privileged bool
}

// Plan is an ordered list of steps.
type Plan struct {
	Steps []Step
	// Uncovered lists missing executables that no step in the recipe
	// provides. They are reported, never silently dropped.
	Uncovered []string
}

// Empty reports whether the plan has nothing to run.
func (p *Plan) Empty() bool { return len(p.Steps) == 0 }

// Privileged reports whether any step needs sudo.
func (p *Plan) Privileged() bool {
	for _, s := range p.Steps {
		if s.Privileged {
			return true
		}
	}
	return false
}

// Run executes every step in order through ex, stopping at the first error.
func (p *Plan) Run(ex Executor) error {
	for _, s := range p.Steps {
		if err := ex.Run(s); err != nil {
			return err
		}
	}
	return nil
}

// Executor runs steps. DryRun prints; Exec runs for real.
type Executor interface {
	Run(step Step) error
}

// DryRun writes each step to Out and never executes anything. The caller
// prints the "Would run:" heading.
type DryRun struct {
	Out io.Writer
}

// Run implements Executor.
func (d DryRun) Run(step Step) error {
	_, err := fmt.Fprintf(d.Out, "\n%s\n", indent(step.String(), "  "))
	return err
}

// String renders a step the way a user would type it:
//
//	sudo dnf install -y golang gopls
//
//	GOBIN=/home/user/.local/bin \
//	  go install github.com/nametake/golangci-lint-langserver@latest
func (s Step) String() string {
	var cmd string
	if s.Shell != "" {
		cmd = s.Shell
	} else {
		quoted := make([]string, len(s.Args))
		for i, a := range s.Args {
			quoted[i] = shellQuote(a)
		}
		cmd = strings.Join(quoted, " ")
	}
	if s.Privileged {
		cmd = "sudo " + cmd
	}
	if len(s.Env) == 0 {
		return cmd
	}
	var b strings.Builder
	for _, kv := range s.Env {
		b.WriteString(kv)
		b.WriteString(" \\\n")
	}
	b.WriteString("  ")
	b.WriteString(cmd)
	return b.String()
}

// ErrNoPlatformBlock is returned when a recipe has no block for the
// platform. The error text names both.
var ErrNoPlatformBlock = errors.New("recipe has no block for this platform")

// Build makes the plan for a recipe on a platform given the executables
// currently missing. Packages are grouped into a single package-manager
// step (D-011); commands become one step each, ordered so that their Needs
// are satisfied. Steps that provide nothing from missing are left out,
// except that a package is kept when an included command needs one of its
// executables (installing an already-present package is a no-op).
func Build(p Platform, r *recipes.Recipe, missing []string) (*Plan, error) {
	block := p.Block(r)
	if block == nil {
		return nil, fmt.Errorf("%w: %s has no %s block", ErrNoPlatformBlock, r.Language, p.Name)
	}
	want := toSet(missing)

	// Commands that provide something missing, plus commands that provide
	// something those commands need, until nothing new is pulled in.
	// Executables the chosen commands rely on are also collected so the
	// packages providing them are kept.
	needed := map[string]bool{}
	chosen := make([]bool, len(block.Commands))
	for changed := true; changed; {
		changed = false
		for i, c := range block.Commands {
			if chosen[i] || !(anyIn(c.Provides, want) || anyIn(c.Provides, needed)) {
				continue
			}
			chosen[i] = true
			changed = true
			for _, n := range c.Needs {
				needed[n] = true
			}
		}
	}
	var cmds []recipes.Command
	for i, c := range block.Commands {
		if chosen[i] {
			cmds = append(cmds, c)
		}
	}

	// Packages that provide something missing or something needed.
	var pkgs []string
	covered := map[string]bool{}
	for _, pkg := range block.Packages {
		exes := pkg.Executables()
		if anyIn(exes, want) || anyIn(exes, needed) {
			pkgs = append(pkgs, pkg.Name)
			for _, e := range exes {
				covered[e] = true
			}
		}
	}

	plan := &Plan{}
	if len(pkgs) > 0 {
		plan.Steps = append(plan.Steps, Step{
			Provides:   coveredIn(want, covered),
			Args:       append(append([]string(nil), p.PackageArgs...), pkgs...),
			Privileged: true,
		})
	}

	ordered, err := orderCommands(cmds)
	if err != nil {
		return nil, fmt.Errorf("recipe %s: %w", r.Language, err)
	}
	for _, c := range ordered {
		plan.Steps = append(plan.Steps, commandStep(c))
		for _, e := range c.Provides {
			covered[e] = true
		}
	}

	for _, m := range missing {
		if !covered[m] {
			plan.Uncovered = append(plan.Uncovered, m)
		}
	}
	return plan, nil
}

func commandStep(c recipes.Command) Step {
	s := Step{
		Provides:   append([]string(nil), c.Provides...),
		Shell:      c.Shell,
		Privileged: c.Sudo,
	}
	if c.Shell == "" {
		s.Args = append([]string(nil), c.Args...)
	}
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// Values come from bundled recipes only (D-010); expansion is for
		// ${HOME} and friends, never user input.
		s.Env = append(s.Env, k+"="+os.ExpandEnv(c.Env[k]))
	}
	return s
}

// orderCommands sorts commands so that any command whose Needs are provided
// by another command runs after it. Recipe order is preserved otherwise.
func orderCommands(cmds []recipes.Command) ([]recipes.Command, error) {
	providedBy := map[string]int{}
	for i, c := range cmds {
		for _, p := range c.Provides {
			providedBy[p] = i
		}
	}
	const (
		unvisited = iota
		visiting
		done
	)
	state := make([]int, len(cmds))
	var out []recipes.Command
	var visit func(i int) error
	visit = func(i int) error {
		switch state[i] {
		case done:
			return nil
		case visiting:
			return errors.New("commands depend on each other in a cycle")
		}
		state[i] = visiting
		for _, n := range cmds[i].Needs {
			if j, ok := providedBy[n]; ok && j != i {
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		state[i] = done
		out = append(out, cmds[i])
		return nil
	}
	for i := range cmds {
		if err := visit(i); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func anyIn(xs []string, set map[string]bool) bool {
	for _, x := range xs {
		if set[x] {
			return true
		}
	}
	return false
}

// coveredIn returns the members of want that are in covered, sorted.
func coveredIn(want, covered map[string]bool) []string {
	var out []string
	for w := range want {
		if covered[w] {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// shellQuote single-quotes an argument when it contains anything the shell
// would interpret, for display only. Execution never goes through a shell
// unless the recipe says so.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:@,+", r))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
