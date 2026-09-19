package installer

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestRequireFedora(t *testing.T) {
	cases := map[string]struct {
		content string
		wantOK  bool
	}{
		"fedora":        {"NAME=\"Fedora Linux\"\nVERSION=\"43 (Workstation Edition)\"\nID=fedora\nID_LIKE=\n", true},
		"fedora quoted": {"ID=\"fedora\"\n", true},
		"ubuntu":        {"NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", false},
		"empty":         {"", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "os-release")
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
			old := OSReleasePath
			OSReleasePath = path
			t.Cleanup(func() { OSReleasePath = old })
			err := RequireFedora()
			if (err == nil) != c.wantOK {
				t.Errorf("RequireFedora() = %v, wantOK %v", err, c.wantOK)
			}
			if err != nil && !strings.Contains(err.Error(), "ID=fedora") {
				t.Errorf("error should name the expected value: %v", err)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		old := OSReleasePath
		OSReleasePath = filepath.Join(t.TempDir(), "nope")
		t.Cleanup(func() { OSReleasePath = old })
		if err := RequireFedora(); err == nil {
			t.Error("expected error for a missing os-release")
		}
	})
}
