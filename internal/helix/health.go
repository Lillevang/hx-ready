// Package helix runs Helix's own health checks and turns the output into data.
//
// Helix is the source of truth for what a language needs. This package never
// reimplements Helix's configuration resolution; it only executes
// "hx --health <language>" and parses the result.
package helix

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// Status is the state of one capability as reported by Helix.
type Status int

const (
	StatusUnknown Status = iota // could not be determined from the output
	StatusOK                    // ✓
	StatusMissing               // ✘
	StatusNone                  // Helix has nothing configured for this slot
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusMissing:
		return "missing"
	case StatusNone:
		return "none"
	default:
		return "unknown"
	}
}

// Tool is one external executable Helix wants for a language: a language
// server, debug adapter, or formatter.
type Tool struct {
	// Name is what Helix calls the tool, e.g. "golangci-lint-lsp".
	// Empty for debug adapters and formatters, which Helix reports without a name.
	Name string
	// Binary is the executable Helix looks for on PATH, e.g. "golangci-lint-langserver".
	Binary string
	// Path is the resolved location when Status is StatusOK.
	Path   string
	Status Status
}

// Health is the parsed result of "hx --health <language>".
type Health struct {
	Language        string
	LanguageServers []Tool
	DebugAdapter    Tool // Status is StatusNone when Helix has no adapter configured
	Formatter       Tool // Status is StatusNone when Helix has no formatter configured
	Parser          Status
	Highlight       Status
	Textobjects     Status
	Indent          Status
}

// Missing returns the executables Helix wants but cannot find, in report order.
//
// A tool Helix reports as missing but with an empty command name (which the
// default javascript debug adapter produces) is left out: there is no
// executable to install. See docs/DECISIONS.md, D-015.
func (h *Health) Missing() []string {
	var out []string
	add := func(t Tool) {
		if t.Status == StatusMissing && t.Binary != "" {
			out = append(out, t.Binary)
		}
	}
	for _, ls := range h.LanguageServers {
		add(ls)
	}
	add(h.DebugAdapter)
	add(h.Formatter)
	return out
}

// Ready reports whether every configured tool is present. A slot Helix has
// not configured at all (StatusNone) does not count against readiness; see
// docs/DECISIONS.md, D-005.
func (h *Health) Ready() bool {
	return len(h.Missing()) == 0
}

// Sentinel errors. Callers should print an actionable message for each.
var (
	ErrNotImplemented   = errors.New("not implemented yet")
	ErrHelixNotFound    = errors.New("could not run Helix: executable \"hx\" not found on PATH")
	ErrUnknownLanguage  = errors.New("helix does not know this language")
	ErrUnexpectedOutput = errors.New("could not understand hx --health output")
)

var languageNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.+-]*$`)

// ValidateLanguageName rejects anything that is not a plausible Helix language
// identifier. Language names come from the user and end up as an argument to
// hx, so keep the accepted alphabet small.
func ValidateLanguageName(name string) error {
	if !languageNameRe.MatchString(name) {
		return fmt.Errorf("%q is not a valid Helix language name (expected lowercase letters, digits, - _ . +)", name)
	}
	return nil
}

// Runner executes Helix health checks. It is an interface so the rest of the
// program can be tested against captured output instead of a real Helix.
type Runner interface {
	// Health returns the raw output of "hx --health <language>".
	// An empty language runs the full "hx --health" report.
	Health(language string) (string, error)
}

// ExecRunner runs the real hx binary.
type ExecRunner struct {
	// Path is the hx executable. Empty means "hx", resolved via PATH.
	Path string
}

func (r ExecRunner) exe() string {
	if r.Path == "" {
		return "hx"
	}
	return r.Path
}

// Check confirms Helix is runnable. Returns ErrHelixNotFound otherwise.
func (r ExecRunner) Check() error {
	if _, err := exec.LookPath(r.exe()); err != nil {
		return ErrHelixNotFound
	}
	return nil
}

// Health runs "hx --health [language]" and returns its combined output.
// Helix emits ANSI colour codes even when not attached to a terminal; callers
// should pass the output through StripANSI (Parse does this).
func (r ExecRunner) Health(language string) (string, error) {
	args := []string{"--health"}
	if language != "" {
		if err := ValidateLanguageName(language); err != nil {
			return "", err
		}
		args = append(args, language)
	}
	cmd := exec.Command(r.exe(), args...)
	// The language table truncates names to fit the terminal width. Ask
	// for a wide one so doctor sees whole names (D-006). Helix only honours
	// COLUMNS when TERM is set, and a session without a tty (ssh without
	// -t, CI) has no TERM, so supply a harmless one.
	cmd.Env = append(os.Environ(), "COLUMNS=250")
	if os.Getenv("TERM") == "" {
		cmd.Env = append(cmd.Env, "TERM=dumb")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrHelixNotFound
		}
		return string(out), fmt.Errorf("hx --health failed: %w", err)
	}
	return string(out), nil
}
