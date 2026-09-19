#!/usr/bin/env bash
set -euo pipefail

# End-to-end test in a throwaway Fedora Cloud VM (qemu+kvm, no libvirt).
#
# Nothing hx-ready does to a system (dnf, sudo, go install, ~/.local/bin)
# ever runs on the host. This script builds the binary on the host, boots a
# fresh VM, copies the binary and test/vm/inside.sh in, runs the scenario,
# and destroys the VM. Modelled on linux-bootstrap/test/fedora_vm_test.sh.
#
#   just vm                       # full run, VM destroyed afterwards
#   KEEP=1 just vm                # leave the VM running for inspection
#   just vm-ssh                   # ssh into a VM kept with KEEP=1
#   just vm-stop                  # kill a kept VM
#
# The ~700MB cloud image is downloaded once and cached. Each run boots a
# fresh overlay, so the VM is always clean. cloud-init's seed is served
# from a tiny HTTP server on the host when no ISO tool is installed, so the
# host needs only qemu, kvm, python3, ssh and curl.

FEDORA_VER="43"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="$HOME/.cache/hx-ready-vmtest"
BASE_IMG="$CACHE_DIR/fedora-${FEDORA_VER}-cloud.qcow2"
# Reuse linux-bootstrap's download when it exists; it is only ever a
# read-only backing file.
ALT_BASE_IMG="$HOME/.cache/linux-bootstrap-vmtest/fedora-${FEDORA_VER}-cloud.qcow2"
WORK_IMG="$CACHE_DIR/work.qcow2"
SEED_DIR="$CACHE_DIR/seed"
SEED_ISO="$CACHE_DIR/seed.iso"
SSH_KEY="$CACHE_DIR/id_test"
PID_FILE="$CACHE_DIR/qemu.pid"
HTTP_PID_FILE="$CACHE_DIR/http.pid"
CONSOLE_LOG="$CACHE_DIR/console.log"
SSH_PORT="${SSH_PORT:-2223}"
HTTP_PORT="${HTTP_PORT:-8790}"
VM_USER="test"

SSH_OPTS=(
  -i "$SSH_KEY"
  -p "$SSH_PORT"
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR
)

vm_ssh() {
  # shellcheck disable=SC2029  # client-side expansion of $@ is intended
  ssh "${SSH_OPTS[@]}" "$VM_USER@127.0.0.1" "$@"
}

kill_pidfile() {
  if [ -f "$1" ] && kill -0 "$(cat "$1")" 2>/dev/null; then
    kill "$(cat "$1")" || true
  fi
  rm -f "$1"
}

vm_destroy() {
  kill_pidfile "$PID_FILE"
  kill_pidfile "$HTTP_PID_FILE"
}

case "${1:-}" in
  ssh) exec ssh "${SSH_OPTS[@]}" "$VM_USER@127.0.0.1" ;;
  stop) vm_destroy; echo "VM stopped"; exit 0 ;;
  "") ;;
  *) echo "usage: $0 [ssh|stop]" >&2; exit 2 ;;
esac

for tool in qemu-system-x86_64 qemu-img ssh ssh-keygen curl python3; do
  command -v "$tool" >/dev/null || { echo "missing on host: $tool" >&2; exit 1; }
done
[ -w /dev/kvm ] || { echo "/dev/kvm is not writable; kvm is required" >&2; exit 1; }

mkdir -p "$CACHE_DIR" "$SEED_DIR"

# --- build on the host (pure Go, no system impact) ---
echo "building hx-ready ..."
(cd "$REPO_DIR" && CGO_ENABLED=0 go build -o "$CACHE_DIR/hx-ready" .)

# --- fetch base image (cached across runs) ---
if [ ! -f "$BASE_IMG" ] && [ -f "$ALT_BASE_IMG" ]; then
  BASE_IMG="$ALT_BASE_IMG"
