# Tasks

One task = one PR. Work top to bottom. A task moves to "Blocked" when it
needs an answer that is not derivable from CLAUDE.md or the code; the
question lives in [DECISIONS.md](DECISIONS.md) under "Open questions", and
answering it moves the task back up.

Definition of done for every PR: `just gate` passes (gofmt, vet, test, build),
and any new behaviour has a fixture-driven test.

## Ready

Nothing at the moment; see Roadmap and Blocked.

## Roadmap

Not scheduled; recorded so they are not forgotten.

* **Release-binary downloads** (D-023). A `download:` step type with a
  pinned URL, checksum and target in `~/.local/bin`, which would unblock
  T-11 (java, c-sharp) and T-16 (markdown, helm).
* **User config** (D-022). `~/.config/hx-ready/config.yaml` listing the
  languages `doctor` should consider (T-12).
* **Tree-sitter query gaps** (distant future). Helix ships no
  `textobjects.scm` or `indents.scm` for astro, html or vue, so `check`
  shows ⚠ under "Editor support". Fixing that means query files in
  `~/.config/helix/runtime/queries/<lang>/` or an upstream Helix
  contribution, neither of which hx-ready does today (it never writes to
  Helix's config or runtime). Noted 2026-09-20; not planned.

## Blocked

### T-11 Java and C# recipes
Needs the release-download step from the roadmap (D-023).

### T-16 markdown and helm recipes
Needs the release-download step from the roadmap (D-023).

### T-12 User config
Deferred by D-022; see the roadmap.

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

### T-03 No-recipe and wrong-name paths
`check ocaml` prints what Helix expects, says no recipe exists, and
suggests `dnf search <binary>` per missing tool. `check terraform`
surfaces Helix's own "Did you mean" line (Helix has no `terraform`
language; it is `hcl`). Add recipe aliases per D-009 so `terraform`
resolves to `hcl` once that recipe exists. Depends on T-02.
Done 2026-09-19. Captured `health-terraform-unknown.txt`: Helix suggests
only t-languages, not hcl, so the alias (not the suggestion) is what makes
`check terraform` work once T-06 lands.

### T-04 Install plan and `--dry-run` (milestone 2a)
Implement `installer.Fedora`: group packages into one `sudo dnf install`
step, one step per command, order by `needs`, expand `${HOME}` in env,
drop steps that provide nothing from the missing list. Implement `DryRun`
rendering ("Would run:") and wire `install --dry-run`. Tests use the
`Executor` interface; nothing touches the system. Depends on T-02.
Done 2026-09-19. `check` renders the same filtered plan, and both list
tools the recipe cannot provide under "Not covered by the recipe". The go
recipe now declares `needs: [go, golangci-lint]` so those packages stay in
the plan although Helix never reports them missing.

### T-05 Real install (milestone 2b)
Confirmation prompt before privileged steps (`--yes` skips it), `Exec`
executor streaming output, Fedora detection (D-008), re-run health
afterwards and print the final `check` view, PATH hint when a tool was
installed to `~/.local/bin` but Helix still cannot see it (D-007).
Depends on T-04.
Done 2026-09-19. The PATH hint lives in the `check` view so it shows both
after an install and on a plain `check`. Verified end to end with `just vm`.

### T-06 Recipes wave 1: Fedora-packaged languages
Add `rust.yaml` (rust-analyzer, lldb for lldb-dap), `bash.yaml`
(nodejs-bash-language-server), `hcl.yaml` (terraform-ls, alias
`terraform`). All are plain dnf packages, verified available on Fedora 43.
Add a test asserting that every `requires` entry is provided by some
package or command in the same recipe. Can run in parallel with T-04.
Done 2026-09-19 for rust and bash, verified by installing them in the
`just vm` scenario. hcl was pulled out: `terraform-ls` is not in Fedora's
repositories (dnf: "No match for argument"), it comes from HashiCorp's own
yum repo. Split into T-15, blocked on Q-07.

### T-07 `doctor`
Candidate languages per D-006, one `hx --health <lang>` per candidate,
grouped output (Ready / Partially configured) per CLAUDE.md, `--all` flag.
Fixture: `health-all-languages-table.txt`. Depends on T-01 and T-06.
Done 2026-09-19. The wide table does list every language server via
continuation rows (D-006 assumed only the first), but names can be truncated
and paths are absent, so the per-language check is still needed. Helix
honours COLUMNS only when TERM is set, so ExecRunner supplies TERM=dumb in
tty-less sessions; the parser also copes with an 80-column table
(`health-all-languages-table-narrow.txt`, a real capture).

