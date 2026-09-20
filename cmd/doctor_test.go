package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// mapRunner serves a fixture per language; unknown languages fail unless
// the "*" key names a fallback fixture.
type mapRunner map[string]string

func (m mapRunner) Health(language string) (string, error) {
	f, ok := m[language]
	if !ok {
		f, ok = m["*"]
	}
	if !ok {
		return "", errors.New("no fixture for " + language)
	}
	return fakeRunner{fixture: f}.Health(language)
}

// doctorRunner covers every candidate the captured table produces with the
// bundled recipes (bash, go, hcl, rust) plus languages with a ✓ tool.
func doctorRunner() mapRunner {
	return mapRunner{
		"languages":  "health-all-languages-table.txt",
		"bash":       "health-bash-ready.txt",
		"go":         "health-go-missing.txt",
		"hcl":        "health-hcl-ready.txt",
		"rust":       "health-rust-ready.txt",
		"python":     "health-python-partial.txt",
		"javascript": "health-javascript-empty-debugger.txt",
		"typescript": "health-typescript-missing.txt",
		"yaml":       "health-yaml-missing.txt",
		"json":       "health-json-missing.txt",
		"dockerfile": "health-dockerfile-missing.txt",
		"astro":      "health-astro-missing.txt",
	}
}

func TestDoctor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	useRunner(t, doctorRunner(), nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (go is not ready)", code, ExitError)
	}
	for _, w := range []string{
		"Ready\n\n  ✓ bash\n  ✓ hcl\n  ✓ rust\n",
		"Partially configured\n\n  ⚠ astro\n      missing astro-ls\n      hx-ready install astro\n  ⚠ dockerfile\n      missing docker-langserver\n      hx-ready install dockerfile\n",
		"  ⚠ go\n      missing gopls\n      missing golangci-lint-lsp\n      missing dlv\n      hx-ready install go\n",
		"  ⚠ python\n      ty not covered by the recipe\n      jedi not covered by the recipe\n      missing pylsp\n      hx-ready install python\n",
		"  ⚠ javascript\n      missing typescript-language-server\n      debug adapter has no command configured\n      hx-ready install javascript\n",
		"  ⚠ yaml\n      missing yaml-language-server\n      hx-ready install yaml\n",
		"Could not check\n\n  ✘ c: no fixture for c\n",
	} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
		}
	}
	// ada has nothing installed and no recipe, so it is filtered out.
	for _, w := range []string{"ada:", "✓ ada", "\x1b["} {
		if strings.Contains(stdout.String(), w) {
			t.Errorf("stdout should not contain %q\nstdout:\n%s", w, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestDoctorAll(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	useRunner(t, doctorRunner(), nil)
	var stdout, stderr bytes.Buffer
	Run([]string{"doctor", "--all"}, &stdout, &stderr)
	for _, w := range []string{"  ✘ ada: no fixture for ada\n", "  ⚠ javascript\n      missing typescript-language-server\n      debug adapter has no command configured\n"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
		}
	}
	if !strings.Contains(stdout.String(), "  ✘ zig: no fixture for zig\n") {
		t.Errorf("--all should include every table row\nstdout:\n%s", stdout.String())
	}
}

func TestDoctorAllReady(t *testing.T) {
	useRunner(t, mapRunner{
		"languages": "health-all-languages-table.txt",
		"go":        "health-go-formatter-found-synthetic.txt",
		"*":         "health-rust-ready.txt",
	}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("exit = %d, want 0\nstdout:\n%s", code, stdout.String())
	}
	if strings.Contains(stdout.String(), "Partially") || strings.Contains(stdout.String(), "Could not") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

// TestDoctorNarrowTable: a truncated language name cannot be checked and is
// reported rather than passed to hx.
func TestDoctorNarrowTable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	useRunner(t, mapRunner{
		"languages": "health-all-languages-table-narrow.txt",
		"*":         "health-rust-ready.txt",
	}, nil)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--all"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stdout.String(), "  ✘ dockerfil…: name truncated by Helix; run doctor in a wider terminal\n") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "  ✓ go\n") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestDoctorErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	useRunner(t, mapRunner{"languages": "health-garbage.txt"}, nil)
	if code := Run([]string{"doctor"}, &stdout, &stderr); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr.String(), `Could not understand the output of "hx --health languages".`) {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}
