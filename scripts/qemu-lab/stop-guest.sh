#!/usr/bin/env bash
# Stop the MDS lab QEMU guest and clean up its tap device.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
RUN_DIR="${LAB_DIR}/run"
PIDFILE="${RUN_DIR}/${VM_NAME}.pid"
TAP_NAME="tap-${VM_NAME}"

if [[ -f "$PIDFILE" ]]; then
  pid="$(cat "$PIDFILE")"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" || true
    fi
  fi
  rm -f "$PIDFILE"
fi

if ip link show "$TAP_NAME" >/dev/null 2>&1; then
  sudo ip link set "$TAP_NAME" down || true
  sudo ip link delete "$TAP_NAME" || true
fi

echo "Guest ${VM_NAME} stopped"
