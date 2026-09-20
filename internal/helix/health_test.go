package helix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(b)
}

func TestStripANSI(t *testing.T) {
	got := StripANSI(fixture(t, "health-go-missing.txt"))
	want := fixture(t, "health-go-missing-plain.txt")
	if got != want {
		t.Errorf("StripANSI mismatch\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Error("escape character survived StripANSI")
	}
}

func TestValidateLanguageName(t *testing.T) {
	valid := []string{"go", "c-sharp", "cpp", "git-commit", "json5", "c++", "python"}
	for _, name := range valid {
		if err := ValidateLanguageName(name); err != nil {
			t.Errorf("ValidateLanguageName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "Go", "go lang", "go;ls", "-go", "../go", "go\n"}
	for _, name := range invalid {
		if err := ValidateLanguageName(name); err == nil {
			t.Errorf("ValidateLanguageName(%q) = nil, want error", name)
		}
	}
}

func TestHealthMissingAndReady(t *testing.T) {
	h := &Health{
		Language: "go",
		LanguageServers: []Tool{
			{Name: "gopls", Binary: "gopls", Status: StatusMissing},
			{Name: "golangci-lint-lsp", Binary: "golangci-lint-langserver", Status: StatusOK, Path: "/usr/bin/golangci-lint-langserver"},
		},
		DebugAdapter: Tool{Binary: "dlv", Status: StatusMissing},
		Formatter:    Tool{Status: StatusNone},
	}
	got := h.Missing()
	want := []string{"gopls", "dlv"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
	if h.Ready() {
		t.Error("Ready() = true, want false")
	}

	h.LanguageServers[0].Status = StatusOK
	h.DebugAdapter.Status = StatusOK
	if !h.Ready() {
		t.Error("Ready() = false after everything found, want true")
	}
}

// TestParseFixturesExist keeps the fixture set honest: every file the parser
// task (docs/TASKS.md, T-01) is expected to cover must be present.
func TestParseFixturesExist(t *testing.T) {
	required := []string{
		"health-go-missing.txt",
		"health-go-missing-plain.txt",
		"health-python-partial.txt",
		"health-rust-ready.txt",
		"health-bash-ready.txt",
		"health-yaml-missing.txt",
		"health-hcl-ready.txt",
		"health-ocaml-no-recipe.txt",
		"health-javascript-empty-debugger.txt",
		"health-unknown-language.txt",
		"health-terraform-unknown.txt",
		"health-garbage.txt",
		"health-all-languages-table.txt",
		"health-all-languages-table-narrow.txt",
		"health-go-formatter-found-synthetic.txt",
		"health-go-formatter-missing-synthetic.txt",
		"health-csv-servers-none-synthetic.txt",
		"health-hcl-missing-synthetic.txt",
		"health-python-ready-uncovered-synthetic.txt",
		"health-typescript-missing.txt",
		"health-json-missing.txt",
		"health-dockerfile-missing.txt",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join("testdata", name)); err != nil {
			t.Errorf("missing fixture %s: %v", name, err)
		}
	}
}
