# hx-ready

`hx-ready` is a small CLI tool for preparing Helix Editor for a programming language.

Helix already knows about many languages, language servers, formatters, debug adapters, syntax grammars, text objects, and indentation rules. The missing piece is usually external tooling.

The goal of `hx-ready` is to answer:

> “I want to use Helix for language X. What am I missing, and how do I install it?”

The tool should inspect Helix's current language health, compare it with known installation recipes, optionally install the missing tooling, and verify the result.

## Primary UX

The intended commands are:

```bash
hx-ready check go
hx-ready install go

hx-ready check terraform
hx-ready install terraform

hx-ready doctor
```

Example:

```text
$ hx-ready check go

Go

Language servers
  ✘ gopls
  ✘ golangci-lint-lsp

Debug adapter
  ✘ dlv

Formatter
  ✓ built-in / none required

Suggested installation

  sudo dnf install golang gopls delve golangci-lint
  GOBIN=~/.local/bin go install github.com/nametake/golangci-lint-langserver@latest

Verify

  hx --health go
```

After installation:

```text
$ hx-ready check go

Go

  ✓ gopls
  ✓ golangci-lint-lsp
  ✓ dlv
  ✓ highlighting
  ✓ textobjects
  ✓ indentation

Ready.
```

## MVP scope

Do not attempt to support every language Helix knows about.

Start with languages that are useful on a developer/platform-engineering workstation:

* bash
* python
* go
* rust
* javascript
* typescript
* json
* yaml
* hcl
* terraform
* markdown
* dockerfile
* helm
* java
* c-sharp

Unknown languages should still be useful.

For example:

```text
$ hx-ready check ocaml

Helix expects:

  language server: ocamllsp

No hx-ready installation recipe exists yet.

Try:
  dnf search ocamllsp

Helix:
  hx --health ocaml
```

## Initial platform support

Start with:

```text
Fedora Linux
```

Do not prematurely build a generic cross-platform installer abstraction.

However, structure the data model so additional platforms can be added later without rewriting the core logic.

Likely future targets:

* Ubuntu/Debian
* Arch Linux
* macOS/Homebrew

## Implementation language

Use Go.

Reasons:

* simple static binary
* easy subprocess execution
* good YAML/JSON support
* straightforward testing
* appropriate for a developer CLI
* easy distribution later

Prefer the standard library unless a dependency clearly reduces complexity.

Reasonable dependencies include:

* Cobra for CLI handling, if useful
* a YAML library for language recipe files

Do not introduce a framework merely because the Go ecosystem has successfully produced one.

## Core architecture

Keep the design small.

Suggested structure:

```text
hx-ready/
├── cmd/
│   ├── root.go
│   ├── check.go
│   ├── install.go
│   └── doctor.go
├── internal/
│   ├── helix/
│   │   ├── health.go
│   │   └── parser.go
│   ├── recipes/
│   │   ├── loader.go
│   │   └── recipe.go
│   └── installer/
│       └── fedora.go
├── languages/
│   ├── go.yaml
│   ├── rust.yaml
│   ├── python.yaml
│   ├── yaml.yaml
│   └── terraform.yaml
├── main.go
├── go.mod
└── AGENTS.md
```

Do not treat this layout as sacred. Prefer fewer packages until separation is actually useful.

## Helix integration

The primary source of truth for the machine's current state is:

```bash
hx --health <language>
```

The tool should execute this command rather than attempt to reimplement Helix's configuration resolution.

It should parse at least:

* language server status
* debug adapter status
* formatter status
* highlighting
* textobjects
* indentation

The full:

```bash
hx --health
```

output can later power `hx-ready doctor`.

Do not depend on ANSI color codes being enabled.

Where possible, run Helix with output suitable for parsing or strip ANSI codes robustly.

## Recipe model

Installation knowledge should primarily be data-driven.

Example:

```yaml
language: go
display_name: Go

fedora:
  packages:
    - golang
    - gopls
    - delve
    - golangci-lint

  commands:
    - provides:
        - golangci-lint-lsp
      run: GOBIN=${HOME}/.local/bin go install github.com/nametake/golangci-lint-langserver@latest
```

A more detailed recipe may eventually support:

```yaml
language: terraform
display_name: Terraform

requires:
  language_servers:
    - terraform-ls

fedora:
  packages:
    - terraform-ls
```

Keep recipes declarative where possible.

Do not encode every language installation as custom Go code.

## Capability mapping

Recipes should map installed things to the names Helix reports.

For example:

```text
Fedora package:
  delve

Helix executable:
  dlv
```

and:

```text
Go module:
  github.com/nametake/golangci-lint-langserver

Helix executable:
  golangci-lint-langserver
```

The recipe format needs to distinguish between:

* package name
* installed executable / capability name
* shell installation command

## check command

```bash
hx-ready check <language>
```

Responsibilities:

1. Validate that `hx` exists.
2. Run `hx --health <language>`.
3. Parse the result.
4. Show:

   * working capabilities
   * missing capabilities
   * whether an installation recipe exists
5. Show the installation plan without changing the system.

The command must be safe and read-only.

## install command

```bash
hx-ready install <language>
```

Responsibilities:

1. Run the same health check as `check`.
2. Determine what is missing.
3. Load the relevant Fedora recipe.
4. Show exactly what will be installed.
5. Ask for confirmation before executing privileged commands.
6. Install only missing components where practical.
7. Run `hx --health <language>` again.
8. Show the final result.

Do not silently install packages.

Add a future-friendly noninteractive mode such as:

