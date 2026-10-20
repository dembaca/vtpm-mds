#!/usr/bin/env bash
# Mint a per-cluster DevID intermediate from an offline root.
#
# Deployment (Q1): root stays offline on a CA host; Ansible (or an operator)
# runs this (or equivalent) once per cluster, distributes intermediate
# cert+key to that cluster's hypervisors, and the root *certificate* to SPIRE
# servers. This script never writes the root key into a hypervisor path.
#
# Intermediate profile (Q3): 1 year, pathLenConstraint=0, KeyUsage keyCertSign
# only (no crlSign, no EKU, no Name Constraints).
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: gen-devid-intermediate.sh --root-cert FILE --root-key FILE --out-dir DIR [--cn NAME] [--days N]

  --root-cert FILE   Offline DevID root certificate (PEM)
  --root-key FILE    Offline DevID root private key (PEM); keep offline
  --out-dir DIR      Writes devid-intermediate.pem and devid-intermediate-key.pem
  --cn NAME          Intermediate subject CN (default: DevID Intermediate)
  --days N           Validity in days (default: 365)

Does not touch SPIRE or hypervisor install paths. See design Q1/Q3 in
openspec/changes/issue-ldevids-from-an-intermediate-per-cluster/.
EOF
}

ROOT_CERT=""
ROOT_KEY=""
OUT_DIR=""
CN="DevID Intermediate"
DAYS=365

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --root-cert) ROOT_CERT="$2"; shift 2 ;;
    --root-key) ROOT_KEY="$2"; shift 2 ;;
    --out-dir) OUT_DIR="$2"; shift 2 ;;
    --cn) CN="$2"; shift 2 ;;
    --days) DAYS="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$ROOT_CERT" || -z "$ROOT_KEY" || -z "$OUT_DIR" ]]; then
  usage >&2
  exit 2
fi
for f in "$ROOT_CERT" "$ROOT_KEY"; do
  [[ -s "$f" ]] || { echo "missing or empty: $f" >&2; exit 1; }
done

mkdir -p "$OUT_DIR"
INTER_KEY="$OUT_DIR/devid-intermediate-key.pem"
INTER_CSR="$OUT_DIR/devid-intermediate.csr"
INTER_CERT="$OUT_DIR/devid-intermediate.pem"
EXT="$OUT_DIR/devid-intermediate.ext"

openssl genrsa -out "$INTER_KEY" 2048
chmod 600 "$INTER_KEY"
openssl req -new -key "$INTER_KEY" -out "$INTER_CSR" -subj "/CN=${CN}"

# pathLen 0, CA:TRUE, keyCertSign only
cat >"$EXT" <<'EOF'
basicConstraints=critical,CA:TRUE,pathlen:0
keyUsage=critical,keyCertSign
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
EOF

openssl x509 -req -in "$INTER_CSR" \
  -CA "$ROOT_CERT" -CAkey "$ROOT_KEY" -CAcreateserial \
  -out "$INTER_CERT" -days "$DAYS" -sha256 -extfile "$EXT"

# CSR/ext cleanup; -CAcreateserial may write <cert>.srl next to the CA cert.
rm -f "$INTER_CSR" "$EXT" \
  "${ROOT_CERT%.pem}.srl" \
  "$(dirname "$ROOT_CERT")/$(basename "${ROOT_CERT%.pem}").srl" 2>/dev/null || true

echo "Wrote $INTER_CERT and $INTER_KEY (${DAYS}d, pathLen=0, keyCertSign)"
openssl x509 -in "$INTER_CERT" -noout -subject -issuer -dates
openssl x509 -in "$INTER_CERT" -noout -text | awk '
  /Basic Constraints/,/X509v3 Key Usage/ { print }
  /X509v3 Key Usage/,/X509v3 Subject Key/ { print }
' | head -20
