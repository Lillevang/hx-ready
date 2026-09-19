package recipes

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// TestAllBundledRecipesLoad is the schema check for languages/*.yaml. Adding a
// recipe that does not parse or validate fails this test.
func TestAllBundledRecipesLoad(t *testing.T) {
	names, err := Languages()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no bundled recipes found")
	}
	for _, name := range names {
		if _, err := Load(name); err != nil {
			t.Errorf("Load(%q): %v", name, err)
		}
	}
}

func TestLoadGo(t *testing.T) {
	r, err := Load("go")
	if err != nil {
		t.Fatal(err)
	}
	if r.DisplayName != "Go" {
		t.Errorf("DisplayName = %q, want Go", r.DisplayName)
	}
	want := "gopls,golangci-lint-langserver,dlv"
	if got := strings.Join(r.Requires.All(), ","); got != want {
		t.Errorf("Requires.All() = %s, want %s", got, want)
	}

	var delve Package
	for _, p := range r.Fedora.Packages {
		if p.Name == "delve" {
			delve = p
		}
	}
	if got := strings.Join(delve.Executables(), ","); got != "dlv" {
		t.Errorf("delve provides %q, want dlv", got)
	}
	if len(r.Fedora.Commands) != 1 || r.Fedora.Commands[0].Args[0] != "go" {
		t.Errorf("expected one argv-form go install command, got %+v", r.Fedora.Commands)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("no-such-language")
	if !errors.Is(err, ErrNoRecipe) {
		t.Errorf("err = %v, want ErrNoRecipe", err)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]string{
		"name mismatch":     "language: rust\ndisplay_name: X\nfedora: {}\n",
		"no display name":   "language: x\nfedora: {}\n",
		"no fedora":         "language: x\ndisplay_name: X\n",
		"package no name":   "language: x\ndisplay_name: X\nfedora:\n  packages:\n    - provides: [a]\n",
		"command no output": "language: x\ndisplay_name: X\nfedora:\n  commands:\n    - args: [a]\n",
		"args and shell":    "language: x\ndisplay_name: X\nfedora:\n  commands:\n    - provides: [a]\n      args: [a]\n      shell: a\n",
		"neither":           "language: x\ndisplay_name: X\nfedora:\n  commands:\n    - provides: [a]\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{"x.yaml": {Data: []byte(body)}}
			if _, err := loadFrom(fsys, "x"); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}
