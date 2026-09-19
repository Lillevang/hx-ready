// Package recipes loads the bundled, declarative installation recipes.
//
// A recipe maps the executables Helix reports (gopls, dlv, ...) to the
// packages and commands that provide them on a given platform. Recipes are
// data, not code; see languages/*.yaml.
package recipes

// Recipe describes how to make one Helix language ready.
type Recipe struct {
	// Language is the Helix language name, e.g. "go" or "c-sharp".
	Language    string `yaml:"language"`
	DisplayName string `yaml:"display_name"`

	// Aliases are other names users type for this language, e.g.
	// "terraform" for Helix's "hcl". Resolve maps them to Language before
	// anything reaches hx. See docs/DECISIONS.md, D-009.
	Aliases []string `yaml:"aliases"`

	// Requires lists the executables that must be on PATH for hx-ready to
	// report "Ready". Helix may configure more tools than this; those are
	// reported but not installed. See docs/DECISIONS.md, D-004.
	Requires Requires `yaml:"requires"`

	// One block per platform. Only Fedora exists today; adding a platform
	// means adding a field here and an installer, not rewriting the model.
	Fedora *Platform `yaml:"fedora"`
}

// Requires names the executables a language needs, as Helix reports them.
type Requires struct {
	LanguageServers []string `yaml:"language_servers"`
	DebugAdapters   []string `yaml:"debug_adapters"`
	Formatters      []string `yaml:"formatters"`
}

// All returns every required executable, in a stable order.
func (r Requires) All() []string {
	var out []string
	out = append(out, r.LanguageServers...)
	out = append(out, r.DebugAdapters...)
	out = append(out, r.Formatters...)
	return out
}

// Platform is the installation knowledge for one operating system.
type Platform struct {
	Packages []Package `yaml:"packages"`
	Commands []Command `yaml:"commands"`
}

// Package is a distribution package installed with the system package
// manager (dnf on Fedora). Installing packages always requires sudo.
type Package struct {
	Name string `yaml:"name"`
	// Provides lists the executables the package puts on PATH. Defaults to
	// [Name] when omitted, which covers the common case (gopls -> gopls)
	// and makes the exceptions explicit (delve -> dlv).
	Provides []string `yaml:"provides"`
}

// Executables returns Provides, or [Name] when Provides is empty.
func (p Package) Executables() []string {
	if len(p.Provides) == 0 {
		return []string{p.Name}
	}
	return p.Provides
}

// Command is a non-package installation step, such as "go install" or
// "npm install -g". Exactly one of Args or Shell must be set.
type Command struct {
	// Provides lists the executables this command puts on PATH. Required.
	Provides []string `yaml:"provides"`
	// Args is the preferred form: an argv slice run without a shell.
	Args []string `yaml:"args"`
	// Shell is the explicit escape hatch: a string run via "sh -c". Use it
	// only when argv form is impossible, and say why in a YAML comment.
	Shell string `yaml:"shell"`
	// Env sets extra environment variables for the command. Values may
	// reference ${HOME} and other variables; they are expanded by hx-ready.
	Env map[string]string `yaml:"env"`
	// Needs lists executables that must exist before this command runs
	// (e.g. "go" before "go install"). Used to order steps and to explain
	// failures.
	Needs []string `yaml:"needs"`
	// Sudo marks a command that must run with elevated privileges.
	Sudo bool `yaml:"sudo"`
}
