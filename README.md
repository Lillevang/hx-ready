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

`check`, `install` (with `--dry-run` and `--yes`) and `doctor` work on
Fedora, with recipes for go, rust and bash. See
[docs/TASKS.md](docs/TASKS.md) for what is next and
[docs/DECISIONS.md](docs/DECISIONS.md) for the reasoning and open questions.

## Development

Requires Go 1.26 and [just](https://github.com/casey/just). Fedora and a
Helix install are needed only to run the binary, not the tests.

```
just gate        # gofmt, vet, test, build: must pass before every PR
just bin         # build ./bin/hx-ready
just vm          # end-to-end run in a throwaway Fedora VM (qemu+kvm)
```

Unit tests never touch the system: the parser runs on captured fixtures
and the installer runs against an executor interface. Anything that
really calls `hx`, `dnf` or `sudo` happens in the VM, never on the host.
`KEEP=1 just vm` leaves the VM up; `just vm-ssh` gets you in.

Design notes and scope live in [AGENTS.md](AGENTS.md) (CLAUDE.md is a symlink to it).
