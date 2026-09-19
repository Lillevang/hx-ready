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
expect_contains 'Helix does not know language "nosuchlang"'

# T-03: a language Helix knows but hx-ready has no recipe for.
step "T-03: check ocaml has no recipe"
expect_exit 1 hx-ready check ocaml
expect_contains "No hx-ready installation recipe exists yet"
expect_contains "dnf search ocamllsp"

# T-03/T-06: terraform is not a Helix language; the hcl recipe's alias is.
step "T-03: check terraform resolves to hcl"
expect_exit 1 hx-ready check terraform
expect_contains '"terraform" is Helix'"'"'s "hcl" language; checking hcl.'
expect_contains "sudo dnf install -y terraform-ls"

# T-04: dry run prints the plan and changes nothing.
step "T-04: install go --dry-run"
expect_exit 0 hx-ready install go --dry-run
expect_contains "Would run:"
expect_contains "sudo dnf install -y golang gopls delve golangci-lint"
expect_contains "go install github.com/nametake/golangci-lint-langserver@latest"
if command -v gopls >/dev/null; then
  echo "FAIL: dry run installed gopls"; fail=1
else
  echo "ok:   gopls still absent after dry run"
fi

# T-05: declining the prompt installs nothing.
step "T-05: install go, answer n"
expect_exit 1 bash -c 'echo n | hx-ready install go'
expect_contains "Will run:"
expect_contains "Proceed? [y/N]"
expect_contains "Aborted. Nothing was installed."
if command -v gopls >/dev/null; then
  echo "FAIL: declined install still installed gopls"; fail=1
else
  echo "ok:   gopls still absent after declining"
fi

# T-05: the real thing. dnf plus a go install from the network.
step "T-05: install go --yes"
expect_exit 0 hx-ready install go --yes
expect_contains "\$ sudo dnf install -y golang gopls delve golangci-lint"
expect_contains "\$ GOBIN=/home/test/.local/bin"
expect_contains "Verifying with hx --health go"
expect_contains "Ready."
expect_lacks "Proceed?"

step "T-05: tools are really there"
expect_exit 0 bash -c 'command -v gopls && command -v dlv && command -v golangci-lint && command -v golangci-lint-langserver'
expect_exit 0 hx-ready check go
expect_contains "Ready."
expect_exit 0 bash -c '! hx --health go | grep -q ✘'

step "T-05: second install is a no-op"
expect_exit 0 hx-ready install go --yes
expect_contains "Nothing to install."
expect_lacks "\$ sudo"

# D-007: with ~/.local/bin off PATH, Helix cannot see the langserver and
# check says why.
step "T-05: PATH hint when ~/.local/bin is not on PATH"
expect_exit 1 env PATH=/usr/local/bin:/usr/bin:/bin "$HOME/.local/bin/hx-ready" check go
expect_contains "golangci-lint-langserver exists in /home/test/.local/bin"
expect_contains 'export PATH="$HOME/.local/bin:$PATH"'

# T-06: recipes wave 1, all plain dnf packages.
for lang in bash hcl rust; do
  step "T-06: install $lang --yes"
  expect_exit 0 hx-ready install "$lang" --yes
  expect_contains "Ready."
  expect_exit 0 bash -c "! hx --health $lang | grep -q ✘"
done

# T-07: doctor after installing go, bash, hcl and rust.
step "T-07: doctor"
expect_exit 0 hx-ready doctor
expect_contains "Ready"
expect_contains "✓ bash"
expect_contains "✓ go"
expect_contains "✓ hcl"
expect_contains "✓ rust"
expect_lacks "Could not check"

step "T-07: doctor --all lists many languages"
expect_exit 1 hx-ready doctor --all
expect_contains "⚠ python"
expect_contains "missing"

echo
if [ "$fail" -eq 0 ]; then
  echo "scenario: PASS"
else
  echo "scenario: FAIL"
fi
exit "$fail"
