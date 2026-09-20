package installer

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Lillevang/hx-ready/internal/recipes"
)

func goRecipe(t *testing.T) *recipes.Recipe {
	t.Helper()
	r, err := recipes.Load("go")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// nothing reports every executable as absent, the state of a fresh machine.
func nothing(string) bool { return false }

// only reports the named executables as present.
func only(exes ...string) func(string) bool {
	set := toSet(exes)
	return func(e string) bool { return set[e] }
}

func stepStrings(p *Plan) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.String())
	}
	return out
}

func TestFedoraGoAllMissing(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	plan, err := Build(Fedora, goRecipe(t), []string{"gopls", "golangci-lint-langserver", "dlv"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sudo dnf install -y golang gopls delve golangci-lint",
		"GOBIN=/home/tester/.local/bin \\\n  go install github.com/nametake/golangci-lint-langserver@latest",
	}
	if got := stepStrings(plan); strings.Join(got, "\n---\n") != strings.Join(want, "\n---\n") {
		t.Errorf("steps =\n%s\nwant\n%s", strings.Join(got, "\n---\n"), strings.Join(want, "\n---\n"))
	}
	if !plan.Steps[0].Privileged || plan.Steps[1].Privileged {
		t.Errorf("privileged flags wrong: %+v", plan.Steps)
	}
	if got := strings.Join(plan.Steps[0].Provides, ","); got != "dlv,gopls" {
		t.Errorf("package step provides %q, want dlv,gopls", got)
	}
	if len(plan.Uncovered) != 0 {
		t.Errorf("Uncovered = %v, want none", plan.Uncovered)
	}
	if !plan.Privileged() {
		t.Error("Privileged() = false")
	}
}

func TestFedoraGoOnlyDebugger(t *testing.T) {
	plan, err := Build(Fedora, goRecipe(t), []string{"dlv"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sudo dnf install -y delve"}
	if got := stepStrings(plan); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("steps = %v, want %v", got, want)
	}
}

// The go install command needs go and golangci-lint, so those packages are
// kept even though Helix never reports them missing.
func TestFedoraGoOnlyLangserver(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	plan, err := Build(Fedora, goRecipe(t), []string{"golangci-lint-langserver"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sudo dnf install -y golang golangci-lint",
		"GOBIN=/home/tester/.local/bin \\\n  go install github.com/nametake/golangci-lint-langserver@latest",
	}
	if got := stepStrings(plan); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

func TestFedoraNothingMissing(t *testing.T) {
	plan, err := Build(Fedora, goRecipe(t), nil, nothing)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Empty() || plan.Privileged() {
		t.Errorf("plan = %+v, want empty", plan)
	}
}

func TestFedoraUncovered(t *testing.T) {
	plan, err := Build(Fedora, goRecipe(t), []string{"gopls", "gofumpt"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.Uncovered, ","); got != "gofumpt" {
		t.Errorf("Uncovered = %q, want gofumpt", got)
	}
	if got := stepStrings(plan); len(got) != 1 || got[0] != "sudo dnf install -y gopls" {
		t.Errorf("steps = %q", got)
	}
}

func TestFedoraOrdersCommandsByNeeds(t *testing.T) {
	r := &recipes.Recipe{
		Language: "x",
		Fedora: &recipes.Platform{
			Commands: []recipes.Command{
				{Provides: []string{"b"}, Args: []string{"make-b"}, Needs: []string{"a"}},
				{Provides: []string{"a"}, Args: []string{"make-a"}, Sudo: true},
				{Provides: []string{"c"}, Shell: "echo c > c", Needs: []string{"b"}},
			},
		},
	}
	plan, err := Build(Fedora, r, []string{"a", "b", "c"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sudo make-a", "make-b", "echo c > c"}
	if got := stepStrings(plan); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

func TestFedoraCycle(t *testing.T) {
	r := &recipes.Recipe{
		Language: "x",
		Fedora: &recipes.Platform{
			Commands: []recipes.Command{
				{Provides: []string{"a"}, Args: []string{"make-a"}, Needs: []string{"b"}},
				{Provides: []string{"b"}, Args: []string{"make-b"}, Needs: []string{"a"}},
			},
		},
	}
	if _, err := Build(Fedora, r, []string{"a", "b"}, nothing); err == nil {
		t.Error("expected a cycle error")
	}
}

func TestBuildDebian(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	plan, err := Build(Debian, goRecipe(t), []string{"gopls", "golangci-lint-langserver", "dlv"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStrings(plan); len(got) != 3 || got[0] != "sudo apt-get install -y golang-go gopls delve" {
		t.Errorf("steps = %q", got)
	}
}

// A command needed by another command is pulled in even though Helix never
// reports what it provides as missing (golangci-lint on Ubuntu).
func TestBuildNeededCommand(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	plan, err := Build(Debian, goRecipe(t), []string{"golangci-lint-langserver"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sudo apt-get install -y golang-go",
		"GOBIN=/home/tester/.local/bin \\\n  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
		"GOBIN=/home/tester/.local/bin \\\n  go install github.com/nametake/golangci-lint-langserver@latest",
	}
	if got := stepStrings(plan); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

// Toolchains already on PATH are not reinstalled: with npm present, only
// the npm install step remains; with go present, only golangci-lint.
func TestBuildSkipsPresentNeeds(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	yaml, err := recipes.Load("yaml")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Build(Fedora, yaml, []string{"yaml-language-server"}, only("npm"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStrings(plan); len(got) != 1 || strings.HasPrefix(got[0], "sudo") {
		t.Errorf("steps = %q, want only the npm install", got)
	}

	plan, err = Build(Fedora, goRecipe(t), []string{"golangci-lint-langserver"}, only("go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStrings(plan); len(got) != 2 || got[0] != "sudo dnf install -y golangci-lint" {
		t.Errorf("steps = %q, want golangci-lint package then go install", got)
	}

	// On Debian golangci-lint is itself a command; with go and
	// golangci-lint present only the langserver step remains.
	plan, err = Build(Debian, goRecipe(t), []string{"golangci-lint-langserver"}, only("go", "golangci-lint"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStrings(plan); len(got) != 1 || !strings.Contains(got[0], "golangci-lint-langserver@latest") {
		t.Errorf("steps = %q, want only the langserver step", got)
	}

	// What Helix reports missing is installed even if a same-named
	// executable turns up on PATH: Helix's verdict wins.
	plan, err = Build(Fedora, goRecipe(t), []string{"gopls"}, only("gopls"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stepStrings(plan); len(got) != 1 || got[0] != "sudo dnf install -y gopls" {
		t.Errorf("steps = %q", got)
	}
}

func TestBuildNoBlock(t *testing.T) {
	r := &recipes.Recipe{Language: "x", Fedora: &recipes.Platform{}}
	if _, err := Build(Debian, r, []string{"a"}, nothing); !errors.Is(err, ErrNoPlatformBlock) {
		t.Errorf("err = %v, want ErrNoPlatformBlock", err)
	}
}

func TestDryRun(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	plan, err := Build(Fedora, goRecipe(t), []string{"gopls", "golangci-lint-langserver", "dlv"}, nothing)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := plan.Run(DryRun{Out: &out}); err != nil {
		t.Fatal(err)
	}
	want := "\n  sudo dnf install -y golang gopls delve golangci-lint\n" +
		"\n  GOBIN=/home/tester/.local/bin \\\n    go install github.com/nametake/golangci-lint-langserver@latest\n"
	if out.String() != want {
		t.Errorf("dry run output\n got: %q\nwant: %q", out.String(), want)
	}
}

func TestStepString(t *testing.T) {
	cases := []struct {
		step Step
		want string
	}{
		{Step{Args: []string{"echo", "hello world", "it's"}}, `echo 'hello world' 'it'\''s'`},
		{Step{Args: []string{"x"}, Env: []string{"A=1", "B=2"}}, "A=1 \\\nB=2 \\\n  x"},
		{Step{Shell: "a | b", Privileged: true}, "sudo a | b"},
		{Step{Args: []string{"dnf", "install", "-y", "go"}, Privileged: true}, "sudo dnf install -y go"},
	}
	for _, c := range cases {
		if got := c.step.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}
