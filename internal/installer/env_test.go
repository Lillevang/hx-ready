package installer

import (
	"os/exec"
	"testing"
)

// Suggested commands must preserve exactly the environment Exec passes,
// including characters that a shell would otherwise expand or execute.
func TestStepEnvironmentShellRoundTrip(t *testing.T) {
	for _, value := range []string{"", "/home/Jane Doe/.local/bin", "it's a path", "$HOME/`echo expanded`/$(echo expanded)", "value; echo injected", "line one\nline two", "a=b"} {
		t.Run(value, func(t *testing.T) {
			s := Step{Args: []string{"sh", "-c", `printf '%s' "$HXREADY_VALUE"`}, Env: []string{"HXREADY_VALUE=" + value}}
			out, err := exec.Command("sh", "-c", s.String()).CombinedOutput()
			if err != nil {
				t.Fatalf("rendered command failed: %v: %s", err, out)
			}
			if string(out) != value {
				t.Fatalf("got %q, want %q; command: %s", out, value, s.String())
			}
		})
	}
}
