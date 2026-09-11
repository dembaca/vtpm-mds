#!/usr/bin/env bash
# Generate lab DevID CA + EK trust chain (from swtpm-localca).
set -euo pipefail

PKI_DIR="${MDS_LAB_PKI_DIR:-/var/lib/mds-lab/pki}"
mkdir -p "$PKI_DIR" /etc/vtpm-mds

if [[ ! -f "$PKI_DIR/devid-ca.pem" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$PKI_DIR/devid-ca-key.pem" \
    -out "$PKI_DIR/devid-ca.pem" \
    -days 3650 \
    -subj "/CN=vtpm-mds Lab DevID CA/O=vtpm-mds"
fi

# Prefer swtpm-localca issuer chain when present
if [[ -f /var/lib/swtpm-localca/issuercert.pem ]]; then
  cat /var/lib/swtpm-localca/issuercert.pem \
      /var/lib/swtpm-localca/swtpm-localca-rootca-cert.pem \
      > "$PKI_DIR/ek-chain.pem"
else
  echo "WARNING: swtpm-localca issuer missing; create a guest TPM first" >&2
fi

sudo cp -f "$PKI_DIR/devid-ca.pem" "$PKI_DIR/devid-ca-key.pem" /etc/vtpm-mds/
[[ -f "$PKI_DIR/ek-chain.pem" ]] && sudo cp -f "$PKI_DIR/ek-chain.pem" /etc/vtpm-mds/
sudo chmod 644 /etc/vtpm-mds/devid-ca.pem /etc/vtpm-mds/ek-chain.pem 2>/dev/null || true
sudo chmod 600 /etc/vtpm-mds/devid-ca-key.pem
echo "Lab PKI ready under $PKI_DIR and /etc/vtpm-mds"
