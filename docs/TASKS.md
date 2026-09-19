# Tasks

One task = one PR. Work top to bottom. A task moves to "Blocked" when it
needs an answer that is not derivable from CLAUDE.md or the code; the
question lives in [DECISIONS.md](DECISIONS.md) under "Open questions", and
answering it moves the task back up.

Definition of done for every PR: `just gate` passes (gofmt, vet, test, build),
and any new behaviour has a fixture-driven test.

## Ready

### T-03 No-recipe and wrong-name paths
`check ocaml` prints what Helix expects, says no recipe exists, and
suggests `dnf search <binary>` per missing tool. `check terraform`
surfaces Helix's own "Did you mean" line (Helix has no `terraform`
language; it is `hcl`). Add recipe aliases per D-009 so `terraform`
resolves to `hcl` once that recipe exists. Depends on T-02.

### T-04 Install plan and `--dry-run` (milestone 2a)
Implement `installer.Fedora`: group packages into one `sudo dnf install`
step, one step per command, order by `needs`, expand `${HOME}` in env,
drop steps that provide nothing from the missing list. Implement `DryRun`
rendering ("Would run:") and wire `install --dry-run`. Tests use the
`Executor` interface; nothing touches the system. Depends on T-02.

### T-05 Real install (milestone 2b)
Confirmation prompt before privileged steps (`--yes` skips it), `Exec`
executor streaming output, Fedora detection (D-008), re-run health
afterwards and print the final `check` view, PATH hint when a tool was
installed to `~/.local/bin` but Helix still cannot see it (D-007).
Depends on T-04.

### T-06 Recipes wave 1: Fedora-packaged languages
Add `rust.yaml` (rust-analyzer, lldb for lldb-dap), `bash.yaml`
(nodejs-bash-language-server), `hcl.yaml` (terraform-ls, alias
`terraform`). All are plain dnf packages, verified available on Fedora 43.
Add a test asserting that every `requires` entry is provided by some
package or command in the same recipe. Can run in parallel with T-04.

### T-07 `doctor`
Candidate languages per D-006, one `hx --health <lang>` per candidate,
grouped output (Ready / Partially configured) per CLAUDE.md, `--all` flag.
Fixture: `health-all-languages-table.txt`. Depends on T-01 and T-06.

### T-08 CI and release
GitHub Actions running test, vet and gofmt on push and PR; tagged
releases building a static Linux binary with `-ldflags -X
.../cmd.Version`. Needs the git repository to exist first.

## Blocked

### T-09 Recipes wave 2: tools Fedora does not package
yaml-language-server, typescript-language-server (javascript, typescript),
vscode-json-language-server (json), docker-langserver (dockerfile) are npm
packages. marksman (markdown) and helm_ls (helm) ship as GitHub release
binaries. Blocked on Q-01 (which install channels are allowed).

### T-10 Python recipe
Helix lists four servers (ty, ruff, jedi, pylsp). ruff and pylsp are dnf
packages; ty and jedi-language-server are pip/pipx. Blocked on Q-02
(what "ready" means when Helix lists alternatives) and Q-01.

### T-11 Java and C# recipes
jdtls, OmniSharp and netcoredbg are not packaged; they need a JDK or .NET
SDK plus a download step. Blocked on Q-01 and Q-03.

### T-12 User config (`~/.config/hx-ready/config.yaml`)
CLAUDE.md says not before `check` works. Blocked on T-07 landing and Q-04.

### T-13 Second platform
Blocked on Q-05 (Ubuntu, Arch or macOS first) and on Fedora being solid
through T-07.

### T-14 Formatters
Every MVP language reports formatter `None` with default Helix config.
Blocked on Q-06 (install formatters Helix has not been configured to use?).

## Done

### T-01 Health parser
Implement `helix.Parse` in `internal/helix/parser.go` against the captured
fixtures in `internal/helix/testdata/`.

Must handle: several language servers, found and missing; debug adapter
found (`✓ /path`), missing (`✘ 'dlv' not found in $PATH`), empty name
(`✘ '' not found in $PATH`, see javascript fixture) and `None`; formatter
`None`; the four tree-sitter lines; `Language 'x' not found` →
`ErrUnknownLanguage` (Helix exits 0 for this, so it must be detected from
text); garbage → `ErrUnexpectedOutput`. Also add hand-written fixtures for
a found and a missing formatter, since no default Helix config produces one.

Done when every fixture has a table-driven test and `Missing()` returns the
right binaries for each.
Done 2026-09-19. Adds D-015 (empty command names are not missing
executables) and three `-synthetic` fixtures.

### T-02 `check go` end to end (milestone 1)
Wire `ExecRunner` → `Parse` → `recipes.Load` in `cmd/check.go` and print
the layout from CLAUDE.md. Actionable errors for: hx not on PATH, unknown
language, health command failure. Exit 0 when ready, 1 when not.
Introduce a small output helper (✓ ✘ ⚠, colour only on a TTY) that later
commands reuse. Depends on T-01.
Done 2026-09-19. Unknown-language errors already surface Helix's "Did you
mean" suggestions; T-03 adds aliases and the no-recipe hints.
