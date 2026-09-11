#!/usr/bin/env bash
# End-to-end: boot guest with vTPM, enroll DevID via IMDS, verify SPIRE material.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
SHARED="${LAB_DIR}/vms/${VM_NAME}/shared"
WAIT_SECS="${MDS_LAB_DEVID_WAIT:-600}"

chmod +x scripts/qemu-lab/*.sh
make build
go build -o bin/devid-enroll ./cmd/devid-enroll

./scripts/qemu-lab/gen-lab-pki.sh
./scripts/qemu-lab/setup-host.sh
./scripts/qemu-lab/download-image.sh

# Ensure MDS is running with DevID CA
if ! curl -fsS http://169.254.169.1/health >/dev/null 2>&1; then
  sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.lab.yaml -debug >/tmp/vtpm-mds.log 2>&1 &
  sleep 1
fi
curl -fsS http://169.254.169.1/health >/dev/null

# Stop prior guest before recreating TPM/seed
./scripts/qemu-lab/stop-guest.sh >/dev/null 2>&1 || true
./scripts/qemu-lab/setup-guest-tpm.sh
./scripts/qemu-lab/create-guest.sh
./scripts/qemu-lab/start-guest.sh

echo "Waiting up to ${WAIT_SECS}s for guest DevID enrollment (TCG boot is slow)..."
deadline=$((SECONDS + WAIT_SECS))
while (( SECONDS < deadline )); do
  if [[ -f "${SHARED}/DONE" ]]; then
    rc="$(cat "${SHARED}/DONE" | tr -d '[:space:]')"
    echo "Guest enroll finished with rc=${rc}"
    [[ -f "${SHARED}/devid-enroll.log" ]] && tail -40 "${SHARED}/devid-enroll.log" || true
    if [[ "$rc" != "0" ]]; then
      exit 1
    fi
    break
  fi
  sleep 5
done

if [[ ! -f "${SHARED}/DONE" ]]; then
  echo "ERROR: timed out waiting for guest DevID" >&2
  tail -c 4000 "${LAB_DIR}/run/${VM_NAME}.serial.log" | strings | tail -40 >&2 || true
  exit 1
fi

test -f "${SHARED}/devid-out/devid.crt.pem"
test -f "${SHARED}/devid-out/devid.priv.blob"
test -f "${SHARED}/devid-out/devid.pub.blob"

openssl x509 -in "${SHARED}/devid-out/devid.crt.pem" -noout -subject -issuer -dates || true
python3 - <<PY
from pathlib import Path
pem = Path("${SHARED}/devid-out/devid.crt.pem").read_text()
assert "BEGIN CERTIFICATE" in pem
priv = Path("${SHARED}/devid-out/devid.priv.blob").read_bytes()
pub = Path("${SHARED}/devid-out/devid.pub.blob").read_bytes()
assert len(priv) > 0 and len(pub) > 0
print("DevID PEM + TPM2B blobs OK (SPIRE tpm_devid materials)")
PY

# Write SPIRE agent snippet for the materials
cat >"${SHARED}/devid-out/spire-agent-tpm_devid.hcl" <<EOF
NodeAttestor "tpm_devid" {
    plugin_data {
        devid_cert_path = "/opt/spire/conf/agent/devid.crt.pem"
        devid_priv_path = "/opt/spire/conf/agent/devid.priv.blob"
        devid_pub_path  = "/opt/spire/conf/agent/devid.pub.blob"
    }
}
EOF

echo "GUEST DEVID E2E PASSED"
echo "Materials: ${SHARED}/devid-out/"
ls -la "${SHARED}/devid-out/"
