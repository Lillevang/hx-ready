package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestFlagsFirst(t *testing.T) {
	cases := []struct{ in, want string }{
		{"go --dry-run", "--dry-run go"},
		{"--dry-run go", "--dry-run go"},
		{"go --dry-run --yes", "--dry-run --yes go"},
		{"-- -weird", "-- -weird"},
		{"go -- --dry-run", "go -- --dry-run"},
		{"", ""},
	}
	for _, c := range cases {
		var in []string
		if c.in != "" {
			in = strings.Fields(c.in)
		}
		if got := strings.Join(flagsFirst(in), " "); got != c.want {
			t.Errorf("flagsFirst(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string // substring of stdout
		wantErr  string // substring of stderr
	}{
		{"no args prints usage", nil, ExitUsage, "", "Usage:"},
		{"help", []string{"help"}, ExitOK, "Usage:", ""},
		{"--help", []string{"--help"}, ExitOK, "Usage:", ""},
		{"unknown command", []string{"frobnicate"}, ExitUsage, "", `unknown command "frobnicate"`},
		{"version", []string{"version"}, ExitOK, "hx-ready dev", ""},
		{"check without language", []string{"check"}, ExitUsage, "", "Usage: hx-ready check"},
		{"check with bad language", []string{"check", "go; rm -rf /"}, ExitUsage, "", "not a valid Helix language name"},
		{"install without language", []string{"install", "--dry-run"}, ExitUsage, "", "Usage: hx-ready install"},
		{"doctor with extra arg", []string{"doctor", "go"}, ExitUsage, "", "Usage: hx-ready doctor"},
		{"flag after positional", []string{"install", "go", "--bogus"}, ExitUsage, "", "flag provided but not defined: -bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
			if got != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", got, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantOut)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantErr)
			}
		})
	}
}
