# Quality gate for hx-ready. Every task/PR must pass `just gate`.
# See CLAUDE.md "Working on this repo" and docs/TASKS.md "Definition of done".

set shell := ["bash", "-euo", "pipefail", "-c"]

# Run the full gate (default).
default: gate

# fmt + vet + test + build. Fails on the first problem.
gate: fmt vet test build
    @echo "gate: ok"

# Fail if any file is not gofmt-clean. Lists the offenders.
fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    out="$(gofmt -l .)"
    if [ -n "$out" ]; then
        echo "gofmt: unformatted files:" >&2
        echo "$out" >&2
        echo "run: just fmt-fix" >&2
        exit 1
    fi

# Rewrite files in place with gofmt.
fmt-fix:
    gofmt -w .

vet:
    go vet ./...

test:
    go test ./...

# Compile everything without producing a binary.
build:
    go build ./...

# Build the CLI into ./bin/hx-ready.
bin:
    mkdir -p bin
    go build -o bin/hx-ready .
