package helix

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTable(t *testing.T) {
	rows, err := ParseTable(fixture(t, "health-all-languages-table.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 277 {
		t.Errorf("rows = %d, want 277", len(rows))
	}
	byName := map[string]LanguageRow{}
	for _, r := range rows {
		byName[r.Language] = r
	}

	// go: both servers (the second on a continuation row) and dlv missing.
	g := byName["go"]
	if len(g.LanguageServers) != 2 || g.LanguageServers[0].Name != "gopls" || g.LanguageServers[1].Name != "golangci-lint-lsp" || g.LanguageServers[0].Status != StatusMissing {
		t.Errorf("go servers = %+v", g.LanguageServers)
	}
	if g.DebugAdapter.Name != "dlv" || g.DebugAdapter.Status != StatusMissing || g.Formatter.Status != StatusNone {
		t.Errorf("go adapter/formatter = %+v / %+v", g.DebugAdapter, g.Formatter)
	}
	if g.HasInstalledTool() {
		t.Error("go should have no installed tool")
	}

	// python: continuation rows carry the other servers; ruff is present.
	p := byName["python"]
	var names []string
	for _, s := range p.LanguageServers {
		names = append(names, s.Name+":"+s.Status.String())
	}
	if got := strings.Join(names, ","); got != "ty:missing,ruff:ok,jedi:missing,pylsp:missing" {
		t.Errorf("python servers = %s", got)
	}
	if !p.HasInstalledTool() {
		t.Error("python should count as having an installed tool (ruff)")
	}

	// rust: everything green.
	r := byName["rust"]
	if !r.HasInstalledTool() || r.DebugAdapter.Name != "lldb-dap" || r.DebugAdapter.Status != StatusOK {
		t.Errorf("rust = %+v", r)
	}
	if r.Highlight != StatusOK || r.Textobjects != StatusOK || r.Indent != StatusOK {
		t.Errorf("rust queries = %v %v %v", r.Highlight, r.Textobjects, r.Indent)
	}

	// crystal: formatter present, continuation server present.
	c := byName["crystal"]
	if c.Formatter.Status != StatusOK || c.Formatter.Name != "crystal" || len(c.LanguageServers) != 2 || c.LanguageServers[1].Name != "ameba-ls" {
		t.Errorf("crystal = %+v", c)
	}

	// adl: nothing configured at all.
	a := byName["adl"]
	if len(a.LanguageServers) != 0 || a.DebugAdapter.Status != StatusNone || a.Formatter.Status != StatusNone || a.HasInstalledTool() {
		t.Errorf("adl = %+v", a)
	}

	// agda: missing queries are reported.
	if q := byName["agda"]; q.Highlight != StatusOK || q.Textobjects != StatusMissing || q.Indent != StatusMissing {
		t.Errorf("agda queries = %+v", q)
	}

	// Truncated names lose the ellipsis (the fixture has at least one).
	truncated := 0
	for _, r := range rows {
		for _, s := range r.LanguageServers {
			if strings.Contains(s.Name, "…") {
				t.Errorf("%s: name still truncated: %q", r.Language, s.Name)
			}
			if strings.HasPrefix(s.Name, "vscode-eslint-language-server") {
				truncated++
			}
		}
	}
	if truncated == 0 {
		t.Error("expected the truncated vscode-eslint-language-server row to be parsed")
	}
}

func TestParseTableErrors(t *testing.T) {
	cases := map[string]string{
		"garbage":         fixture(t, "health-garbage.txt"),
		"single language": fixture(t, "health-go-missing.txt"),
		"header only":     "Language   Language servers   Debug adapter   Formatter   Highlight   Textobject   Indent\n",
		"continuation first": "Language   Language servers   Debug adapter   Formatter   Highlight   Textobject   Indent\n" +
			"           ✘ orphan\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTable(in); !errors.Is(err, ErrUnexpectedOutput) {
				t.Errorf("err = %v, want ErrUnexpectedOutput", err)
			}
		})
	}
}

// TestParseTableAfterClipboardInfo covers plain "hx --health", which prints
// clipboard details before the table.
func TestParseTableAfterClipboardInfo(t *testing.T) {
	in := "Config file: default\nLanguage file: default\nLog file: /tmp/x\nRuntime directories: /tmp/y\nClipboard provider: wl-clipboard\nSystem clipboard provider: wl-clipboard\n\n" +
		fixture(t, "health-all-languages-table.txt")
	rows, err := ParseTable(in)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Language != "ada" {
		t.Errorf("first row = %q, want ada", rows[0].Language)
	}
}