fi
if [ ! -f "$BASE_IMG" ]; then
  BASE_URL="https://download.fedoraproject.org/pub/fedora/linux/releases/${FEDORA_VER}/Cloud/x86_64/images"
  IMG_NAME="$(curl -fsSL "$BASE_URL/" | grep -oE 'Fedora-Cloud-Base-Generic[^"<]*\.qcow2' | head -1)"
  [ -n "$IMG_NAME" ] || {
    echo "could not resolve cloud image name from $BASE_URL" >&2
    exit 1
  }
  echo "downloading $IMG_NAME ..."
  curl -fL --progress-bar -o "$BASE_IMG.part" "$BASE_URL/$IMG_NAME"
  mv "$BASE_IMG.part" "$BASE_IMG"
fi

# --- ephemeral ssh key + cloud-init seed ---
[ -f "$SSH_KEY" ] || ssh-keygen -q -t ed25519 -N '' -f "$SSH_KEY"

cat >"$SEED_DIR/user-data" <<EOD
#cloud-config
users:
  - name: $VM_USER
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - $(cat "$SSH_KEY.pub")
EOD
cat >"$SEED_DIR/meta-data" <<EOD
instance-id: hx-ready-test
local-hostname: hx-ready-test
EOD
: >"$SEED_DIR/vendor-data"

vm_destroy
rm -f "$WORK_IMG"
qemu-img create -q -f qcow2 -b "$BASE_IMG" -F qcow2 "$WORK_IMG" 20G

# cloud-init NoCloud: an ISO labelled "cidata" when an ISO tool exists,
# otherwise the seed URL in the SMBIOS serial, served over HTTP. 10.0.2.2 is
# the host as seen from qemu's user-mode network.
SEED_ARGS=()
if command -v genisoimage >/dev/null; then
  genisoimage -quiet -output "$SEED_ISO" -volid cidata -joliet -rock "$SEED_DIR/user-data" "$SEED_DIR/meta-data"
  SEED_ARGS=(-cdrom "$SEED_ISO")
elif command -v xorrisofs >/dev/null; then
  xorrisofs -quiet -output "$SEED_ISO" -volid cidata -joliet -rock "$SEED_DIR/user-data" "$SEED_DIR/meta-data"
  SEED_ARGS=(-cdrom "$SEED_ISO")
else
  python3 -m http.server --bind 127.0.0.1 --directory "$SEED_DIR" "$HTTP_PORT" >"$CACHE_DIR/http.log" 2>&1 &
  echo $! >"$HTTP_PID_FILE"
  SEED_ARGS=(-smbios "type=1,serial=ds=nocloud;s=http://10.0.2.2:${HTTP_PORT}/")
fi

qemu-system-x86_64 \
  -machine q35,accel=kvm \
  -cpu host -m 4096 -smp 4 \
  -drive "file=$WORK_IMG,if=virtio" \
  "${SEED_ARGS[@]}" \
  -nic "user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:${SSH_PORT}-:22" \
  -display none \
  -serial "file:$CONSOLE_LOG" \
  -pidfile "$PID_FILE" \
  -daemonize

echo "waiting for ssh (console: $CONSOLE_LOG) ..."
for _ in $(seq 60); do
  if vm_ssh -o ConnectTimeout=3 true 2>/dev/null; then
    break
  fi
  sleep 5
done
vm_ssh true || {
  echo "VM never became reachable" >&2
  vm_destroy
  exit 1
}

# --- copy binary and scenario in ---
echo "copying hx-ready and scenario ..."
tar -C "$CACHE_DIR" -czf - hx-ready | vm_ssh 'mkdir -p ~/.local/bin && tar -xzf - -C ~/.local/bin'
tar -C "$REPO_DIR/test/vm" -czf - . | vm_ssh 'mkdir -p vm && tar -xzf - -C vm'

# --- run the scenario ---
FAILED=0
echo
echo "===== scenario"
vm_ssh 'bash vm/inside.sh' || FAILED=1

echo
if [ "${KEEP:-0}" = "1" ]; then
  echo "VM left running: 'just vm-ssh' to inspect, 'just vm-stop' to stop"
else
  vm_destroy
  echo "VM destroyed"
fi

if [ "$FAILED" -eq 0 ]; then
  echo "RESULT: PASS"
else
  echo "RESULT: FAIL"
  exit 1
fi
