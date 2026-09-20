# hx-ready

Prepare [Helix](https://helix-editor.com) for a programming language.

Helix already knows which language servers, debug adapters and formatters
it wants. `hx-ready` runs `hx --health <language>`, reports what is
missing, and installs it from a bundled recipe for Fedora or Ubuntu/Debian.

```
hx-ready check go       # what is missing, and how to install it
hx-ready install go     # install it (asks first; --dry-run to only print)
hx-ready doctor         # summary across languages
```

## Install

Releases ship a static Linux binary for amd64 and arm64 as
`hx-ready_<version>_linux_<arch>.tar.gz`, with a `SHA256SUMS` file and a
GitHub build-provenance attestation.

```
v=v0.1.0
curl -fsSLO "https://github.com/Lillevang/hx-ready/releases/download/$v/hx-ready_${v}_linux_amd64.tar.gz"
curl -fsSLO "https://github.com/Lillevang/hx-ready/releases/download/$v/SHA256SUMS"
sha256sum --ignore-missing -c SHA256SUMS
gh attestation verify "hx-ready_${v}_linux_amd64.tar.gz" --repo Lillevang/hx-ready   # optional
tar -xzf "hx-ready_${v}_linux_amd64.tar.gz"
install -m 0755 "hx-ready_${v}_linux_amd64/hx-ready" ~/.local/bin/
```

Or build from source with `just bin`.

## Status

`check`, `install` (with `--dry-run` and `--yes`) and `doctor` work on
Fedora and Ubuntu/Debian, with recipes for bash, dockerfile, go, hcl
(alias terraform), javascript, json, python, rust, typescript and yaml. See
[docs/TASKS.md](docs/TASKS.md) for what is next and
[docs/DECISIONS.md](docs/DECISIONS.md) for the reasoning and open questions.

## Development

Requires Go 1.26 and [just](https://github.com/casey/just). Fedora and a
Helix install are needed only to run the binary, not the tests.

```
just gate        # gofmt, vet, test, build: must pass before every PR
just bin         # build ./bin/hx-ready
just release v0.1.0   # build ./dist packages and SHA256SUMS as the release workflow does
just vm          # end-to-end run in a throwaway Fedora VM (qemu+kvm)
just vm-ubuntu   # the same scenario on Ubuntu 24.04
```

Unit tests never touch the system: the parser runs on captured fixtures
and the installer runs against an executor interface. Anything that
really calls `hx`, `dnf` or `sudo` happens in the VM, never on the host.
`KEEP=1 just vm` leaves the VM up; `just vm-ssh` gets you in.

Design notes and scope live in [AGENTS.md](AGENTS.md) (CLAUDE.md is a symlink to it).
