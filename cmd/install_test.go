package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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

// A formatter the user configured but the recipe does not require is
// reported, not installed, and does not block readiness (D-017, D-020).
func TestInstallUncoveredFormatter(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	useRunner(t, fakeRunner{fixture: "health-go-formatter-missing-synthetic.txt"}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("exit = %d, want %d", code, ExitOK)
	}
	for _, w := range []string{"  ⚠ gofumpt (not covered by the recipe)\n", "Ready.\nNothing to install.\n"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
		}
	}
}

// D-017: install only plans the recipe's requires.
func TestInstallDryRunPython(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	useRunner(t, fakeRunner{fixture: "health-python-partial.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "python", "--dry-run"}, &stdout, &stderr); code != ExitOK {
		t.Errorf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stdout.String() != "Would run:\n\n  sudo dnf install -y python3-lsp-server\n" {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

// npm recipes install into ~/.local via npm_config_prefix (D-007, D-016)
// and pull in nodejs-npm through needs.
func TestInstallDryRunNpm(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	cases := map[string]string{
		"yaml":       "npm install -g yaml-language-server",
		"typescript": "npm install -g typescript-language-server typescript",
		"javascript": "npm install -g typescript-language-server typescript",
		"json":       "npm install -g vscode-langservers-extracted",
		"dockerfile": "npm install -g dockerfile-language-server-nodejs",
	}
	fixtures := map[string]string{
		"yaml":       "health-yaml-missing.txt",
		"typescript": "health-typescript-missing.txt",
		"javascript": "health-javascript-empty-debugger.txt",
		"json":       "health-json-missing.txt",
		"dockerfile": "health-dockerfile-missing.txt",
	}
	for lang, cmd := range cases {
		useRunner(t, fakeRunner{fixture: fixtures[lang]}, nil)
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"install", lang, "--dry-run"}, &stdout, &stderr); code != ExitOK {
			t.Errorf("%s: exit = %d, want 0; stderr: %s", lang, code, stderr.String())
		}
		want := "Would run:\n\n  sudo dnf install -y nodejs-npm\n\n  npm_config_prefix=/home/tester/.local \\\n    " + cmd + "\n"
		if stdout.String() != want {
			t.Errorf("%s: stdout:\n%s\nwant:\n%s", lang, stdout.String(), want)
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
	usePlatform(t, osID, "")
	oldStdin := stdin
	stdin = strings.NewReader(input)
	rec := &recordingExecutor{fail: fail}
	oldExec := newExecutor
	newExecutor = func(io.Writer, io.Writer) installer.Executor { return rec }
	t.Cleanup(func() {
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

func TestInstallRefusesUnsupportedPlatform(t *testing.T) {
	rec := installHarness(t, "arch", "y\n", false)
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if len(rec.steps) != 0 {
		t.Errorf("steps were run: %+v", rec.steps)
	}
	if !strings.Contains(stderr.String(), "only supports Fedora and Ubuntu/Debian") || !strings.Contains(stderr.String(), "ID=arch") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}

// D-019: on Ubuntu the debian block is used and packages go through apt.
func TestInstallUbuntu(t *testing.T) {
	rec := installHarness(t, "ubuntu", "", false)
	useRunner(t, &sequenceRunner{fixtures: []string{"health-go-missing.txt", "health-go-formatter-found-synthetic.txt"}}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--yes"}, &stdout, &stderr); code != ExitOK {
		t.Errorf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if len(rec.steps) != 3 || strings.Join(rec.steps[0].Args, " ") != "apt-get install -y golang-go gopls delve" ||
		!strings.Contains(strings.Join(rec.steps[1].Args, " "), "cmd/golangci-lint@latest") ||
		!strings.Contains(strings.Join(rec.steps[2].Args, " "), "golangci-lint-langserver@latest") {
		t.Errorf("steps = %+v", rec.steps)
	}
}

// A recipe without a block for the running platform is reported, not run.
func TestInstallNoPlatformBlock(t *testing.T) {
	installHarness(t, "ubuntu", "", false)
	useRecipes(t, fstest.MapFS{
		"go.yaml": {Data: []byte("language: go\ndisplay_name: Go\nrequires:\n  language_servers: [gopls]\nfedora:\n  packages:\n    - name: gopls\n")},
	})
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"install", "go", "--dry-run"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), "The Go recipe has no Ubuntu/Debian block yet") {
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
