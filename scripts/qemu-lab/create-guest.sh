#!/usr/bin/env bash
# Create (or refresh) a QEMU guest disk + cloud-init seed for MDS lab testing.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
VM_DIR="${LAB_DIR}/vms/${VM_NAME}"
IMAGE_PATH="${MDS_LAB_IMAGE_PATH:-${LAB_DIR}/images/ubuntu-24.04-server-cloudimg-amd64.img}"
DISK_SIZE="${MDS_LAB_DISK_SIZE:-8G}"
IMDS_MAC="${MDS_LAB_IMDS_MAC:-52:54:00:a1:b2:c3}"
IMDS_GUEST_IP="${MDS_LAB_GUEST_IP:-169.254.169.10}"
IMDS_HOST_IP="${MDS_LAB_HOST_IP:-169.254.169.1}"
IMDS_PREFIX="${MDS_LAB_GUEST_PREFIX:-16}"
SSH_PUBKEY_PATH="${MDS_LAB_SSH_PUBKEY:-${HOME}/.ssh/id_ed25519.pub}"

if [[ ! -f "$IMAGE_PATH" ]]; then
  echo "Missing base image: $IMAGE_PATH (run download-image.sh first)" >&2
  exit 1
fi

mkdir -p "$VM_DIR" "${HOME}/.ssh"
if [[ ! -f "${HOME}/.ssh/id_ed25519" ]]; then
  ssh-keygen -t ed25519 -N "" -f "${HOME}/.ssh/id_ed25519" >/dev/null
fi
SSH_PUBKEY_PATH="${HOME}/.ssh/id_ed25519.pub"
SSH_PUBKEY="$(cat "$SSH_PUBKEY_PATH")"

if [[ ! -f "${VM_DIR}/disk.qcow2" ]]; then
  qemu-img create -f qcow2 -F qcow2 -b "$IMAGE_PATH" "${VM_DIR}/disk.qcow2" "$DISK_SIZE"
else
  echo "Reusing existing disk ${VM_DIR}/disk.qcow2"
fi

cat >"${VM_DIR}/user-data" <<EOF
#cloud-config
hostname: ${VM_NAME}
manage_etc_hosts: true
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo
    shell: /bin/bash
    ssh_authorized_keys:
      - ${SSH_PUBKEY}
ssh_pwauth: false
package_update: false
runcmd:
  - [ bash, -lc, "ip link set ens4 up || true" ]
  - [ bash, -lc, "ip addr replace ${IMDS_GUEST_IP}/${IMDS_PREFIX} dev ens4 || true" ]
  - [ bash, -lc, "ip route replace ${IMDS_HOST_IP} dev ens4 || true" ]
EOF

cat >"${VM_DIR}/meta-data" <<EOF
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
EOF

cat >"${VM_DIR}/network-config" <<EOF
version: 2
ethernets:
  ens3:
    dhcp4: true
  ens4:
    dhcp4: false
    addresses:
      - ${IMDS_GUEST_IP}/${IMDS_PREFIX}
    routes:
      - to: ${IMDS_HOST_IP}/32
        via: ${IMDS_HOST_IP}
        on-link: true
      - to: 169.254.169.254/32
        via: ${IMDS_HOST_IP}
        on-link: true
EOF

cloud-localds -N "${VM_DIR}/network-config" "${VM_DIR}/seed.iso" "${VM_DIR}/user-data" "${VM_DIR}/meta-data"

cat >"${VM_DIR}/guest.env" <<EOF
VM_NAME=${VM_NAME}
IMDS_MAC=${IMDS_MAC}
IMDS_GUEST_IP=${IMDS_GUEST_IP}
DISK=${VM_DIR}/disk.qcow2
SEED=${VM_DIR}/seed.iso
EOF

echo "Guest prepared in ${VM_DIR}"
echo "  disk: ${VM_DIR}/disk.qcow2"
echo "  seed: ${VM_DIR}/seed.iso"
echo "  imds mac: ${IMDS_MAC}"
echo "  imds ip:  ${IMDS_GUEST_IP}"
