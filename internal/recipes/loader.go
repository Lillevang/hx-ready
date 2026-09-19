package recipes

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Lillevang/hx-ready/languages"
)

// ErrNoRecipe is returned when no bundled recipe exists for a language.
var ErrNoRecipe = errors.New("no hx-ready recipe for this language")

// Source is where recipes are read from: the embedded languages/ directory.
// Tests may point it at an in-memory fs.FS.
var Source fs.FS = languages.FS

// Load returns the bundled recipe for a Helix language name. Aliases are not
// consulted; use Resolve for user input.
func Load(language string) (*Recipe, error) {
	return loadFrom(Source, language)
}

// Resolve maps a user-supplied name to a recipe, following aliases (D-009).
// It returns the canonical Helix language name alongside the recipe, so the
// caller can tell the user when an alias was followed. When no recipe or
// alias matches, the name is returned unchanged with ErrNoRecipe.
func Resolve(name string) (canonical string, r *Recipe, err error) {
	return resolveFrom(Source, name)
}

func resolveFrom(fsys fs.FS, name string) (string, *Recipe, error) {
	r, err := loadFrom(fsys, name)
	if err == nil {
		return name, r, nil
	}
	if !errors.Is(err, ErrNoRecipe) {
		return name, nil, err
	}
	names, err := languagesIn(fsys)
	if err != nil {
		return name, nil, err
	}
	for _, lang := range names {
		r, err := loadFrom(fsys, lang)
		if err != nil {
			return name, nil, err
		}
		for _, a := range r.Aliases {
			if a == name {
				return lang, r, nil
			}
		}
	}
	return name, nil, ErrNoRecipe
}

// Languages returns the names of every bundled recipe, sorted.
func Languages() ([]string, error) {
	return languagesIn(Source)
}

func languagesIn(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".yaml"); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func loadFrom(fsys fs.FS, language string) (*Recipe, error) {
	b, err := fs.ReadFile(fsys, language+".yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoRecipe
	}
	if err != nil {
		return nil, err
	}
	var r Recipe
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("recipe %s.yaml: %w", language, err)
	}
	if err := r.validate(language); err != nil {
		return nil, fmt.Errorf("recipe %s.yaml: %w", language, err)
	}
	return &r, nil
}

// validate catches recipe authoring mistakes at load time so they surface in
// tests rather than on a user's machine.
func (r *Recipe) validate(filename string) error {
	if r.Language != filename {
		return fmt.Errorf("language field %q does not match file name %q", r.Language, filename)
	}
	if r.DisplayName == "" {
		return errors.New("display_name is required")
	}
	for _, a := range r.Aliases {
		if a == "" || a == r.Language {
			return fmt.Errorf("alias %q is empty or equal to the language name", a)
		}
	}
	if r.Fedora == nil {
		return errors.New("fedora block is required (it is the only supported platform)")
	}
	for i, p := range r.Fedora.Packages {
		if p.Name == "" {
			return fmt.Errorf("fedora.packages[%d]: name is required", i)
		}
	}
	for i, c := range r.Fedora.Commands {
		if len(c.Provides) == 0 {
			return fmt.Errorf("fedora.commands[%d]: provides is required", i)
		}
		if (len(c.Args) == 0) == (c.Shell == "") {
			return fmt.Errorf("fedora.commands[%d]: exactly one of args or shell must be set", i)
		}
	}
	return nil
}
