package helix

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// sgrRe matches ANSI SGR sequences such as "\x1b[38;5;9m" and "\x1b[39m".
var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes colour escape sequences. Helix colours its output even
// when piped, so every parser entry point runs input through this first.
func StripANSI(s string) string {
	return sgrRe.ReplaceAllString(s, "")
}

// Line shapes produced by helix-term's health.rs (Helix 25.07). Captured
// examples live in testdata/; see docs/DECISIONS.md, D-014.
const (
	markOK      = "✓"
	markMissing = "✘"

	serversHeader   = "Configured language servers:"
	debuggerHeader  = "Configured debug adapter:"
	formatterHeader = "Configured formatter:"
	parserLine      = "Tree-sitter parser:"
	highlightLine   = "Highlight queries:"
	textobjectLine  = "Textobject queries:"
	indentLine      = "Indent queries:"

	didYouMean = "Did you mean one of these:"
)

var (
	notFoundRe        = regexp.MustCompile(`^'(.*)' not found in \$PATH$`)
	unknownLanguageRe = regexp.MustCompile(`^Language '(.*)' not found$`)
)

// UnknownLanguageError is returned when Helix does not know the requested
// language. It satisfies errors.Is(err, ErrUnknownLanguage) and carries the
// alternatives Helix suggested, if any.
type UnknownLanguageError struct {
	Language    string
	Suggestions []string
}

func (e *UnknownLanguageError) Error() string {
	if len(e.Suggestions) == 0 {
		return fmt.Sprintf("helix does not know language %q", e.Language)
	}
	return fmt.Sprintf("helix does not know language %q (did you mean: %s)", e.Language, strings.Join(e.Suggestions, ", "))
}

// Is makes errors.Is(err, ErrUnknownLanguage) true.
func (e *UnknownLanguageError) Is(target error) bool { return target == ErrUnknownLanguage }

// section tracks which "Configured ...:" header the indented item lines
// below it belong to.
type section int

const (
	sectionNone section = iota
	sectionServers
	sectionDebugger
	sectionFormatter
)

// Parse converts the output of "hx --health <language>" into a Health.
//
// It handles: found and missing language servers (several, or "None"),
// debug adapters and formatters that are present, missing or "None", an
// empty command name (two quotes with nothing between), the four tree-sitter lines,
// the "Language 'x' not found" response (an *UnknownLanguageError that
// matches ErrUnknownLanguage), and output that is not a health report at
// all (ErrUnexpectedOutput).
//
// Lines the parser does not recognise are skipped so that a newer Helix
// adding a row does not break the tool. External-tool sections must be
// complete and understood before their health can be trusted.
func Parse(language, output string) (*Health, error) {
	lines := nonEmptyLines(StripANSI(output))
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: hx printed nothing", ErrUnexpectedOutput)
	}
	if m := unknownLanguageRe.FindStringSubmatch(lines[0]); m != nil {
		return nil, parseUnknownLanguage(m[1], lines[1:])
	}
	if !strings.HasPrefix(lines[0], serversHeader) {
		return nil, fmt.Errorf("%w: expected %q, got %q", ErrUnexpectedOutput, serversHeader, truncate(lines[0], 60))
	}

	h := &Health{Language: language}
	serversNone := false
	cur := sectionNone
	for _, line := range lines {
		// Indented lines are items under the current header.
		if strings.HasPrefix(line, "  ") {
			item := strings.TrimSpace(line)
			switch cur {
			case sectionServers:
				h.LanguageServers = append(h.LanguageServers, parseNamedTool(item))
			case sectionDebugger:
				h.DebugAdapter = parseUnnamedTool(item)
			case sectionFormatter:
				h.Formatter = parseUnnamedTool(item)
			default:
				return nil, fmt.Errorf("%w: unexpected indented line %q", ErrUnexpectedOutput, truncate(item, 60))
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, serversHeader):
			cur = sectionServers
			if rest(line, serversHeader) == "None" {
				serversNone = true
				cur = sectionNone // nothing configured; no items follow
			}
		case strings.HasPrefix(line, debuggerHeader):
			cur = sectionDebugger
			if rest(line, debuggerHeader) == "None" {
				h.DebugAdapter = Tool{Status: StatusNone}
				cur = sectionNone
			}
		case strings.HasPrefix(line, formatterHeader):
			cur = sectionFormatter
			if rest(line, formatterHeader) == "None" {
				h.Formatter = Tool{Status: StatusNone}
				cur = sectionNone
			}
		case strings.HasPrefix(line, parserLine):
			cur = sectionNone
			h.Parser = parseMark(rest(line, parserLine))
		case strings.HasPrefix(line, highlightLine):
			cur = sectionNone
			h.Highlight = parseMark(rest(line, highlightLine))
		case strings.HasPrefix(line, textobjectLine):
			cur = sectionNone
			h.Textobjects = parseMark(rest(line, textobjectLine))
		case strings.HasPrefix(line, indentLine):
			cur = sectionNone
			h.Indent = parseMark(rest(line, indentLine))
		default:
			// Unknown top-level line: tolerate it, but it ends any item list.
			cur = sectionNone
		}
	}
	if (!serversNone && len(h.LanguageServers) == 0) || !h.ToolsKnown() {
		return nil, fmt.Errorf("%w: incomplete or unrecognised external-tool status", ErrUnexpectedOutput)
	}
	return h, nil
}

