#!/usr/bin/env bash
# Generate lab DevID CA + EK trust chain (from the swtpm local CA in use).
#
# Single-tier (default): one self-signed devid-ca.pem used as both host signing
# CA and SPIRE devid_ca_path (legacy lab shape).
#
# Two-tier (MDS_LAB_DEVID_TIER=two): offline root + pathLen=0 intermediate.
#   devid-root.pem / devid-root-key.pem  → SPIRE server devid_ca_path (root only)
#   devid-ca.pem / devid-ca-key.pem      → host MDS signing material (intermediate)
# Root key stays under PKI_DIR; do not copy it next to the daemon's credential.
set -euo pipefail

PKI_DIR="${MDS_LAB_PKI_DIR:-/var/lib/mds-lab/pki}"
MDS_LAB_ETC_DIR="${MDS_LAB_ETC_DIR:-/etc/vtpm-mds}"  # pragma: allowlist secret
SWTPM_SETUP_CONF="${MDS_LAB_SWTPM_SETUP_CONF:-/etc/swtpm_setup.conf}"
DEFAULT_LOCALCA_DIR="/var/lib/swtpm-localca"
DEFAULT_ISSUERCERT="${DEFAULT_LOCALCA_DIR}/issuercert.pem"
TIER="${MDS_LAB_DEVID_TIER:-one}" # one | two
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

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

mkdir -p "$PKI_DIR" "$MDS_LAB_ETC_DIR"

case "$TIER" in
  one)
    if [[ ! -f "$PKI_DIR/devid-ca.pem" ]]; then
      openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout "$PKI_DIR/devid-ca-key.pem" \
        -out "$PKI_DIR/devid-ca.pem" \
        -days 3650 \
        -subj "/CN=Lab DevID CA/O=Lab"
    fi
    ;;
  two)
    # Fresh two-tier when root missing, or when forced.
    if [[ ! -f "$PKI_DIR/devid-root.pem" || "${MDS_LAB_PKI_FORCE:-}" == "1" ]]; then
      openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout "$PKI_DIR/devid-root-key.pem" \
        -out "$PKI_DIR/devid-root.pem" \
        -days 3650 \
        -subj "/CN=Lab DevID Root/O=Lab" \
        -addext "basicConstraints=critical,CA:TRUE,pathlen:1" \
        -addext "keyUsage=critical,keyCertSign"
      chmod 600 "$PKI_DIR/devid-root-key.pem"
      bash "$REPO_ROOT/scripts/gen-devid-intermediate.sh" \
        --root-cert "$PKI_DIR/devid-root.pem" \
        --root-key "$PKI_DIR/devid-root-key.pem" \
        --out-dir "$PKI_DIR" \
        --cn "Lab DevID Intermediate" \
        --days 365
      cp -f "$PKI_DIR/devid-intermediate.pem" "$PKI_DIR/devid-ca.pem"
      cp -f "$PKI_DIR/devid-intermediate-key.pem" "$PKI_DIR/devid-ca-key.pem"
      chmod 600 "$PKI_DIR/devid-ca-key.pem"
    elif [[ ! -f "$PKI_DIR/devid-ca.pem" ]]; then
      bash "$REPO_ROOT/scripts/gen-devid-intermediate.sh" \
        --root-cert "$PKI_DIR/devid-root.pem" \
        --root-key "$PKI_DIR/devid-root-key.pem" \
        --out-dir "$PKI_DIR" \
        --cn "Lab DevID Intermediate" \
        --days 365
      cp -f "$PKI_DIR/devid-intermediate.pem" "$PKI_DIR/devid-ca.pem"
      cp -f "$PKI_DIR/devid-intermediate-key.pem" "$PKI_DIR/devid-ca-key.pem"
      chmod 600 "$PKI_DIR/devid-ca-key.pem"
    fi
    ;;
  *)
    echo "MDS_LAB_DEVID_TIER must be 'one' or 'two' (got: $TIER)" >&2
    exit 2
    ;;
esac

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

# Host signing material: intermediate (or single-tier CA). Never copy root key.
sudo cp -f "$PKI_DIR/devid-ca.pem" "$PKI_DIR/devid-ca-key.pem" "$PKI_DIR/ek-chain.pem" "$MDS_LAB_ETC_DIR"/
sudo chmod 644 "$MDS_LAB_ETC_DIR"/devid-ca.pem "$MDS_LAB_ETC_DIR"/ek-chain.pem
sudo chmod 600 "$MDS_LAB_ETC_DIR"/devid-ca-key.pem

if [[ "$TIER" == "two" ]]; then
  # SPIRE trust anchor = offline root (certificate only).
  if [[ -d /var/lib/mds-lab/spire/conf/server ]]; then
    sudo cp -f "$PKI_DIR/devid-root.pem" /var/lib/mds-lab/spire/conf/server/devid-ca.pem
  fi
  echo "Lab PKI (two-tier) ready under $PKI_DIR and $MDS_LAB_ETC_DIR"
  echo "  host signing:  $MDS_LAB_ETC_DIR/devid-ca.pem (intermediate)"
  echo "  SPIRE root:    /var/lib/mds-lab/spire/conf/server/devid-ca.pem"
  echo "  root key:      $PKI_DIR/devid-root-key.pem (keep offline; not installed for MDS)"
else
  # Single-tier: SPIRE and host share the same CA cert — but do not clobber a
  # two-tier SPIRE root left by a previous MDS_LAB_DEVID_TIER=two run.
  if [[ -d /var/lib/mds-lab/spire/conf/server ]]; then
    if [[ -f "$PKI_DIR/devid-root.pem" ]]; then
      echo "NOTE: $PKI_DIR/devid-root.pem present; leaving SPIRE devid-ca.pem as root (use MDS_LAB_DEVID_TIER=two to refresh)." >&2
    else
      sudo cp -f "$PKI_DIR/devid-ca.pem" /var/lib/mds-lab/spire/conf/server/devid-ca.pem
    fi
  fi
  echo "Lab PKI ready under $PKI_DIR and $MDS_LAB_ETC_DIR (EK issuer: $issuercert)"
fi
