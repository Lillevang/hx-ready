package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Lillevang/hx-ready/internal/helix"
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
