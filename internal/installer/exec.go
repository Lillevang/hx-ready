package installer

import (
	"bufio"
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

// OSReleasePath is where the distribution identifies itself. A variable so
// tests can point it at a fixture.
var OSReleasePath = "/etc/os-release"

// RequireFedora returns nil on Fedora and an actionable error elsewhere
// (D-008). Only Fedora recipes exist, so running them anywhere else would
// at best fail and at worst do something surprising.
func RequireFedora() error {
	f, err := os.Open(OSReleasePath)
	if err != nil {
		return fmt.Errorf("could not read %s to confirm this is Fedora: %w\n\nhx-ready install only supports Fedora for now.", OSReleasePath, err)
	}
	defer f.Close()
	id := osReleaseID(f)
	if id == "fedora" {
		return nil
	}
	return fmt.Errorf("hx-ready install only supports Fedora for now.\n\n%s reports:\n  ID=%s\n\nExpected:\n  ID=fedora\n\n\"hx-ready check\" still works here; it only reads.", OSReleasePath, id)
}

// osReleaseID extracts the ID field from os-release content.
func osReleaseID(r io.Reader) string {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		v, ok := strings.CutPrefix(line, "ID=")
		if !ok {
			continue
		}
		return strings.Trim(v, `"'`)
	}
	return ""
}
