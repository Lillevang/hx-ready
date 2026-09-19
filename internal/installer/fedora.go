// Package installer turns a recipe and a health report into a plan of
// commands, and runs that plan through an Executor.
//
// Plans are always shown before they run. Commands come only from bundled
// recipes, never from Helix output or user input.
package installer

import (
	"errors"
	"io"

	"github.com/Lillevang/hx-ready/internal/recipes"
)

// ErrNotImplemented marks scaffolded functions. See docs/TASKS.md.
var ErrNotImplemented = errors.New("not implemented yet")

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
}

// Executor runs steps. DryRun prints; Exec runs for real.
type Executor interface {
	Run(step Step) error
}

// DryRun writes each step to Out in the "Would run:" format and never
// executes anything.
type DryRun struct {
	Out io.Writer
}

// Run implements Executor.
func (d DryRun) Run(step Step) error {
	return ErrNotImplemented
}

// Fedora builds the plan for a recipe given the executables currently
// missing. Packages are grouped into a single "sudo dnf install" step;
// commands become one step each, ordered so that their Needs are satisfied.
// Steps that provide nothing from missing are left out.
func Fedora(r *recipes.Recipe, missing []string) (*Plan, error) {
	return nil, ErrNotImplemented
}
