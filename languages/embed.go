// Package languages bundles the installation recipes shipped with hx-ready.
//
// One YAML file per Helix language name (languages/<name>.yaml). The file
// name must match the "language" field inside it. Recipes are the only
// source of installation commands; nothing is ever derived from Helix output.
package languages

import "embed"

// FS holds every bundled recipe.
//
//go:embed *.yaml
var FS embed.FS
