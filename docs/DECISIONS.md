# Decisions

Numbered so tasks and code comments can point at them. Add a new entry
rather than editing an old one when a decision is reversed.

## Decided

### D-001 Module path
`github.com/Lillevang/hx-ready`, matching the namespace of linux-bootstrap.

### D-002 Standard library `flag`, not Cobra
Three subcommands and three flags do not justify a dependency. `cmd/root.go`
is a name-to-function table. Revisit if commands gain nested subcommands
or shell completion becomes a requirement.

### D-003 Recipes are embedded YAML
`gopkg.in/yaml.v3` is the only dependency. Recipes live in `languages/`
and are compiled in with `go:embed`, so the binary is self-contained. The
file name must equal the `language` field; the loader enforces it.

### D-004 Recipes name executables, not Helix display names
`requires` and `provides` use the binary Helix looks for on PATH
(`golangci-lint-langserver`, `dlv`), because that is what can be checked
and what Helix prints in "not found" messages. Helix's own label for a
server (`golangci-lint-lsp`) is kept in `helix.Tool.Name` for display.

### D-005 "None" is not "missing"
When Helix reports a formatter or debug adapter as `None`, nothing is
configured and there is nothing to install. hx-ready never edits Helix
config, so `Ready()` ignores those slots.

### D-006 `doctor` candidate rule
A language appears in `doctor` when it has a recipe, or when its row in
the wide `hx --health languages` table (`COLUMNS=250`) shows at least one
✓ tool. The table only shows the first language server per language, so
`doctor` then runs `hx --health <lang>` for each candidate to get the full
picture. `--all` skips the filter.

### D-007 User-level installs go to `~/.local/bin`; PATH is never edited
`go install` uses `GOBIN=${HOME}/.local/bin`. After installing, if the
binary exists there but Helix still reports it missing, print a one-line
PATH hint. Shell rc files are not touched.

### D-008 Install refuses to run on non-Fedora
`install` reads `/etc/os-release` and requires `ID=fedora`. `check` works
anywhere Helix runs. The error names the file and the expected value.

### D-009 Recipe aliases
A recipe may declare `aliases: [terraform]`. `check terraform` resolves to
`hcl`, says so on the first line, and continues. Aliases are resolved
before anything is passed to `hx`.

### D-010 argv first, shell explicit
Recipe commands use `args:` (no shell). `shell:` exists as a visible
escape hatch and the loader rejects a command that sets both or neither.
Environment values are expanded with `os.ExpandEnv` and come only from
bundled recipes.

### D-011 One confirmation, then `dnf install -y`
hx-ready shows the full plan and asks once. Package steps then run
`sudo dnf install -y ...` so dnf does not prompt a second time. `--yes`
skips the hx-ready prompt. sudo's own password prompt is left alone;
hx-ready never handles credentials.

### D-012 Exit codes
0 ready or success, 1 not ready or failure, 2 usage error.

### D-013 Language names are validated before reaching `hx`
`^[a-z0-9][a-z0-9_.+-]*$`, enforced in `helix.ValidateLanguageName`. The
name is passed as an exec argument, never through a shell, but the alphabet
is kept small anyway.

### D-014 Fixtures are real captures
`internal/helix/testdata/*.txt` are byte-for-byte `hx --health` output
from Helix 25.07.1 on Fedora 43, ANSI codes included. Hand-written
fixtures are named with a `-synthetic` suffix so the distinction stays
visible.

