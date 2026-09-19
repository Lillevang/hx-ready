package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lillevang/hx-ready/internal/helix"
)

// fakeRunner serves captured hx --health output from internal/helix/testdata.
type fakeRunner struct {
	fixture string // file to return for any language
	err     error  // returned from Health when set
}

func (f fakeRunner) Health(string) (string, error) {
	var out string
	if f.fixture != "" {
		b, err := os.ReadFile(filepath.Join("..", "internal", "helix", "testdata", f.fixture))
		if err != nil {
			return "", err
		}
		out = string(b)
	}
	return out, f.err
}

// useRunner swaps in a fake runner for the duration of a test.
func useRunner(t *testing.T, r helix.Runner, err error) {
	t.Helper()
	old := newRunner
	newRunner = func() (helix.Runner, error) { return r, err }
	t.Cleanup(func() { newRunner = old })
}

func TestCheck(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	tests := []struct {
		name       string
		runner     helix.Runner
		runnerErr  error
		language   string
		wantCode   int
		wantOut    []string // substrings of stdout
		wantErr    []string // substrings of stderr
		wantAbsent []string // must not appear in stdout
	}{
		{
			name:     "go missing everything",
			runner:   fakeRunner{fixture: "health-go-missing.txt"},
			language: "go",
			wantCode: ExitError,
			wantOut: []string{
				"Go\n",
				"Language servers\n  ✘ gopls\n  ✘ golangci-lint-lsp\n",
				"Debug adapter\n  ✘ dlv\n",
				"Formatter\n  ✓ none configured\n",
				"Suggested installation\n\n  sudo dnf install golang gopls delve golangci-lint\n",
				"  GOBIN=/home/tester/.local/bin go install github.com/nametake/golangci-lint-langserver@latest\n",
				"Verify\n\n  hx --health go\n",
			},
			wantAbsent: []string{"Ready.", "Editor support", "\x1b["},
		},
		{
			name:     "rust ready without a recipe",
			runner:   fakeRunner{fixture: "health-rust-ready.txt"},
			language: "rust",
			wantCode: ExitOK,
			wantOut: []string{
				"rust\n",
				"  ✓ rust-analyzer\n  ✓ lldb-dap\n  ✓ highlighting\n  ✓ textobjects\n  ✓ indentation\n",
				"Ready.\n",
			},
			wantAbsent: []string{"Suggested installation"},
		},
		{
			name:     "no recipe for a missing language",
			runner:   fakeRunner{fixture: "health-ocaml-no-recipe.txt"},
			language: "ocaml",
			wantCode: ExitError,
			wantOut: []string{
				"  ✘ ocamllsp\n",
				"Editor support\n  ✓ highlighting\n  ⚠ textobjects (no tree-sitter queries)\n",
				"No hx-ready installation recipe exists yet.",
				"hx --health ocaml",
			},
		},
		{
			name:     "debug adapter without a command is a warning",
			runner:   fakeRunner{fixture: "health-javascript-empty-debugger.txt"},
			language: "javascript",
			wantCode: ExitError,
			wantOut:  []string{"Debug adapter\n  ⚠ no command configured in Helix\n"},
		},
		{
			name:      "hx not on PATH",
			runnerErr: helix.ErrHelixNotFound,
			language:  "go",
			wantCode:  ExitError,
			wantErr:   []string{"Could not run Helix.", "Expected executable:\n  hx\n"},
		},
		{
			name:     "unknown language",
			runner:   fakeRunner{fixture: "health-unknown-language.txt"},
			language: "nosuchlang",
			wantCode: ExitError,
			wantErr:  []string{`Helix does not know language "nosuchlang".`, "Did you mean one of these?\n  nickel, nix", "hx --health languages"},
		},
		{
			name:     "unexpected output",
			runner:   fakeRunner{fixture: "health-garbage.txt"},
			language: "go",
			wantCode: ExitError,
			wantErr:  []string{`Could not understand the output of "hx --health go".`, "Helix printed:\n  this is not helix output"},
		},
		{
			name:     "health command fails",
			runner:   fakeRunner{err: errors.New("hx --health failed: exit status 1")},
			language: "go",
			wantCode: ExitError,
			wantErr:  []string{`Running "hx --health go" failed.`, "exit status 1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useRunner(t, tt.runner, tt.runnerErr)
			var stdout, stderr bytes.Buffer
			got := Run([]string{"check", tt.language}, &stdout, &stderr)
			if got != tt.wantCode {
				t.Errorf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, tt.wantCode, stdout.String(), stderr.String())
			}
			for _, w := range tt.wantOut {
				if !strings.Contains(stdout.String(), w) {
					t.Errorf("stdout lacks %q\nstdout:\n%s", w, stdout.String())
				}
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr lacks %q\nstderr:\n%s", w, stderr.String())
				}
			}
			for _, w := range tt.wantAbsent {
				if strings.Contains(stdout.String(), w) {
					t.Errorf("stdout should not contain %q\nstdout:\n%s", w, stdout.String())
				}
			}
		})
	}
}

// TestCheckGoMissingLayout pins the whole milestone-1 screen so layout
// changes are deliberate.
func TestCheckGoMissingLayout(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	useRunner(t, fakeRunner{fixture: "health-go-missing.txt"}, nil)
	var stdout, stderr bytes.Buffer
	Run([]string{"check", "go"}, &stdout, &stderr)
	want := `Go

Language servers
  ✘ gopls
  ✘ golangci-lint-lsp

Debug adapter
  ✘ dlv

Formatter
  ✓ none configured

Suggested installation

  sudo dnf install golang gopls delve golangci-lint
  GOBIN=/home/tester/.local/bin go install github.com/nametake/golangci-lint-langserver@latest

Verify

  hx --health go
`
	if stdout.String() != want {
		t.Errorf("layout changed\n got:\n%s\nwant:\n%s", stdout.String(), want)
	}
}
