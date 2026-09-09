#!/usr/bin/env bash
# Stop the MDS lab QEMU guest and clean up its tap device.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
RUN_DIR="${LAB_DIR}/run"
PIDFILE="${RUN_DIR}/${VM_NAME}.pid"
TAP_NAME="tap-${VM_NAME}"

stop_pidfile() {
  local f="$1"
  [[ -f "$f" ]] || return 0
  local pid
  pid="$(cat "$f" 2>/dev/null || true)"
  if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" || true
    fi
  fi
  rm -f "$f"
}

stop_pidfile "$PIDFILE"
stop_pidfile "${RUN_DIR}/${VM_NAME}.tpm.pid"
rm -f "${RUN_DIR}/${VM_NAME}.tpm.sock" 2>/dev/null || true

if ip link show "$TAP_NAME" >/dev/null 2>&1; then
  sudo ip link set "$TAP_NAME" down || true
  sudo ip link delete "$TAP_NAME" || true
fi

echo "Guest ${VM_NAME} stopped"