### D-015 An empty command name is not a missing executable
Helix's default javascript debug adapter has no command, so `hx --health
javascript` prints `✘ '' not found in $PATH`. The parser keeps the tool with
`StatusMissing` and an empty `Binary` so `check` can show the slot, but
`Missing()` skips it: nothing can be installed to fix it, and hx-ready never
edits Helix config (D-005).

### D-016 Install channels (answers Q-01)
Recipes may use Fedora packages plus `npm install -g`, `pipx install`,
`go install` and `cargo install`, each with an explicit `needs` on the
toolchain executable so the toolchain package is pulled in. User-level
channels install into `~/.local` (D-007): `GOBIN`, `npm_config_prefix`,
pipx's default `~/.local/bin`, `CARGO_INSTALL_ROOT`. No downloading of
release binaries: that is where the tool becomes a package manager.
Decided 2026-09-20.

### D-017 Readiness follows the recipe (answers Q-02)
When a recipe exists, its `requires` is the target. Tools Helix lists that
the recipe does not require are shown as "not covered" (⚠) and do not block
"Ready" or fail `install`. Without a recipe, readiness is everything Helix
lists, as before. Decided 2026-09-20.

### D-018 Toolchain prerequisites (answers Q-03)
A recipe may list a runtime or toolchain as an ordinary package when the
platform packages it (`golang`, `nodejs-npm`, `java-21-openjdk`). When it
does not, the recipe stops with an actionable message rather than
downloading one. Decided 2026-09-20.

### D-019 Second platform is Ubuntu/Debian (answers Q-05)
Detection: `ID=ubuntu` or `ID=debian`, or `ID_LIKE` containing `debian`.
Package steps use `sudo apt-get install -y`. Recipes gain a `debian` block
with the same shape as `fedora`; a recipe without a block for the running
platform reports that plainly. Decided 2026-09-20.

### D-020 Formatters are installed only when Helix asks (answers Q-06)
Recipes may provide formatters, but a formatter is installed only when
`hx --health` reports it missing, i.e. the user has configured one. No
bundled recipe requires a formatter, since Helix's defaults configure none.
Decided 2026-09-20.

### D-021 terraform-ls via go install (answers Q-07)
Follows from D-016: `go install github.com/hashicorp/terraform-ls@latest`
with `GOBIN=${HOME}/.local/bin`, needing the `golang` package. No
third-party rpm repository. Decided 2026-09-20.

## Open questions

Each entry states what it blocks and a proposed default. Answering one
means moving it to "Decided" with a D-number and moving the task in
TASKS.md to "Ready".

### Q-04 Should user config exist at all in v1?
CLAUDE.md sketches `~/.config/hx-ready/config.yaml` listing languages for
`doctor`. With D-006 the default candidate rule may be good enough.

Proposed: skip until someone asks. Blocks T-12.

### Q-08 Release binaries
D-016 rules out downloading GitHub release binaries in the MVP. jdtls,
OmniSharp, netcoredbg, marksman and helm_ls have no other channel on
Fedora. Options: (a) keep them out and let `check` point at the project
URL, (b) a `download:` step type with a pinned URL and sha256 per recipe,
(c) wait for Fedora/COPR packaging. Proposed: (a) for now, revisit after
Ubuntu lands. Blocks T-11, T-16.

## Answered questions

Kept for the reasoning; see the D-number named in each.

### Q-01 Which install channels may a recipe use?
Fedora packages cover go, rust, bash and hcl. Everything else needs npm
(yaml, typescript, json, dockerfile), pip/pipx (ty, jedi), or a GitHub
release binary (marksman, helm_ls, jdtls). Note that linux-bootstrap runs
`cargo install marksman`, which cannot work: marksman is an F# program
distributed as prebuilt binaries.

Proposed: allow `npm install -g`, `pipx install`, `go install` and
`cargo install` as recipe commands with an explicit `needs` on the
toolchain. Do not download release binaries in the MVP; that is where the
tool starts becoming a package manager. Answered by D-016.

### Q-02 What does "ready" mean when Helix lists alternative servers?
Python lists ty, ruff, jedi and pylsp. Installing all four is wasteful and
ty overlaps pylsp. Options: (a) ready means everything Helix lists,
(b) the recipe's `requires` is the target and other Helix-listed tools are
shown as "not covered", (c) leave it to the user's languages.toml.

Proposed: (b), and print the uncovered tools so nothing is hidden. This
also settles whether `install` may skip tools the recipe does not know.
Answered by D-017.

### Q-03 Are toolchain prerequisites in scope?
jdtls needs a JDK, OmniSharp needs the .NET SDK, typescript-language-server
needs Node. Should a recipe install the runtime too (`dnf install
java-21-openjdk dotnet-sdk-9.0 nodejs`), or stop with "install X first"?

Proposed: recipes may list runtimes as ordinary packages when Fedora
packages them; otherwise stop with an actionable message. Answered by D-018.

### Q-05 Which second platform?
Ubuntu/Debian, Arch, or macOS. Proposed: decide after T-07 based on which
machine you actually use next. Answered by D-019.

### Q-06 Formatters
Helix configures no formatter by default for any MVP language. Installing
gofumpt or black without configuring Helix to use it does nothing.

Proposed: only install formatters that Helix reports as missing, which
means only when the user has configured one. Answered by D-020.

### Q-07 terraform-ls is not a Fedora package
`sudo dnf install terraform-ls` fails on a clean Fedora 43 ("No match for
argument"). The copy on the workstation comes from the HashiCorp yum
repository (`rpm -qi terraform-ls` says Packager: HashiCorp; the `hashicorp`
repo is enabled). Options: (a) a recipe step that adds the HashiCorp repo
and its GPG key, then `dnf install terraform-ls`; (b) `go install
github.com/hashicorp/terraform-ls@latest` into `~/.local/bin`, needing the
golang package; (c) no recipe, `check terraform` keeps pointing at
`dnf search terraform-ls`.

Adding a third-party repository is a privileged, persistent system change
that outlives the tool, which is why this is not decided here. Proposed:
(b), since it reuses the go-install channel the go recipe already relies
on and touches nothing outside `~/.local/bin`. Answered by D-021.
