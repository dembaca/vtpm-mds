#!/usr/bin/env bash
# Host-side pieces for reconciler lab 2.2/4.1: two-tier PKI, SPIRE server,
# fake Proxmox API, reconciler config. Guest enrollment is separate
# (e2e-devid-guest.sh / agent in guest).
#
# On Cloud Agent hosts set MDS_LAB_ACCEL=tcg before start-guest.sh (nested
# KVM wedges; start-guest.sh documents this). SPIRE listens on
# 169.254.169.1:8081 so a guest on the IMDS bridge can attest.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
# shellcheck source=common.sh
source "${ROOT}/scripts/qemu-lab/common.sh"

require_tools python3 openssl sudo ip
refuse_non_lab_host || exit 1

SPIRE_ROOT="${MDS_LAB_SPIRE_DIR:-/var/lib/mds-lab/spire}"
FAKE_DIR="${MDS_LAB_FAKE_PVE_DIR:-/var/lib/mds-lab/fake-pve}"
PKI_DIR="${MDS_LAB_PKI_DIR:-/var/lib/mds-lab/pki}"
INV="${FAKE_DIR}/inventory.json"
TOKEN_FILE="${FAKE_DIR}/token"
CFG="${FAKE_DIR}/reconcile.yaml"
CLUSTER_NAME="${MDS_LAB_RECONCILE_CLUSTER:-lab1}"

mkdir -p "$SPIRE_ROOT"/{bin,conf/server,data/server,sockets,logs} "$FAKE_DIR"

# --- SPIRE binary ---
if [[ ! -x "$SPIRE_ROOT/bin/spire-server" ]]; then
  echo "ERROR: install SPIRE 1.14.x into $SPIRE_ROOT/bin first" >&2
  exit 1
fi

# --- Two-tier DevID PKI + copy root to SPIRE ---
export MDS_LAB_DEVID_TIER=two
./scripts/qemu-lab/gen-lab-pki.sh
# gen-lab-pki already sudo-mkdir's SPIRE conf when present; keep one sudo here.
sudo mkdir -p "$SPIRE_ROOT/conf/server"
sudo cp -f "$PKI_DIR/devid-root.pem" "$SPIRE_ROOT/conf/server/devid-ca.pem"
# EK trust for tpm_devid (lab PKI chain; same material MDS uses after gen-lab-pki)
sudo cp -f "$PKI_DIR/ek-chain.pem" "$SPIRE_ROOT/conf/server/ek-chain.pem"
sudo cp -f scripts/qemu-lab/reconcile-lab-spire-server.conf "$SPIRE_ROOT/conf/server/server.conf"

# --- Fake Proxmox inventory ---
# qemu 100: pool + two tags; qemu 101: no pool/tags (fields omitted);
# lxc and a template are listed so the reconciler's type/template filters show.
if [[ ! -f "$INV" ]]; then
  cat >"$INV" <<'EOF'
{"data":[
  {"vmid":100,"type":"qemu","pool":"web","tags":"vtpm_mds.io.role.frontend;vtpm_mds.io.env.lab","template":0},
  {"vmid":101,"type":"qemu","template":0},
  {"vmid":102,"type":"qemu","pool":"web","tags":"","template":1},
  {"vmid":200,"type":"lxc","pool":"web","tags":"vtpm_mds.io.role.frontend","template":0}
]}
EOF
fi
echo 'lab@pve!reconcile=not-checked' >"$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"

# --- TLS + fake API ---
if [[ ! -f "$FAKE_DIR/cert.pem" ]]; then
  # IP SAN required: reconciler verifies TLS against api_url's host (127.0.0.1).
  openssl req -x509 -newkey rsa:2048 -sha256 -days 7 -nodes \
    -subj '/CN=fake-proxmox-lab' \
    -addext 'subjectAltName=IP:127.0.0.1,DNS:localhost' \
    -keyout "$FAKE_DIR/key.pem" -out "$FAKE_DIR/cert.pem" 2>/dev/null
fi
if [[ -f "$FAKE_DIR/api.pid" ]] && kill -0 "$(cat "$FAKE_DIR/api.pid")" 2>/dev/null; then
  echo "fake-proxmox already running pid=$(cat "$FAKE_DIR/api.pid")"
else
  python3 scripts/qemu-lab/fake-proxmox-api.py \
    --listen 127.0.0.1:8443 \
    --inventory "$INV" \
    --tls-dir "$FAKE_DIR" \
    >"$FAKE_DIR/api.log" 2>&1 &
  echo $! >"$FAKE_DIR/api.pid"
  sleep 0.5
  if ! kill -0 "$(cat "$FAKE_DIR/api.pid")" 2>/dev/null; then
    echo "ERROR: fake-proxmox failed to start; see $FAKE_DIR/api.log" >&2
    tail -50 "$FAKE_DIR/api.log" >&2 || true
    exit 1
  fi
  if ! curl -sk --cacert "$FAKE_DIR/cert.pem" \
      "https://127.0.0.1:8443/api2/json/access/permissions" >/dev/null; then
    echo "ERROR: fake-proxmox not answering; see $FAKE_DIR/api.log" >&2
    tail -50 "$FAKE_DIR/api.log" >&2 || true
    exit 1
  fi
  echo "fake-proxmox pid=$(cat "$FAKE_DIR/api.pid")"
fi

# --- Reconciler config (intermediate = host signing CA, not root) ---
INTER_PEM="$PKI_DIR/devid-ca.pem"
cat >"$CFG" <<EOF
trust_domain: lab.local
spire_socket: ${SPIRE_ROOT}/sockets/server.sock
interval: 10s
clusters:
  - name: ${CLUSTER_NAME}
    api_url: https://127.0.0.1:8443
    ca_file: ${FAKE_DIR}/cert.pem
    token_file: ${TOKEN_FILE}
    intermediates:
      - ${INTER_PEM}
EOF

# --- SPIRE server ---
if [[ -f "$SPIRE_ROOT/server.pid" ]] && kill -0 "$(cat "$SPIRE_ROOT/server.pid")" 2>/dev/null; then
  echo "spire-server already running pid=$(cat "$SPIRE_ROOT/server.pid")"
else
  rm -f "$SPIRE_ROOT/sockets/server.sock"
  "$SPIRE_ROOT/bin/spire-server" run -config "$SPIRE_ROOT/conf/server/server.conf" \
    >"$SPIRE_ROOT/logs/server.log" 2>&1 &
  echo $! >"$SPIRE_ROOT/server.pid"
  for _ in $(seq 1 30); do
    [[ -S "$SPIRE_ROOT/sockets/server.sock" ]] && break
    sleep 0.5
  done
  if [[ ! -S "$SPIRE_ROOT/sockets/server.sock" ]]; then
    echo "ERROR: spire-server socket missing; see $SPIRE_ROOT/logs/server.log" >&2
    tail -50 "$SPIRE_ROOT/logs/server.log" >&2 || true
    exit 1
  fi
  echo "spire-server pid=$(cat "$SPIRE_ROOT/server.pid") sock=$SPIRE_ROOT/sockets/server.sock"
fi

echo "reconcile config: $CFG"
echo "inventory: $INV (edit between scenarios)"
echo "intermediate: $INTER_PEM"
openssl x509 -noout -fingerprint -sha1 -in "$INTER_PEM" \
  | sed 's/^.*=//' | tr -d ':' | tr 'A-F' 'a-f'
