#!/usr/bin/env bash
# Build release packages into dist/: one tar.gz per target with the static
# binary and README, plus SHA256SUMS covering all of them. Used by the
# release workflow and by "just release <version>"; runs anywhere Go does
# and touches nothing outside dist/.
#
#   scripts/release.sh v1.2.3
set -euo pipefail

VERSION="${1:?usage: scripts/release.sh <version>}"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$REPO_DIR/dist"
MODULE="github.com/Lillevang/hx-ready"
TARGETS="linux/amd64 linux/arm64"

rm -rf "$DIST"
mkdir -p "$DIST"

for target in $TARGETS; do
  os="${target%/*}"
  arch="${target#*/}"
  name="hx-ready_${VERSION}_${os}_${arch}"
  stage="$DIST/$name"
  mkdir -p "$stage"

  echo "building $name"
  (cd "$REPO_DIR" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X ${MODULE}/cmd.Version=${VERSION}" \
    -o "$stage/hx-ready" .)

  cp "$REPO_DIR/README.md" "$stage/"
  [ -f "$REPO_DIR/LICENSE" ] && cp "$REPO_DIR/LICENSE" "$stage/"

  # Reproducible archive: fixed owner and mtime, sorted entries.
  tar -C "$DIST" --owner=0 --group=0 --numeric-owner --sort=name \
    --mtime="@${SOURCE_DATE_EPOCH:-0}" -czf "$DIST/$name.tar.gz" "$name"
  rm -rf "$stage"
done

(cd "$DIST" && sha256sum -- *.tar.gz > SHA256SUMS)

echo
echo "dist/"
(cd "$DIST" && command ls -1 && echo && cat SHA256SUMS)
