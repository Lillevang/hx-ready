package cmd

import (
	"github.com/Lillevang/hx-ready/internal/helix"
	"github.com/Lillevang/hx-ready/internal/recipes"
)

// verdict is hx-ready's judgement of a health report, which differs from
// Helix's own when a recipe exists: the recipe's requires is the target and
// other tools Helix lists are reported but neither installed nor counted
// against readiness (D-017). Without a recipe, everything Helix lists
// counts, as Helix itself sees it.
type verdict struct {
	// missing lists executables to install: Helix reports them missing and
	// (when a recipe exists) the recipe requires them.
	missing []string
	// uncovered lists tools Helix reports missing that the recipe does not
	// require. Empty without a recipe.
	uncovered []helix.Tool
	ready     bool
}

func assess(h *helix.Health, r *recipes.Recipe) verdict {
	if r == nil {
		return verdict{missing: h.Missing(), ready: h.Ready()}
	}
	required := map[string]bool{}
	for _, name := range r.Requires.All() {
		required[name] = true
	}
	var v verdict
	consider := func(t helix.Tool) {
		if t.Status != helix.StatusMissing {
			return
		}
		switch {
		case t.Binary == "":
			// Nothing to install (D-015); shown by the renderer, not a gap.
		case required[t.Binary]:
			v.missing = append(v.missing, t.Binary)
		default:
			v.uncovered = append(v.uncovered, t)
		}
	}
	for _, ls := range h.LanguageServers {
		consider(ls)
	}
	consider(h.DebugAdapter)
	consider(h.Formatter)
	v.ready = h.ToolsKnown() && len(v.missing) == 0
	return v
}

// covered reports whether the recipe requires this tool. Without a recipe
// every tool counts.
func (v verdict) covered(t helix.Tool) bool {
	for _, u := range v.uncovered {
		if u.Binary == t.Binary && u.Name == t.Name {
			return false
		}
	}
	return true
}
