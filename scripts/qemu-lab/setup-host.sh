#!/usr/bin/env bash
# Prepare host networking for the MDS QEMU lab.
# Creates br-imds with 169.254.169.1 and DNATs classic IMDS address 169.254.169.254.
set -euo pipefail

BRIDGE="${MDS_LAB_BRIDGE:-br-imds}"
HOST_IP="${MDS_LAB_HOST_IP:-169.254.169.1}"
IMDS_IP="${MDS_LAB_IMDS_IP:-169.254.169.254}"
PREFIX="${MDS_LAB_PREFIX:-16}"

ensure_kvm_access() {
  if [[ ! -e /dev/kvm ]]; then
    echo "NOTE: /dev/kvm missing — lab will use TCG emulation" >&2
    return 0
  fi
  if ! getent group kvm >/dev/null 2>&1; then
    sudo groupadd -f kvm
  fi
  sudo usermod -aG kvm,rdma "$USER" 2>/dev/null || true
  sudo chown root:kvm /dev/kvm || true
  # Nested KVM is currently broken on Cloud Agent hosts; chmod keeps the node usable
  # for future kvm attempts without blocking the default TCG path.
  sudo chmod 666 /dev/kvm || true
}

ensure_bridge() {
  if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
    sudo ip link add name "$BRIDGE" type bridge
  fi
  sudo ip link set "$BRIDGE" up
  if ! ip -4 addr show dev "$BRIDGE" | grep -q " ${HOST_IP}/"; then
    sudo ip addr add "${HOST_IP}/${PREFIX}" dev "$BRIDGE" || true
  fi
  # Own the classic IMDS address so guests can ARP it; DNAT redirects to HOST_IP.
  if ! ip -4 addr show dev "$BRIDGE" | grep -q " ${IMDS_IP}/"; then
    sudo ip addr add "${IMDS_IP}/${PREFIX}" dev "$BRIDGE" || true
  fi
  sudo sysctl -w net.ipv4.conf.all.rp_filter=0 >/dev/null
  sudo sysctl -w "net.ipv4.conf.${BRIDGE}.rp_filter=0" >/dev/null || true
  sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null
}

ensure_dnat() {
  # DNAT guest traffic destined to classic IMDS IP onto the host listener
  if ! sudo iptables -t nat -C PREROUTING -d "$IMDS_IP" -p tcp --dport 80 -j DNAT --to-destination "${HOST_IP}:80" 2>/dev/null; then
    sudo iptables -t nat -A PREROUTING -d "$IMDS_IP" -p tcp --dport 80 -j DNAT --to-destination "${HOST_IP}:80"
  fi
  if ! sudo iptables -t nat -C OUTPUT -d "$IMDS_IP" -p tcp --dport 80 -j DNAT --to-destination "${HOST_IP}:80" 2>/dev/null; then
    sudo iptables -t nat -A OUTPUT -d "$IMDS_IP" -p tcp --dport 80 -j DNAT --to-destination "${HOST_IP}:80"
  fi
  if ! sudo iptables -C FORWARD -i "$BRIDGE" -o "$BRIDGE" -j ACCEPT 2>/dev/null; then
    sudo iptables -A FORWARD -i "$BRIDGE" -o "$BRIDGE" -j ACCEPT
  fi
}

ensure_dirs() {
  sudo mkdir -p /var/lib/mds-lab/{images,vms,run,inventory} /etc/mds-lab /etc/prox-mds
  sudo chown -R "$USER:$USER" /var/lib/mds-lab /etc/mds-lab
}

install_inventory() {
  local src="${MDS_LAB_INVENTORY_SRC:-}"
  if [[ -z "$src" ]]; then
    # Prefer repo checkout when available
    if [[ -f "${PWD}/testdata/vm-inventory/lab.yaml" ]]; then
      src="${PWD}/testdata/vm-inventory/lab.yaml"
    elif [[ -f /workspace/testdata/vm-inventory/lab.yaml ]]; then
      src=/workspace/testdata/vm-inventory/lab.yaml
    fi
  fi
  if [[ -n "$src" && -f "$src" ]]; then
    cp "$src" /var/lib/mds-lab/inventory/lab.yaml
    ln -sfn /var/lib/mds-lab/inventory/lab.yaml /etc/mds-lab/inventory.yaml
  fi
}

install_lab_config() {
  sudo mkdir -p /etc/prox-mds /var/lib/prox-mds
  sudo tee /etc/prox-mds/config.lab.yaml >/dev/null <<EOF
mds:
  listen_addr: "${HOST_IP}:80"
  jwks_path: "/var/lib/prox-mds/jwks.json"
  attestation_ca: "/etc/prox-mds/attestation-ca.pem"
  ek_ca_chain: "/etc/prox-mds/ek-chain.pem"
  devid_ca_cert: "/etc/prox-mds/devid-ca.pem"
  devid_ca_key: "/etc/prox-mds/devid-ca-key.pem"
  token_ttl: "60s"
  jwt_ttl: "5m"
  enable_ec2_compat: true
  enable_tpm_attestation: true
  inventory_path: "/var/lib/mds-lab/inventory/lab.yaml"
EOF
  sudo chown -R "$USER:$USER" /var/lib/prox-mds || true
}

main() {
  ensure_kvm_access
  ensure_dirs
  ensure_bridge
  ensure_dnat
  install_inventory
  install_lab_config
  echo "MDS lab host ready: bridge=${BRIDGE} host=${HOST_IP} imds=${IMDS_IP}"
  ip -br addr show "$BRIDGE" || true
}

main "$@"
