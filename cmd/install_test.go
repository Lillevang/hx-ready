package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lillevang/hx-ready/internal/helix"
	"github.com/Lillevang/hx-ready/internal/installer"
)

func TestInstallDryRun(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("exit = %d, want %d; stderr: %s", code, ExitOK, stderr.String())
	}
	want := `Would run:

  sudo dnf install -y golang gopls delve golangci-lint

  GOBIN=/home/tester/.local/bin \
    go install github.com/nametake/golangci-lint-langserver@latest
`
	if stdout.String() != want {
		t.Errorf("dry run\n got:\n%s\nwant:\n%s", stdout.String(), want)
	}
}

func TestInstallDryRunOnlyMissing(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	// gopls and golangci-lint-langserver present, dlv missing.
	useRunner(t, fakeRunner{fixture: "health-go-formatter-missing-synthetic.txt"}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr)
	// gofumpt is missing but the recipe cannot provide it: nothing to run.
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	for _, w := range []string{
		"Would run:\n\n  nothing: the recipe covers none of the missing tools\n",
		"Not covered by the recipe\n  ⚠ gofumpt\n",
	} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
		}
	}
}

func TestInstallReady(t *testing.T) {
	useRunner(t, fakeRunner{fixture: "health-go-formatter-found-synthetic.txt"}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "Ready.\nNothing to install.\n") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestInstallNoRecipe(t *testing.T) {
	useRunner(t, fakeRunner{fixture: "health-ocaml-no-recipe.txt"}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "ocaml", "--dry-run"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stdout.String(), "No hx-ready installation recipe exists yet for ocaml.") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestInstallHelixErrors(t *testing.T) {
	useRunner(t, nil, helix.ErrHelixNotFound)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "Could not run Helix.") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}

// sequenceRunner returns one fixture per Health call, in order, so a test
// can show "missing" before the install and "ready" after it.
type sequenceRunner struct {
	fixtures []string
	calls    int
}

func (s *sequenceRunner) Health(language string) (string, error) {
	i := s.calls
	if i >= len(s.fixtures) {
		i = len(s.fixtures) - 1
	}
	s.calls++
	return fakeRunner{fixture: s.fixtures[i]}.Health(language)
}

// recordingExecutor records steps instead of running them.
type recordingExecutor struct {
	steps []installer.Step
	fail  bool
}

func (r *recordingExecutor) Run(step installer.Step) error {
	r.steps = append(r.steps, step)
	if r.fail {
		return errors.New("boom")
	}
	return nil
}

// installHarness wires fakes for a real (non dry-run) install: Fedora
// os-release, a recording executor, scripted stdin.
func installHarness(t *testing.T, osID, input string, fail bool) *recordingExecutor {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("ID="+osID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPath := installer.OSReleasePath
	installer.OSReleasePath = path
	oldStdin := stdin
	stdin = strings.NewReader(input)
	rec := &recordingExecutor{fail: fail}
	oldExec := newExecutor
	newExecutor = func(io.Writer, io.Writer) installer.Executor { return rec }
	t.Cleanup(func() {
		installer.OSReleasePath = oldPath
		stdin = oldStdin
		newExecutor = oldExec
	})
	return rec
}

func TestInstallConfirmed(t *testing.T) {
	rec := installHarness(t, "fedora", "y\n", false)
	useRunner(t, &sequenceRunner{fixtures: []string{"health-go-missing.txt", "health-go-formatter-found-synthetic.txt"}}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "go"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, ExitOK, stdout.String(), stderr.String())
	}
	if len(rec.steps) != 2 || !rec.steps[0].Privileged || rec.steps[0].Args[0] != "dnf" || rec.steps[1].Args[0] != "go" {
		t.Errorf("steps run = %+v", rec.steps)
	}
	for _, w := range []string{"Will run:", "Proceed? [y/N] ", "Verifying with hx --health go", "Ready.\n"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
		}
	}
}

func TestInstallDeclined(t *testing.T) {
	for _, input := range []string{"n\n", "\n", ""} {
		rec := installHarness(t, "fedora", input, false)
		useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
		var stdout, stderr bytes.Buffer
		code := Run([]string{"install", "go"}, &stdout, &stderr)
		if code != ExitError {
			t.Errorf("input %q: exit = %d, want %d", input, code, ExitError)
		}
		if len(rec.steps) != 0 {
			t.Errorf("input %q: steps were run: %+v", input, rec.steps)
		}
		if !strings.Contains(stdout.String(), "Aborted. Nothing was installed.") {
			t.Errorf("input %q: stdout:\n%s", input, stdout.String())
		}
	}
}

func TestInstallYesSkipsPrompt(t *testing.T) {
	rec := installHarness(t, "fedora", "", false)
	useRunner(t, &sequenceRunner{fixtures: []string{"health-go-missing.txt", "health-go-formatter-found-synthetic.txt"}}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitOK {
		t.Errorf("exit = %d, want %d; stderr: %s", code, ExitOK, stderr.String())
	}
	if len(rec.steps) != 2 {
		t.Errorf("steps run = %d, want 2", len(rec.steps))
	}
	if strings.Contains(stdout.String(), "Proceed?") {
		t.Error("prompt shown despite --yes")
	}
}

func TestInstallStillMissingAfterwards(t *testing.T) {
	installHarness(t, "fedora", "", false)
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stdout.String(), "Verifying with hx --health go\n\nGo\n\nLanguage servers\n  ✘ gopls") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestInstallRefusesOffFedora(t *testing.T) {
	rec := installHarness(t, "ubuntu", "y\n", false)
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if len(rec.steps) != 0 {
		t.Errorf("steps were run: %+v", rec.steps)
	}
	if !strings.Contains(stderr.String(), "only supports Fedora") || !strings.Contains(stderr.String(), "ID=ubuntu") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}

func TestInstallStepFails(t *testing.T) {
	rec := installHarness(t, "fedora", "", true)
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if len(rec.steps) != 1 {
		t.Errorf("steps run = %d, want 1 (stop at first failure)", len(rec.steps))
	}
	if !strings.Contains(stderr.String(), "Installation stopped: boom") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}

// TestCheckPathHint covers D-007: the tool is in ~/.local/bin but Helix
// cannot see it.
func TestCheckPathHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "golangci-lint-langserver"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	Run([]string{"check", "go"}, &stdout, &stderr)
	want := "PATH\n\n  golangci-lint-langserver exists in " + bin + ", but that directory is not on Helix's PATH.\n"
	if !strings.Contains(stdout.String(), want) || !strings.Contains(stdout.String(), `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}
