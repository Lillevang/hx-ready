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

// TestRequiresAreProvided asserts that every executable a recipe requires is
// provided by a package or command in the same recipe, so "install" can
// never leave a required tool uncovered.
func TestRequiresAreProvided(t *testing.T) {
	names, err := Languages()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		r, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		provided := map[string]bool{}
		for _, p := range r.Fedora.Packages {
			for _, e := range p.Executables() {
				provided[e] = true
			}
		}
		for _, c := range r.Fedora.Commands {
			for _, e := range c.Provides {
				provided[e] = true
			}
		}
		if len(r.Requires.All()) == 0 {
			t.Errorf("%s: requires is empty", name)
		}
		for _, req := range r.Requires.All() {
			if !provided[req] {
				t.Errorf("%s: requires %q but nothing in the recipe provides it", name, req)
			}
		}
	}
}

func TestBundledRecipes(t *testing.T) {
	names, err := Languages()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names, ","); got != "bash,go,hcl,rust" {
		t.Errorf("Languages() = %s, want bash,go,hcl,rust", got)
	}
	hcl, err := Load("hcl")
	if err != nil {
		t.Fatal(err)
	}
	if lang, _, err := Resolve("terraform"); err != nil || lang != "hcl" {
		t.Errorf("Resolve(terraform) = %q, %v; want hcl", lang, err)
	}
	if got := strings.Join(hcl.Aliases, ","); got != "terraform" {
		t.Errorf("hcl aliases = %q", got)
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

// TestAliasesAreUnique guards against an alias shadowing a real recipe or
// being claimed by two recipes.
func TestAliasesAreUnique(t *testing.T) {
	names, err := Languages()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, name := range names {
		seen[name] = name
	}
	for _, name := range names {
		r, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range r.Aliases {
			if owner, dup := seen[a]; dup {
				t.Errorf("alias %q of %s collides with %s", a, name, owner)
			}
			seen[a] = name
		}
	}
}

func TestResolve(t *testing.T) {
	fsys := fstest.MapFS{
		"hcl.yaml": {Data: []byte("language: hcl\ndisplay_name: HCL\naliases: [terraform, tf]\nfedora: {}\n")},
		"go.yaml":  {Data: []byte("language: go\ndisplay_name: Go\nfedora: {}\n")},
	}
	cases := []struct {
		in, wantLang string
		wantErr      error
	}{
		{"go", "go", nil},
		{"hcl", "hcl", nil},
		{"terraform", "hcl", nil},
		{"tf", "hcl", nil},
		{"ocaml", "ocaml", ErrNoRecipe},
	}
	for _, c := range cases {
		lang, r, err := resolveFrom(fsys, c.in)
		if !errors.Is(err, c.wantErr) {
			t.Errorf("Resolve(%q) err = %v, want %v", c.in, err, c.wantErr)
		}
		if lang != c.wantLang {
			t.Errorf("Resolve(%q) language = %q, want %q", c.in, lang, c.wantLang)
		}
		if c.wantErr == nil && (r == nil || r.Language != c.wantLang) {
			t.Errorf("Resolve(%q) recipe = %+v", c.in, r)
		}
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
		"self alias":        "language: x\ndisplay_name: X\naliases: [x]\nfedora: {}\n",
		"empty alias":       "language: x\ndisplay_name: X\naliases: ['']\nfedora: {}\n",
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
