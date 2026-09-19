#!/usr/bin/env bash
# Runs inside the throwaway VM (see test/vm.sh). The user has passwordless
# sudo. ~/.local/bin/hx-ready is the binary built on the host.
#
# Each step prints the command output so a failing run can be read without
# ssh-ing in. Steps are grouped by the task in docs/TASKS.md that they cover.
set -u
export PATH="$HOME/.local/bin:$PATH"

fail=0
step() {
  echo
  echo "----- $1"
}
expect_exit() {
  # expect_exit <want> <cmd...>: run, print output, compare exit code.
  local want="$1"; shift
  local out code
  out="$("$@" 2>&1)"; code=$?
  printf '%s\n' "$out" | sed 's/^/    /'
  if [ "$code" -eq "$want" ]; then
    echo "ok:   exit $code"
  else
    echo "FAIL: exit $code, want $want"
    fail=1
  fi
  LAST_OUT="$out"
}
expect_contains() {
  # expect_contains <substring>: check the previous step's output.
  if printf '%s\n' "$LAST_OUT" | grep -qF -- "$1"; then
    echo "ok:   output contains '$1'"
  else
    echo "FAIL: output lacks '$1'"
    fail=1
  fi
}
expect_lacks() {
  if printf '%s\n' "$LAST_OUT" | grep -qF -- "$1"; then
    echo "FAIL: output contains '$1'"
    fail=1
  else
    echo "ok:   output lacks '$1'"
  fi
}

step "setup: install helix"
sudo dnf install -y -q helix >/dev/null
hx --version

step "hx-ready version"
expect_exit 0 hx-ready version
expect_contains "hx-ready"

# T-02: check go on a clean machine.
step "T-02: check go on a clean machine is not ready"
expect_exit 1 hx-ready check go
expect_contains "gopls"
expect_contains "golangci-lint-lsp"
expect_contains "dlv"
expect_contains "sudo dnf install"
expect_contains "hx --health go"

# T-02: unknown language.
step "T-02: check nosuchlang"
expect_exit 1 hx-ready check nosuchlang
expect_contains "not"

echo
if [ "$fail" -eq 0 ]; then
  echo "scenario: PASS"
else
  echo "scenario: FAIL"
fi
exit "$fail"
