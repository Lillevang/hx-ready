package installer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Exec runs steps for real, streaming their output. Privileged steps are
// run through sudo, which handles its own password prompt on the terminal
// (D-011); hx-ready never touches credentials.
type Exec struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// Run implements Executor. The step is echoed before it runs so the user
// always sees what is happening.
func (e Exec) Run(step Step) error {
	fmt.Fprintf(e.Stdout, "\n$ %s\n", strings.ReplaceAll(step.String(), "\n", "\n  "))

	var argv []string
	if step.Shell != "" {
		// D-010: the recipe asked for a shell explicitly.
		argv = []string{"sh", "-c", step.Shell}
	} else {
		argv = step.Args
	}
	if step.Privileged {
		argv = append([]string{"sudo"}, argv...)
	}
	if len(argv) == 0 {
		return errors.New("step has no command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), step.Env...)
	cmd.Stdout = e.Stdout
	cmd.Stderr = e.Stderr
	cmd.Stdin = e.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", argv[0], err)
	}
	return nil
}
