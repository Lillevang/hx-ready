package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Lillevang/hx-ready/internal/helix"
)

// newRunner returns the Helix runner commands use. It is a variable so tests
// can substitute captured output for a real hx binary.
var newRunner = func() (helix.Runner, error) {
	r := helix.ExecRunner{}
	if err := r.Check(); err != nil {
		return nil, err
	}
	return r, nil
}

// healthFor runs and parses "hx --health <language>". On failure it has
// already written an actionable message to stderr; the caller only returns
// the exit code.
func healthFor(language string, stderr io.Writer) (*helix.Health, int) {
	runner, err := newRunner()
	if err != nil {
		printHelixError(stderr, language, "", err)
		return nil, ExitError
	}
	out, err := runner.Health(language)
	if err != nil {
		printHelixError(stderr, language, out, err)
		return nil, ExitError
	}
	h, err := helix.Parse(language, out)
	if err != nil {
		printHelixError(stderr, language, out, err)
		return nil, ExitError
	}
	return h, ExitOK
}

// printHelixError turns the errors from running or parsing Helix into the
// messages described in AGENTS.md ("Error handling").
func printHelixError(w io.Writer, language, output string, err error) {
	var unknown *helix.UnknownLanguageError
	switch {
	case errors.Is(err, helix.ErrHelixNotFound):
		fmt.Fprint(w, "Could not run Helix.\n\nExpected executable:\n  hx\n\nInstall Helix first or ensure it is on PATH.\n")
	case errors.As(err, &unknown):
		fmt.Fprintf(w, "Helix does not know language %q.\n", unknown.Language)
		if len(unknown.Suggestions) > 0 {
			fmt.Fprintf(w, "\nDid you mean one of these?\n  %s\n", strings.Join(unknown.Suggestions, ", "))
		}
		fmt.Fprint(w, "\nList every language Helix knows with:\n  hx --health languages\n")
	case errors.Is(err, helix.ErrUnexpectedOutput):
		fmt.Fprintf(w, "Could not understand the output of \"hx --health %s\".\n\n%v\n", language, err)
		if strings.TrimSpace(output) != "" {
			fmt.Fprintf(w, "\nHelix printed:\n%s\n", indent(tail(helix.StripANSI(output), 12)))
		}
		fmt.Fprint(w, "\nThis is probably a Helix version hx-ready has not seen. Please report it with the output above.\n")
	default:
		fmt.Fprintf(w, "Running \"hx --health %s\" failed.\n\n%v\n", language, err)
		if strings.TrimSpace(output) != "" {
			fmt.Fprintf(w, "\nHelix printed:\n%s\n", indent(tail(helix.StripANSI(output), 12)))
		}
	}
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

// tail returns the last n lines of s.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
