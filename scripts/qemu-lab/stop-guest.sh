#!/usr/bin/env bash
# Stop the MDS lab QEMU guest gracefully, then its guest vTPM, and clean up tap.
#
# Hard-killing QEMU while swtpm is still attached increments the TPM
# dictionary-attack counter (lockoutCounter). Prefer QMP system_powerdown so a
# reused guest vTPM does not burn auth failures on every e2e run.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
RUN_DIR="${LAB_DIR}/run"
PIDFILE="${RUN_DIR}/${VM_NAME}.pid"
QMP="${RUN_DIR}/${VM_NAME}.qmp"
TAP_NAME="tap-${VM_NAME}"
TPM_PIDFILE="${RUN_DIR}/${VM_NAME}.tpm.pid"
TPM_CTRL="${RUN_DIR}/${VM_NAME}.tpm.sock"
# Bound wait for ACPI powerdown under TCG; then SIGTERM/SIGKILL as fallback.
POWERDOWN_TIMEOUT_SEC="${MDS_LAB_POWERDOWN_TIMEOUT_SEC:-60}"

qmp_powerdown() {
  local sock="$1"
  [[ -S "$sock" ]] || return 1
  # QMP requires a capabilities negotiation before commands.
  if command -v socat >/dev/null 2>&1; then
    printf '%s\n' \
      '{"execute":"qmp_capabilities"}' \
      '{"execute":"system_powerdown"}' \
      | socat - "UNIX-CONNECT:${sock}" >/dev/null 2>&1 || return 1
    return 0
  fi
  # Fallback without socat: bash /dev/tcp does not speak unix sockets; use
  # python if present.
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$sock" <<'PY' >/dev/null 2>&1 || return 1
import json, socket, sys
sock_path = sys.argv[1]
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(5)
s.connect(sock_path)
buf = b""
while b"\n" not in buf:
    chunk = s.recv(4096)
    if not chunk:
        break
    buf += chunk
s.sendall(b'{"execute":"qmp_capabilities"}\n')
buf = b""
while b"\n" not in buf:
    chunk = s.recv(4096)
    if not chunk:
        break
    buf += chunk
s.sendall(b'{"execute":"system_powerdown"}\n')
buf = b""
while b"\n" not in buf:
    chunk = s.recv(4096)
    if not chunk:
        break
    buf += chunk
s.close()
PY
    return 0
  fi
  return 1
}

wait_pid_exit() {
  local pid="$1" timeout_sec="$2"
  local i
  for i in $(seq 1 "$timeout_sec"); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  return 1
}

if [[ -f "$PIDFILE" ]]; then
  pid="$(cat "$PIDFILE")"
  if kill -0 "$pid" 2>/dev/null; then
    if qmp_powerdown "$QMP"; then
      echo "Issued QMP system_powerdown; waiting up to ${POWERDOWN_TIMEOUT_SEC}s"
      if ! wait_pid_exit "$pid" "$POWERDOWN_TIMEOUT_SEC"; then
        echo "Guest did not exit after powerdown; sending SIGTERM" >&2
        kill "$pid" || true
        wait_pid_exit "$pid" 10 || true
      fi
    else
      echo "QMP powerdown unavailable; sending SIGTERM" >&2
      kill "$pid" || true
      wait_pid_exit "$pid" 10 || true
    fi
    if kill -0 "$pid" 2>/dev/null; then
      echo "Guest still running; sending SIGKILL (may bump TPM lockoutCounter)" >&2
      kill -9 "$pid" || true
    fi
  fi
  rm -f "$PIDFILE"
fi

# Stop guest swtpm only after QEMU has released the ctrl socket.
if [[ -f "$TPM_PIDFILE" ]]; then
  tpm_pid="$(cat "$TPM_PIDFILE")"
  if kill -0 "$tpm_pid" 2>/dev/null; then
    kill "$tpm_pid" || true
    wait_pid_exit "$tpm_pid" 10 || kill -9 "$tpm_pid" 2>/dev/null || true
  fi
  rm -f "$TPM_PIDFILE"
fi
rm -f "$TPM_CTRL" "${TPM_CTRL}.ctrl" "$QMP"

if ip link show "$TAP_NAME" >/dev/null 2>&1; then
  sudo ip link set "$TAP_NAME" down || true
  sudo ip link delete "$TAP_NAME" || true
fi

echo "Guest ${VM_NAME} stopped"
