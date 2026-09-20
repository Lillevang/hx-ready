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

# Distro-specific values. The scenario is the same; only package manager
# strings and package names differ (see languages/*.yaml).
. /etc/os-release
case "$ID" in
  fedora)
    PM_INSTALL="sudo dnf install -y"
    PM_SEARCH="dnf search"
    GO_PKGS="golang gopls delve golangci-lint"
    PY_PKGS="ruff python3-lsp-server"
    RUST_ON_THIS_DISTRO=1
    ;;
  ubuntu|debian)
    PM_INSTALL="sudo apt-get install -y"
    PM_SEARCH="apt-cache search"
    GO_PKGS="golang-go gopls delve"
    PY_PKGS="pipx python3-pylsp"
    RUST_ON_THIS_DISTRO=0   # Q-09
    ;;
  *) echo "unsupported VM distro: $ID"; exit 2 ;;
esac

step "setup: install helix ($ID)"
case "$ID" in
  fedora)
    sudo dnf install -y -q helix >/dev/null
    ;;
  ubuntu)
    # Helix is not in Ubuntu's archive. Use the official release tarball,
    # pinned to the version Fedora 43 ships so both VMs test the same
    # Helix. Test setup only; hx-ready itself never downloads binaries.
    HX_VER=25.07.1
    sudo apt-get update -q >/dev/null
    curl -fsSL -o /tmp/helix.tar.xz "https://github.com/helix-editor/helix/releases/download/${HX_VER}/helix-${HX_VER}-x86_64-linux.tar.xz"
    sudo mkdir -p /opt && sudo tar -xJf /tmp/helix.tar.xz -C /opt
    sudo ln -sfn "/opt/helix-${HX_VER}-x86_64-linux" /opt/helix
    sudo ln -sf /opt/helix/hx /usr/local/bin/hx   # runtime/ sits next to hx
    ;;
esac
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
expect_contains "$PM_INSTALL $GO_PKGS"
expect_contains "hx --health go"

# T-02: unknown language.
step "T-02: check nosuchlang"
expect_exit 1 hx-ready check nosuchlang
expect_contains 'Helix does not know language "nosuchlang"'

# T-03: a language Helix knows but hx-ready has no recipe for.
step "T-03: check ocaml has no recipe"
expect_exit 1 hx-ready check ocaml
expect_contains "No hx-ready installation recipe exists yet"
expect_contains "$PM_SEARCH ocamllsp"

# T-03/T-15: terraform is not a Helix language; the hcl recipe's alias is.
step "T-03: check terraform resolves to hcl"
expect_exit 1 hx-ready check terraform
expect_contains '"terraform" is Helix'"'"'s "hcl" language; checking hcl.'
expect_contains "go install github.com/hashicorp/terraform-ls@latest"

# T-04: dry run prints the plan and changes nothing.
step "T-04: install go --dry-run"
expect_exit 0 hx-ready install go --dry-run
expect_contains "Would run:"
expect_contains "$PM_INSTALL $GO_PKGS"
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
expect_contains "\$ $PM_INSTALL $GO_PKGS"
expect_contains "\$ GOBIN=/home/test/.local/bin"
expect_contains "Verifying with hx --health go"
expect_contains "Ready."
expect_lacks "Proceed?"

step "T-05: tools are really there"
expect_exit 0 bash -c 'command -v gopls && command -v dlv && command -v golangci-lint && command -v golangci-lint-langserver'
expect_exit 0 hx-ready check go
expect_contains "Ready."
expect_exit 0 bash -c '! hx --health go | grep -q "not found"'

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

# T-06: recipes wave 1.
step "T-06: install bash --yes"
expect_exit 0 hx-ready install bash --yes
expect_contains "Ready."
expect_exit 0 hx-ready check bash
if [ "$RUST_ON_THIS_DISTRO" = 1 ]; then
  step "T-06: install rust --yes"
  expect_exit 0 hx-ready install rust --yes
  expect_contains "Ready."
  expect_exit 0 hx-ready check rust
else
  step "T-06: rust has no recipe block on $ID (Q-09)"
  expect_exit 1 hx-ready install rust --yes
  expect_contains "The Rust recipe has no Ubuntu/Debian block yet"
  expect_exit 1 hx-ready check rust
  expect_contains "$PM_SEARCH rust-analyzer"
fi

# T-09: npm recipes. Binaries land in ~/.local/bin via npm_config_prefix.
for lang in yaml typescript json dockerfile; do
  step "T-09: install $lang --yes"
  expect_exit 0 hx-ready install "$lang" --yes
  expect_contains "npm_config_prefix=/home/test/.local"
  expect_contains "Ready."
  expect_exit 0 hx-ready check "$lang"
done
# T-15: terraform-ls built from source into ~/.local/bin (D-021).
step "T-15: install terraform --yes (alias of hcl)"
expect_exit 0 hx-ready install terraform --yes
expect_contains "go install github.com/hashicorp/terraform-ls@latest"
expect_contains "Ready."
expect_exit 0 bash -c 'test -x ~/.local/bin/terraform-ls'
expect_exit 0 hx-ready check hcl

step "T-09: javascript shares typescript's server, so nothing to install"
expect_exit 0 hx-ready install javascript --yes
expect_contains "Nothing to install."
expect_exit 0 bash -c 'ls ~/.local/bin/yaml-language-server ~/.local/bin/typescript-language-server ~/.local/bin/vscode-json-language-server ~/.local/bin/docker-langserver'
expect_exit 0 bash -c '! command -v npm | grep -q local'   # npm itself came from the distro

# T-10: python requires only ruff and pylsp (D-017); ty and jedi stay
# uncovered and Helix keeps reporting them, which is expected.
step "T-10: install python --yes"
expect_exit 0 hx-ready install python --yes
expect_contains "\$ $PM_INSTALL $PY_PKGS"
expect_contains "ty (not covered by the recipe)"
expect_contains "Ready."
expect_exit 0 hx-ready check python
expect_exit 0 bash -c 'command -v ruff && command -v pylsp'

# T-07: doctor after installing go, bash, rust and python. lldb-dap from the rust
# recipe also makes c, cpp and zig candidates (D-006), and those miss
# their language servers, so the overall exit is 1.
step "T-07: doctor"
expect_exit 1 hx-ready doctor
expect_contains "Ready"
expect_contains "✓ bash"
expect_contains "✓ go"
expect_contains "✓ python"
expect_contains "✓ hcl"
expect_contains "✓ yaml"
if [ "$RUST_ON_THIS_DISTRO" = 1 ]; then
  expect_contains "✓ rust"
  expect_contains "⚠ c"          # lldb-dap from the rust recipe makes c a candidate
  expect_contains "missing clangd"
else
  expect_contains "⚠ rust"
  expect_contains "missing rust-analyzer"
fi
expect_lacks "Could not check"
expect_lacks "…"

step "T-07: doctor --all lists many languages"
expect_exit 1 hx-ready doctor --all
expect_contains "✓ python"
expect_contains "⚠ ada"
expect_contains "missing ada-language-server"

echo
if [ "$fail" -eq 0 ]; then
  echo "scenario: PASS"
else
  echo "scenario: FAIL"
fi
exit "$fail"
