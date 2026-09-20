package helix

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ok(name, binary, path string) Tool {
	return Tool{Name: name, Binary: binary, Path: path, Status: StatusOK}
}

func missing(name, binary string) Tool {
	return Tool{Name: name, Binary: binary, Status: StatusMissing}
}

var none = Tool{Status: StatusNone}

// TestParseFixtures is the contract for the parser: one entry per captured
// or synthetic "hx --health <language>" report in testdata/.
func TestParseFixtures(t *testing.T) {
	tests := []struct {
		fixture     string
		language    string
		want        Health
		wantMissing []string
		wantReady   bool
	}{
		{
			fixture:  "health-go-missing.txt",
			language: "go",
			want: Health{
				Language: "go",
				LanguageServers: []Tool{
					missing("gopls", "gopls"),
					missing("golangci-lint-lsp", "golangci-lint-langserver"),
				},
				DebugAdapter: missing("", "dlv"),
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"gopls", "golangci-lint-langserver", "dlv"},
		},
		{
			// Same report with the colour codes already stripped.
			fixture:  "health-go-missing-plain.txt",
			language: "go",
			want: Health{
				Language: "go",
				LanguageServers: []Tool{
					missing("gopls", "gopls"),
					missing("golangci-lint-lsp", "golangci-lint-langserver"),
				},
				DebugAdapter: missing("", "dlv"),
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"gopls", "golangci-lint-langserver", "dlv"},
		},
		{
			fixture:  "health-python-partial.txt",
			language: "python",
			want: Health{
				Language: "python",
				LanguageServers: []Tool{
					missing("ty", "ty"),
					ok("ruff", "ruff", "/usr/sbin/ruff"),
					missing("jedi", "jedi-language-server"),
					missing("pylsp", "pylsp"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"ty", "jedi-language-server", "pylsp"},
		},
		{
			fixture:  "health-rust-ready.txt",
			language: "rust",
			want: Health{
				Language: "rust",
				LanguageServers: []Tool{
					ok("rust-analyzer", "rust-analyzer", "/home/jls/.cargo/bin/rust-analyzer"),
				},
				DebugAdapter: ok("", "lldb-dap", "/usr/sbin/lldb-dap"),
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantReady: true,
		},
		{
			fixture:  "health-bash-ready.txt",
			language: "bash",
			want: Health{
				Language: "bash",
				LanguageServers: []Tool{
					ok("bash-language-server", "bash-language-server", "/usr/sbin/bash-language-server"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantReady: true,
		},
		{
			fixture:  "health-yaml-missing.txt",
			language: "yaml",
			want: Health{
				Language: "yaml",
				LanguageServers: []Tool{
					missing("yaml-language-server", "yaml-language-server"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"yaml-language-server"},
		},
		{
			fixture:  "health-hcl-ready.txt",
			language: "hcl",
			want: Health{
				Language: "hcl",
				LanguageServers: []Tool{
					ok("terraform-ls", "terraform-ls", "/usr/sbin/terraform-ls"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantReady: true,
		},
		{
			// A ✘ in the tree-sitter rows must not affect readiness; it is
			// not something hx-ready can install.
			fixture:  "health-ocaml-no-recipe.txt",
			language: "ocaml",
			want: Health{
				Language: "ocaml",
				LanguageServers: []Tool{
					missing("ocamllsp", "ocamllsp"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusMissing, Indent: StatusOK,
			},
			wantMissing: []string{"ocamllsp"},
		},
		{
			// Helix's default javascript debug adapter has no command.
			// The slot is reported missing, but there is nothing to
			// install (D-015).
			fixture:  "health-javascript-empty-debugger.txt",
			language: "javascript",
			want: Health{
				Language: "javascript",
				LanguageServers: []Tool{
					missing("typescript-language-server", "typescript-language-server"),
				},
				DebugAdapter: missing("", ""),
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"typescript-language-server"},
		},
		{
			fixture:  "health-go-formatter-found-synthetic.txt",
			language: "go",
			want: Health{
				Language: "go",
				LanguageServers: []Tool{
					ok("gopls", "gopls", "/usr/bin/gopls"),
					ok("golangci-lint-lsp", "golangci-lint-langserver", "/home/jls/.local/bin/golangci-lint-langserver"),
				},
				DebugAdapter: ok("", "dlv", "/usr/bin/dlv"),
				Formatter:    ok("", "gofumpt", "/home/jls/go/bin/gofumpt"),
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantReady: true,
		},
		{
			fixture:  "health-go-formatter-missing-synthetic.txt",
			language: "go",
			want: Health{
				Language: "go",
				LanguageServers: []Tool{
					ok("gopls", "gopls", "/usr/bin/gopls"),
					ok("golangci-lint-lsp", "golangci-lint-langserver", "/home/jls/.local/bin/golangci-lint-langserver"),
				},
				DebugAdapter: ok("", "dlv", "/usr/bin/dlv"),
				Formatter:    missing("", "gofumpt"),
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"gofumpt"},
		},
		{
			fixture:  "health-hcl-missing-synthetic.txt",
			language: "hcl",
			want: Health{
				Language: "hcl",
				LanguageServers: []Tool{
					missing("terraform-ls", "terraform-ls"),
				},
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"terraform-ls"},
		},
		{
			fixture:  "health-typescript-missing.txt",
			language: "typescript",
			want: Health{
				Language:        "typescript",
				LanguageServers: []Tool{missing("typescript-language-server", "typescript-language-server")},
				DebugAdapter:    none,
				Formatter:       none,
				Parser:          StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"typescript-language-server"},
		},
		{
			fixture:  "health-json-missing.txt",
			language: "json",
			want: Health{
				Language:        "json",
				LanguageServers: []Tool{missing("vscode-json-language-server", "vscode-json-language-server")},
				DebugAdapter:    none,
				Formatter:       none,
				Parser:          StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusOK,
			},
			wantMissing: []string{"vscode-json-language-server"},
		},
		{
			fixture:  "health-dockerfile-missing.txt",
			language: "dockerfile",
			want: Health{
				Language:        "dockerfile",
				LanguageServers: []Tool{missing("docker-langserver", "docker-langserver")},
				DebugAdapter:    none,
				Formatter:       none,
				Parser:          StatusOK, Highlight: StatusOK, Textobjects: StatusOK, Indent: StatusMissing,
			},
			wantMissing: []string{"docker-langserver"},
		},
		{
			fixture:  "health-csv-servers-none-synthetic.txt",
			language: "csv",
			want: Health{
				Language:     "csv",
				DebugAdapter: none,
				Formatter:    none,
				Parser:       StatusOK, Highlight: StatusOK, Textobjects: StatusMissing, Indent: StatusMissing,
			},
			wantReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got, err := Parse(tt.language, fixture(t, tt.fixture))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", *got, tt.want)
			}
			if gotM := got.Missing(); strings.Join(gotM, ",") != strings.Join(tt.wantMissing, ",") {
				t.Errorf("Missing() = %v, want %v", gotM, tt.wantMissing)
			}
			if got.Ready() != tt.wantReady {
				t.Errorf("Ready() = %v, want %v", got.Ready(), tt.wantReady)
			}
		})
	}
}

func TestParseUnknownLanguage(t *testing.T) {
	_, err := Parse("nosuchlang", fixture(t, "health-unknown-language.txt"))
	if !errors.Is(err, ErrUnknownLanguage) {
		t.Fatalf("err = %v, want ErrUnknownLanguage", err)
	}
	var ule *UnknownLanguageError
	if !errors.As(err, &ule) {
		t.Fatalf("err is %T, want *UnknownLanguageError", err)
	}
	if ule.Language != "nosuchlang" {
		t.Errorf("Language = %q, want nosuchlang", ule.Language)
	}
	want := []string{"nickel", "nix", "nestedtext", "nu", "nasm", "nim", "nunjucks", "nginx"}
	if !reflect.DeepEqual(ule.Suggestions, want) {
		t.Errorf("Suggestions = %v, want %v", ule.Suggestions, want)
	}
	if !strings.Contains(err.Error(), "did you mean: nickel") {
		t.Errorf("Error() = %q, want it to list suggestions", err.Error())
	}
}

func TestParseUnknownLanguageTerraform(t *testing.T) {
	_, err := Parse("terraform", fixture(t, "health-terraform-unknown.txt"))
	var ule *UnknownLanguageError
	if !errors.As(err, &ule) {
		t.Fatalf("err = %v, want *UnknownLanguageError", err)
	}
	// Helix matches on the leading letter, so hcl is not among the
	// suggestions. That is why recipes carry aliases (D-009).
	if ule.Language != "terraform" || len(ule.Suggestions) != 20 || ule.Suggestions[0] != "toml" {
		t.Errorf("got %+v, want terraform with 20 suggestions starting with toml", ule)
	}
}

func TestParseUnexpected(t *testing.T) {
	cases := map[string]string{
		"garbage fixture": fixture(t, "health-garbage.txt"),
		"empty":           "",
		"whitespace only": "  \n\t\n",
		"wide table":      fixture(t, "health-all-languages-table.txt"),
		"indent before any header": "Configured language servers: None\n" +
			"Tree-sitter parser: ✓\n" +
			"  ✓ orphan: /usr/bin/orphan\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse("x", in)
			if !errors.Is(err, ErrUnexpectedOutput) {
				t.Errorf("err = %v, want ErrUnexpectedOutput", err)
			}
		})
	}
}

// TestParseTolerant documents the leniency rules: unknown top-level lines
// are skipped and an unknown mark yields StatusUnknown rather than an error.
func TestParseTolerant(t *testing.T) {
	in := "Configured language servers:\n" +
		"  ✓ gopls: /usr/bin/gopls\n" +
		"Some new row Helix added: ✓\n" +
		"Configured debug adapter:\n" +
		"  ? something odd\n" +
		"Configured formatter: None\n" +
		"Tree-sitter parser: ✓\n" +
		"Highlight queries: ?\n"
	h, err := Parse("go", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.LanguageServers) != 1 || h.LanguageServers[0].Status != StatusOK {
		t.Errorf("language servers = %+v", h.LanguageServers)
	}
	if h.DebugAdapter.Status != StatusUnknown {
		t.Errorf("DebugAdapter.Status = %v, want unknown", h.DebugAdapter.Status)
	}
	if h.Highlight != StatusUnknown || h.Textobjects != StatusUnknown {
		t.Errorf("Highlight = %v, Textobjects = %v, want unknown", h.Highlight, h.Textobjects)
	}
}
