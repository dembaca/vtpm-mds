#!/usr/bin/env bash
# Manufacture the guest vTPM and serve it on a control socket for QEMU
# (-tpmdev emulator uses --ctrl).
#
# Also serve a SEPARATE vTPM on a command socket for host-side tests
# (TestEnrollAgainstSwtpm opens it). It must be a second swtpm over its own
# state directory: adding --server to the guest's instance makes QEMU fail with
# "tpm-emulator: Failed to send CMD_SET_DATAFD", and a second instance over the
# guest's state directory would race on NVRAM. Both TPMs get their EK
# certificate from the same swtpm local CA, so one ek-chain.pem covers both.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
TPM_DIR="${LAB_DIR}/vms/${VM_NAME}/tpm"
RUN_DIR="${LAB_DIR}/run"
# QEMU chardev connects to the ctrl socket
CTRL="${RUN_DIR}/${VM_NAME}.tpm.sock"
# Host-test TPM: own state, command socket at the path the test opens.
HOST_TPM_DIR="${LAB_DIR}/vms/${VM_NAME}-hosttest/tpm"
HOST_SOCK="${RUN_DIR}/swtpm.sock"
HOST_PID="${RUN_DIR}/swtpm.pid"

mkdir -p "$TPM_DIR" "$HOST_TPM_DIR" "$RUN_DIR" /var/lib/swtpm-localca
sudo chown -R "$(id -u):$(id -g)" /var/lib/swtpm-localca 2>/dev/null || true

if [[ ! -f "${TPM_DIR}/tpm2-00.permall" ]]; then
  echo "Manufacturing vTPM state in ${TPM_DIR}"
  swtpm_setup --tpm2 --tpm-state "$TPM_DIR" \
    --create-ek-cert --create-platform-cert --lock-nvram --overwrite
fi

if [[ -f "${RUN_DIR}/${VM_NAME}.tpm.pid" ]]; then
  kill "$(cat "${RUN_DIR}/${VM_NAME}.tpm.pid")" 2>/dev/null || true
  rm -f "${RUN_DIR}/${VM_NAME}.tpm.pid"
fi
rm -f "$CTRL" "${CTRL}.ctrl"

swtpm socket --tpm2 --tpmstate "dir=${TPM_DIR}" \
  --ctrl "type=unixio,path=${CTRL},mode=0600" \
  --flags not-need-init \
  --pid "file=${RUN_DIR}/${VM_NAME}.tpm.pid" \
  --daemon

# Host-test TPM. startup-clear: nothing else sends TPM2_Startup on this socket,
# and an unstarted TPM answers TPM_RC_INITIALIZE to every command.
if [[ ! -f "${HOST_TPM_DIR}/tpm2-00.permall" ]]; then
  echo "Manufacturing host-test vTPM state in ${HOST_TPM_DIR}"
  swtpm_setup --tpm2 --tpm-state "$HOST_TPM_DIR" \
    --create-ek-cert --create-platform-cert --lock-nvram --overwrite
fi
if [[ -f "$HOST_PID" ]]; then
  kill "$(cat "$HOST_PID")" 2>/dev/null || true
  rm -f "$HOST_PID"
fi
rm -f "$HOST_SOCK"
swtpm socket --tpm2 --tpmstate "dir=${HOST_TPM_DIR}" \
  --server "type=unixio,path=${HOST_SOCK},mode=0600" \
  --flags not-need-init,startup-clear \
  --pid "file=${HOST_PID}" \
  --daemon

"${ROOT}/scripts/qemu-lab/gen-lab-pki.sh" >/dev/null

echo "swtpm ready for ${VM_NAME}"
echo "  ctrl (qemu): ${CTRL}"
echo "  cmd (tests): ${HOST_SOCK} (separate vTPM, state ${HOST_TPM_DIR})"
echo "  state: ${TPM_DIR}"
