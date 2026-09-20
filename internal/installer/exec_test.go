package installer

import (
	"bytes"
	"strings"
	"testing"
)

// TestExecRunsHarmlessCommands exercises the real executor with commands that
// change nothing: echo via argv, echo via the shell escape hatch, env
// passing, and a failing command. Nothing privileged is ever run in tests.
func TestExecRunsHarmlessCommands(t *testing.T) {
	var out, errOut bytes.Buffer
	e := Exec{Stdout: &out, Stderr: &errOut, Stdin: strings.NewReader("")}

	if err := e.Run(Step{Args: []string{"echo", "hello"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "$ echo hello\nhello\n") {
		t.Errorf("argv step output = %q", out.String())
	}

	out.Reset()
	if err := e.Run(Step{Shell: "echo $HXREADY_TEST", Env: []string{"HXREADY_TEST=via-env"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "via-env\n") {
		t.Errorf("shell step output = %q", out.String())
	}

	if err := e.Run(Step{Args: []string{"sh", "-c", "exit 3"}}); err == nil {
		t.Error("expected failure for exit 3")
	}
	if err := e.Run(Step{}); err == nil {
		t.Error("expected failure for empty step")
	}
}
