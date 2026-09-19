package helix

import "regexp"

// sgrRe matches ANSI SGR sequences such as "\x1b[38;5;9m" and "\x1b[39m".
var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes colour escape sequences. Helix colours its output even
// when piped, so every parser entry point runs input through this first.
func StripANSI(s string) string {
	return sgrRe.ReplaceAllString(s, "")
}

// Parse converts the output of "hx --health <language>" into a Health.
//
// It must handle: found and missing language servers (possibly several),
// debug adapters and formatters both present, missing and "None", the
// tree-sitter/highlight/textobject/indent lines, the
// "Language 'x' not found" response (ErrUnknownLanguage), and output that
// is not a health report at all (ErrUnexpectedOutput).
//
// See internal/helix/testdata for captured fixtures.
func Parse(language, output string) (*Health, error) {
	_ = StripANSI(output)
	return nil, ErrNotImplemented
}
