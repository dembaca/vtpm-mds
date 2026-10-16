#!/usr/bin/env bash
# Read TPM2_PT_LOCKOUT_COUNTER from the guest vTPM state directory.
# Starts a temporary TCP swtpm over that state (startup-clear), prints
# lockout properties, then stops it.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
TPM_DIR="${1:-${LAB_DIR}/vms/${VM_NAME}/tpm}"
RUN_DIR="${LAB_DIR}/run"
PID="${RUN_DIR}/lockout-probe.pid"
PORT="${MDS_LAB_LOCKOUT_PROBE_PORT:-2321}"
CTRL_PORT=$((PORT + 1))

if [[ ! -f "${TPM_DIR}/tpm2-00.permall" ]]; then
  echo "No guest TPM state at ${TPM_DIR}/tpm2-00.permall" >&2
  exit 1
fi

if [[ -f "${RUN_DIR}/${VM_NAME}.tpm.pid" ]] && kill -0 "$(cat "${RUN_DIR}/${VM_NAME}.tpm.pid")" 2>/dev/null; then
  echo "Guest swtpm still running (pid $(cat "${RUN_DIR}/${VM_NAME}.tpm.pid")); stop it first" >&2
  exit 1
fi

mkdir -p "$RUN_DIR"
if [[ -f "$PID" ]]; then
  kill "$(cat "$PID")" 2>/dev/null || true
  rm -f "$PID"
fi

swtpm socket --tpm2 --tpmstate "dir=${TPM_DIR}" \
  --server "port=${PORT}" \
  --ctrl "type=tcp,port=${CTRL_PORT}" \
  --flags not-need-init,startup-clear \
  --pid "file=${PID}" \
  --daemon

cleanup() {
  if [[ -f "$PID" ]]; then
    kill "$(cat "$PID")" 2>/dev/null || true
    rm -f "$PID"
  fi
}
trap cleanup EXIT

export TPM2TOOLS_TCTI="swtpm:host=127.0.0.1,port=${PORT}"
tpm2_getcap properties-variable | tee /tmp/lockout-probe.cap
grep -E 'TPM2_PT_LOCKOUT_COUNTER|TPM2_PT_MAX_AUTH_FAIL|inLockout' /tmp/lockout-probe.cap