```bash
hx-ready install go --yes
```

but it does not need to exist in the first implementation.

## doctor command

`doctor` is useful, but it is not the first milestone.

Eventually:

```bash
hx-ready doctor
```

should parse:

```bash
hx --health
```

and show a concise summary rather than Helix's enormous table.

Example:

```text
Ready

  ✓ bash
  ✓ python
  ✓ elixir
  ✓ zig

Partially configured

  ⚠ go
      missing gopls
      missing dlv
      missing golangci-lint-lsp

  ⚠ hcl
      missing terraform-ls

  ⚠ yaml
      missing yaml-language-server

  ⚠ markdown
      missing marksman
```

Do not report hundreds of unsupported languages by default.

A language should appear in `doctor` when one of the following is true:

* it has an hx-ready recipe
* some of its external tooling is already installed
* it is explicitly requested through configuration

Later there may be flags such as:

```bash
hx-ready doctor --all
```

## Configuration

Avoid configuration until it is actually needed.

A future user config may live at:

```text
~/.config/hx-ready/config.yaml
```

Possible future contents:

```yaml
languages:
  - go
  - python
  - yaml
  - terraform
  - markdown
```

This could define what `doctor` considers relevant.

Do not implement this before the basic `check` workflow works.

## Safety

Installation commands may involve `sudo`.

Rules:

* always display commands before execution
* do not run arbitrary commands derived from Helix output
* commands must originate from trusted bundled recipes
* never interpolate untrusted language names directly into shell commands
* prefer `exec.Command` argument arrays over `sh -c`
* if shell execution is unavoidable for a recipe, make that explicit in the recipe model
* never modify Helix config automatically in the MVP

The first version should install external dependencies only.

## Error handling

Errors should be actionable.

Bad:

```text
failed
```

Good:

```text
Could not run Helix.

Expected executable:
  hx

Install Helix first or ensure it is on PATH.
```

Another example:

```text
Helix knows about language "foo", but hx-ready has no Fedora installation recipe for it.

Helix reports these missing tools:
  foo-lsp
  foo-fmt

Try:
  dnf search foo-lsp
  dnf search foo-fmt
```

## Output style

Keep terminal output compact.

Prefer:

```text
✓ installed
✘ missing
⚠ partial
```

Avoid excessive banners, boxes, ASCII logos, or animated progress.

This is a developer tool, not an airport departure board.

Use color when attached to a terminal, but output should remain understandable without color.

## Testing

Core parsing logic must be unit tested using captured `hx --health` fixtures.

Do not require a real Helix installation for parser tests.

Suggested fixtures:

```text
testdata/
├── health-go-missing.txt
├── health-go-ready.txt
├── health-python-ready.txt
└── health-terraform-missing.txt
```

Test at least:

* all capabilities present
* one missing language server
* multiple language servers
* missing debugger
* missing formatter
* `None`
* ANSI-colored input
* unknown/unexpected health output

Installer logic should be testable without actually invoking `sudo dnf`.

Use an execution abstraction or dry-run mechanism rather than mocking half the operating system.

## Dry run

A dry-run mode will be useful very early:

```bash
hx-ready install go --dry-run
```

Example:

```text
Would run:

  sudo dnf install golang gopls delve golangci-lint

  GOBIN=/home/user/.local/bin \
    go install github.com/nametake/golangci-lint-langserver@latest
```

This is also useful for testing.

## Relationship to linux-bootstrap

The existing `linux-bootstrap` repository already contains working Fedora recipes for several Helix environments:

* Python
* Go
* Rust
* Zig
* Crystal
* Kubernetes tooling

Use those scripts as seed knowledge when writing hx-ready recipes.

Do not make hx-ready depend on linux-bootstrap at runtime.

Long term, the two projects might share recipe metadata, but keep them independent until there is a clear reason to couple them.

## First milestone

The first milestone is deliberately tiny.

Implement:

```bash
hx-ready check go
```

It should:

1. run `hx --health go`
2. parse the result
3. detect:

   * `gopls`
   * `golangci-lint-lsp`
   * `dlv`
4. print which are present and which are missing
5. load a Go Fedora recipe
6. print the suggested installation commands

It should not install anything yet.

Definition of done:

```bash
go test ./...
go vet ./...
hx-ready check go
```

works on a real Fedora workstation.

## Second milestone

Add:

```bash
hx-ready install go --dry-run
```

Then add real installation after the dry-run path is solid.

Only after Go works end-to-end should additional languages be added.

## Design principle

The project should remain boring.

Helix already does the hard part of knowing what language tooling it wants.

`hx-ready` should merely bridge:

```text
Helix expects X
        ↓
X is missing
        ↓
On Fedora, install Y
        ↓
Verify with Helix
```

If the implementation starts to resemble a package manager, language server registry, or generic environment manager, the scope has escaped and should be cut back.

## Working on this repo

- Pick the top "Ready" task in [docs/TASKS.md](docs/TASKS.md). One task is one PR. Do not bundle tasks.
- If a task needs a decision this document does not settle, do not guess. Add a question to [docs/DECISIONS.md](docs/DECISIONS.md), mark the task blocked, and take the next one.
- When a decision is made, record it as a D-number and reference it from code comments and the task.
- Before opening a PR: `just gate` must pass. It runs gofmt (failing on unformatted files), `go vet`, `go test` and `go build` across the module.
- Parser changes need a fixture in `internal/helix/testdata/`. Real captures are preferred; hand-written ones get a `-synthetic` suffix.