### T-08 CI and release
GitHub Actions running test, vet and gofmt on push and PR; tagged
releases building a static Linux binary with `-ldflags -X
.../cmd.Version`. Needs the git repository to exist first.
Done 2026-09-19. `ci.yml` runs `just gate`; `release.yml` builds a static
linux/amd64 binary on `v*` tags. Not exercised until the repo has a GitHub
remote.

### T-14 Formatters
Every MVP language reports formatter `None` with default Helix config.
Blocked on Q-06 (install formatters Helix has not been configured to use?).
Closed 2026-09-20 by D-020: nothing to build. `Missing()` already includes a
formatter Helix reports missing, and a recipe can provide one; none is
required by default.

### T-10 Python recipe and recipe-driven readiness
Implement D-017: when a recipe exists, `check`, `install` and `doctor`
judge readiness by `requires`; other tools Helix lists are shown as
"⚠ not covered by the recipe". Then `python.yaml` requiring `ruff` and
`pylsp` (Fedora: `ruff`, `python3-lsp-server`), matching linux-bootstrap.
ty and jedi are left uncovered.
Done 2026-09-20. `cmd/verdict.go` holds the D-017 logic; check, install and
doctor all use it.

### T-09 Recipes wave 2: npm packages
yaml-language-server (yaml), typescript-language-server (typescript and
javascript), vscode-json-language-server (json, npm package
vscode-langservers-extracted), docker-langserver (dockerfile, npm package
dockerfile-language-server-nodejs). Commands use `npm install -g` with
`npm_config_prefix=${HOME}/.local` so binaries land in `~/.local/bin`
(D-007, D-016); `needs: [npm]` pulls in `nodejs-npm`. Verify in `just vm`.
Done 2026-09-20. Real fixtures captured for typescript, json and
dockerfile. Verified in `just vm`.

### T-15 hcl recipe (terraform-ls)
`hcl.yaml` with `aliases: [terraform]` and `go install
github.com/hashicorp/terraform-ls@latest` per D-021. The alias machinery
and the `health-hcl-missing-synthetic` fixture exist.
Done 2026-09-20. Verified in `just vm`.

### T-13 Ubuntu/Debian platform
Per D-019: `debian` block in the recipe model, `installer.Debian` using
`apt-get install -y`, platform detection from `/etc/os-release` replacing
the Fedora-only check, `debian` blocks for every bundled recipe, and an
Ubuntu cloud image option in `test/vm.sh` (`DISTRO=ubuntu just vm`).
Package names count as verified only once the VM scenario installs them.
Done 2026-09-20. Verified with `just vm-ubuntu` on Ubuntu 24.04, which
found three gaps in the archive: golangci-lint and ruff are built via
`go install` and `pipx` instead; rust-analyzer has no allowed channel, so
rust has no debian block (Q-09, listed as a known gap in the recipe test).
The plan builder now pulls in commands another command needs, not only
packages. The Ubuntu VM installs Helix from the release tarball because
Ubuntu does not package it.

### T-17 rust on Ubuntu/Debian
Per D-024: `debian` block in `rust.yaml` with `rustup` and `lldb`
packages, `rustup default stable` then `rustup component add
rust-analyzer`; remove `rust/debian` from the known gaps in
`internal/recipes/loader_test.go`; extend the D-007 PATH hint to
`~/.cargo/bin`. Verify with `just vm-ubuntu`.
Done 2026-09-20. Ubuntu's lldb package ships only `lldb-dap-18`, so the
block adds a user-level symlink in `~/.local/bin` via the `shell:` escape
hatch (a glob keeps it LLVM-version independent). Verified with
`just vm-ubuntu`. `INSIDE='cmd' bash test/vm.sh` was added for this kind
of exploration.

### T-18 Astro recipe
Added 2026-09-20 on request, outside the MVP list: `astro-ls` from the
`@astrojs/language-server` npm package, plus typescript. Real fixture
captured; verified in `just vm`.

### T-19 Release packaging
Replaces T-08's single-binary release. `scripts/release.sh` builds
`hx-ready_<version>_linux_{amd64,arm64}.tar.gz` (static binary + README,
reproducible tar) and `SHA256SUMS`; the release workflow runs the gate
first, verifies the checksums and the amd64 binary, attaches a GitHub
build-provenance attestation, and creates the release with generated
notes. `just release <version>` runs the same script locally. Done
2026-09-20. No LICENSE file exists yet; the script includes one when it
appears.
