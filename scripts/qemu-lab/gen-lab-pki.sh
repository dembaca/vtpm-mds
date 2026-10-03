#!/usr/bin/env bash
# Generate lab DevID CA + EK trust chain (from the swtpm local CA in use).
set -euo pipefail

PKI_DIR="${MDS_LAB_PKI_DIR:-/var/lib/mds-lab/pki}"
SWTPM_SETUP_CONF="${MDS_LAB_SWTPM_SETUP_CONF:-/etc/swtpm_setup.conf}"
DEFAULT_LOCALCA_DIR="/var/lib/swtpm-localca"
DEFAULT_ISSUERCERT="${DEFAULT_LOCALCA_DIR}/issuercert.pem"

# Print the value of "key = value" from a swtpm config file (comments and
# spacing tolerated; the last assignment wins). Prints nothing when absent.
conf_value() {
  [[ -r "$1" ]] || return 0
  awk -v k="$2" '
    /^[[:space:]]*#/ { next }
    { i = index($0, "="); if (!i) next
      key = substr($0, 1, i - 1); gsub(/[[:space:]]/, "", key)
      if (key != k) next
      v = substr($0, i + 1); sub(/[[:space:]]*#.*$/, "", v)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", v); val = v }
    END { print val }' "$1"
}

mkdir -p "$PKI_DIR" /etc/vtpm-mds

if [[ ! -f "$PKI_DIR/devid-ca.pem" ]]; then
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$PKI_DIR/devid-ca-key.pem" \
    -out "$PKI_DIR/devid-ca.pem" \
    -days 3650 \
    -subj "/CN=vtpm-mds Lab DevID CA/O=vtpm-mds"
fi

# swtpm_setup resolves the CA that signs guest EK certificates through
# create_certs_tool_config; that file, not a fixed path, says which CA issued
# the EK certificate the guest will present. Fall back to the swtpm-localca
# default when nothing is configured.
localca_conf="$(conf_value "$SWTPM_SETUP_CONF" create_certs_tool_config)"
issuercert=""
if [[ -n "$localca_conf" ]]; then
  issuercert="$(conf_value "$localca_conf" issuercert)"
fi
issuercert="${issuercert:-$DEFAULT_ISSUERCERT}"

if [[ ! -s "$issuercert" ]]; then
  {
    echo "ERROR: cannot build the EK trust chain: issuer certificate not found."
    echo "  looked for:   $issuercert"
    echo "  swtpm setup:  $SWTPM_SETUP_CONF ($([[ -r "$SWTPM_SETUP_CONF" ]] && echo read || echo 'not readable'))"
    echo "  CA config:    ${localca_conf:-<create_certs_tool_config not set; default $DEFAULT_ISSUERCERT assumed>}"
    echo "Create a guest vTPM first (setup-guest-tpm.sh), or point"
    echo "MDS_LAB_SWTPM_SETUP_CONF at the swtpm_setup.conf in use."
  } >&2
  exit 1
fi

# The default swtpm-localca layout keeps a root CA certificate next to the
# issuer; include it when it is there (a site CA often has no such file).
cp -f "$issuercert" "$PKI_DIR/ek-chain.pem"
rootca="$(dirname "$issuercert")/swtpm-localca-rootca-cert.pem"
if [[ -s "$rootca" ]]; then
  cat "$rootca" >> "$PKI_DIR/ek-chain.pem"
fi

sudo cp -f "$PKI_DIR/devid-ca.pem" "$PKI_DIR/devid-ca-key.pem" "$PKI_DIR/ek-chain.pem" /etc/vtpm-mds/
sudo chmod 644 /etc/vtpm-mds/devid-ca.pem /etc/vtpm-mds/ek-chain.pem
sudo chmod 600 /etc/vtpm-mds/devid-ca-key.pem
echo "Lab PKI ready under $PKI_DIR and /etc/vtpm-mds (EK issuer: $issuercert)"
