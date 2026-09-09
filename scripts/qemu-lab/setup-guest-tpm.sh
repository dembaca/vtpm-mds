#!/usr/bin/env bash
# Manufacture guest TPM ctrl socket for QEMU (-tpmdev emulator uses --ctrl).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
TPM_DIR="${LAB_DIR}/vms/${VM_NAME}/tpm"
RUN_DIR="${LAB_DIR}/run"
# QEMU chardev connects to the ctrl socket
CTRL="${RUN_DIR}/${VM_NAME}.tpm.sock"

mkdir -p "$TPM_DIR" "$RUN_DIR" /var/lib/swtpm-localca
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

"${ROOT}/scripts/qemu-lab/gen-lab-pki.sh" >/dev/null

echo "swtpm ready for ${VM_NAME}"
echo "  ctrl (qemu): ${CTRL}"
echo "  state: ${TPM_DIR}"