// parseUnknownLanguage builds the error for "Language 'x' not found",
// collecting Helix's "Did you mean one of these: a, b, c ?" suggestions.
func parseUnknownLanguage(name string, following []string) error {
	e := &UnknownLanguageError{Language: name}
	for _, line := range following {
		suggestions, ok := strings.CutPrefix(line, didYouMean)
		if !ok {
			continue
		}
		suggestions = strings.TrimSuffix(strings.TrimSpace(suggestions), "?")
		for _, s := range strings.Split(suggestions, ",") {
			if s = strings.TrimSpace(s); s != "" {
				e.Suggestions = append(e.Suggestions, s)
			}
		}
	}
	return e
}

// parseNamedTool parses a language server item:
//
//	✓ gopls: /usr/bin/gopls
//	✘ golangci-lint-lsp: 'golangci-lint-langserver' not found in $PATH
func parseNamedTool(item string) Tool {
	mark, body := cutMark(item)
	name, detail, ok := strings.Cut(body, ": ")
	if !ok {
		// No "name: detail" shape; treat the whole thing as the detail so
		// the status is still right.
		t := parseDetail(mark, body)
		return t
	}
	t := parseDetail(mark, detail)
	t.Name = name
	return t
}

// parseUnnamedTool parses a debug adapter or formatter item, which Helix
// prints without a name:
//
//	✓ /usr/bin/lldb-dap
//	✘ 'dlv' not found in $PATH
//	✘ '' not found in $PATH
func parseUnnamedTool(item string) Tool {
	mark, body := cutMark(item)
	return parseDetail(mark, body)
}

// parseDetail interprets the part after the name: either a resolved path or
// a "'bin' not found in $PATH" message.
func parseDetail(mark, detail string) Tool {
	if m := notFoundRe.FindStringSubmatch(detail); m != nil {
		return Tool{Binary: m[1], Status: StatusMissing}
	}
	if mark == markOK && detail != "" {
		return Tool{Binary: path.Base(detail), Path: detail, Status: StatusOK}
	}
	// A mark we do not know, or a shape we do not know. Keep the text so the
	// caller can show it, but do not claim to understand it.
	return Tool{Binary: detail, Status: StatusUnknown}
}

// cutMark splits "✓ rest" into its mark and remainder. When the line has no
// leading mark the whole line is returned as the remainder.
func cutMark(item string) (mark, remainder string) {
	for _, m := range []string{markOK, markMissing} {
		if r, ok := strings.CutPrefix(item, m); ok {
			return m, strings.TrimSpace(r)
		}
	}
	return "", item
}

func parseMark(s string) Status {
	switch strings.TrimSpace(s) {
	case markOK:
		return StatusOK
	case markMissing:
		return StatusMissing
	default:
		return StatusUnknown
	}
}

// rest returns what follows a header, trimmed.
func rest(line, header string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, header))
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, " \t\r")
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
