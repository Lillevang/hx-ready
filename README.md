# hx-ready

Prepare [Helix](https://helix-editor.com) for a programming language.

Helix already knows which language servers, debug adapters and formatters
it wants. `hx-ready` runs `hx --health <language>`, reports what is
missing, and installs it from a bundled Fedora recipe.

```
hx-ready check go       # what is missing, and how to install it
hx-ready install go     # install it (asks first; --dry-run to only print)
hx-ready doctor         # summary across languages
```

## Status

Scaffold. Commands parse their arguments and exit with "not implemented".
See [docs/TASKS.md](docs/TASKS.md) for the plan and
[docs/DECISIONS.md](docs/DECISIONS.md) for the reasoning.

## Development

Requires Go 1.26. Fedora and a Helix install are needed only to run the
binary, not the tests.

```
go build ./...
go test ./...
go vet ./...
```

Design notes and scope live in [AGENTS.md](AGENTS.md) (CLAUDE.md is a symlink to it).
