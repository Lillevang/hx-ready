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

// Load returns the bundled recipe for a Helix language name.
func Load(language string) (*Recipe, error) {
	return loadFrom(languages.FS, language)
}

// Languages returns the names of every bundled recipe, sorted.
func Languages() ([]string, error) {
	entries, err := fs.ReadDir(languages.FS, ".")
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
