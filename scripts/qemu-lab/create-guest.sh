#!/usr/bin/env bash
# Create (or refresh) a QEMU guest disk + cloud-init seed for MDS + DevID lab testing.
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
VM_DIR="${LAB_DIR}/vms/${VM_NAME}"
SHARED_DIR="${VM_DIR}/shared"
IMAGE_PATH="${MDS_LAB_IMAGE_PATH:-${LAB_DIR}/images/ubuntu-24.04-server-cloudimg-amd64.img}"
DISK_SIZE="${MDS_LAB_DISK_SIZE:-8G}"
USERNET_MAC="${MDS_LAB_USERNET_MAC:-52:54:00:11:22:33}"
IMDS_MAC="${MDS_LAB_IMDS_MAC:-52:54:00:a1:b2:c3}"
IMDS_GUEST_IP="${MDS_LAB_GUEST_IP:-169.254.169.10}"
IMDS_HOST_IP="${MDS_LAB_HOST_IP:-169.254.169.1}"
IMDS_PREFIX="${MDS_LAB_GUEST_PREFIX:-16}"
INSTANCE_ID="${MDS_LAB_INSTANCE_ID:-${VM_NAME}-$(date +%s)}"

if [[ ! -f "$IMAGE_PATH" ]]; then
  echo "Missing base image: $IMAGE_PATH (run download-image.sh first)" >&2
  exit 1
fi

mkdir -p "$VM_DIR" "$SHARED_DIR" "${HOME}/.ssh"
if [[ ! -f "${HOME}/.ssh/id_ed25519" ]]; then
  ssh-keygen -t ed25519 -N "" -f "${HOME}/.ssh/id_ed25519" >/dev/null
fi
SSH_PUBKEY="$(cat "${HOME}/.ssh/id_ed25519.pub")"

# Stage enroll binary into the 9p share
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ ! -x "${ROOT}/bin/devid-enroll" ]]; then
  (cd "$ROOT" && go build -o bin/devid-enroll ./cmd/devid-enroll)
fi
cp -f "${ROOT}/bin/devid-enroll" "${SHARED_DIR}/devid-enroll"
chmod +x "${SHARED_DIR}/devid-enroll"
rm -rf "${SHARED_DIR}/devid-out" "${SHARED_DIR}/devid-enroll.log" "${SHARED_DIR}/DONE"

if [[ ! -f "${VM_DIR}/disk.qcow2" ]]; then
  qemu-img create -f qcow2 -F qcow2 -b "$IMAGE_PATH" "${VM_DIR}/disk.qcow2" "$DISK_SIZE"
else
  echo "Reusing existing disk ${VM_DIR}/disk.qcow2 (instance-id=${INSTANCE_ID} forces cloud-init)"
fi

# cloud-init runcmd runs under /bin/sh (dash). Ship a bash script via write_files.
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
bootcmd:
  - [ mkdir, -p, /mnt/shared ]
  - [ mount, -t, 9p, -o, trans=virtio,version=9p2000.L,shared, /mnt/shared ]
write_files:
  - path: /usr/local/sbin/devid-enroll-oneshot
    permissions: "0755"
    content: |
      #!/bin/bash
      set -euo pipefail
      mkdir -p /mnt/shared /mnt/shared/devid-out
      mountpoint -q /mnt/shared || mount -t 9p -o trans=virtio,version=9p2000.L shared /mnt/shared
      exec > >(tee -a /mnt/shared/devid-enroll.log) 2>&1
      echo "devid oneshot starting \$(date -Is)"

      iface_by_mac() {
        local mac="\$1" path iface
        mac="\$(echo "\$mac" | tr '[:upper:]' '[:lower:]')"
        for path in /sys/class/net/*/address; do
          if [ "\$(cat "\$path" | tr '[:upper:]' '[:lower:]')" = "\$mac" ]; then
            iface="\$(basename "\$(dirname "\$path")")"
            echo "\$iface"
            return 0
          fi
        done
        return 1
      }

      IMDS_IF=""
      for i in \$(seq 1 60); do
        if IMDS_IF=\$(iface_by_mac ${IMDS_MAC}); then
          break
        fi
        sleep 1
      done
      if [ -z "\${IMDS_IF}" ]; then
        echo "IMDS NIC ${IMDS_MAC} not found" >&2
        ip -br link || true
        echo 1 > /mnt/shared/DONE
        exit 1
      fi

      ip link set "\${IMDS_IF}" up
      ip addr replace ${IMDS_GUEST_IP}/${IMDS_PREFIX} dev "\${IMDS_IF}"
      ip route replace ${IMDS_HOST_IP}/32 dev "\${IMDS_IF}" || true
      ip route replace 169.254.169.254/32 dev "\${IMDS_IF}" || true
      echo "IMDS NIC \${IMDS_IF} configured as ${IMDS_GUEST_IP}/${IMDS_PREFIX}"
      ip -br addr show "\${IMDS_IF}" || true

      TPM_PATH=/dev/tpmrm0
      if [ ! -c "\$TPM_PATH" ]; then
        TPM_PATH=/dev/tpm0
      fi
      for i in \$(seq 1 60); do
        [ -c "\$TPM_PATH" ] && break
        sleep 1
      done
      if [ ! -c "\$TPM_PATH" ]; then
        echo "no TPM device" >&2
        ls -la /dev/tpm* || true
        echo 1 > /mnt/shared/DONE
        exit 1
      fi

      for i in \$(seq 1 60); do
        if curl -fsS -m 2 -o /dev/null -X PUT -H "X-aws-ec2-metadata-token-ttl-seconds: 60" http://169.254.169.254/latest/api/token; then
          echo "IMDS reachable after \${i} attempts"
          break
        fi
        sleep 2
      done

      chmod +x /mnt/shared/devid-enroll
      set +e
      /mnt/shared/devid-enroll \\
        -tpm "\${TPM_PATH}" \\
        -mds http://169.254.169.254 \\
        -out /mnt/shared/devid-out \\
        -cn ${VM_NAME}
      rc=\$?
      set -e
      echo "\$rc" > /mnt/shared/DONE
      chmod -R a+rX /mnt/shared/devid-out /mnt/shared/DONE /mnt/shared/devid-enroll.log 2>/dev/null || true
      echo "devid oneshot finished rc=\$rc \$(date -Is)"
      exit "\$rc"
runcmd:
  - [ /usr/local/sbin/devid-enroll-oneshot ]
EOF

cat >"${VM_DIR}/meta-data" <<EOF
instance-id: ${INSTANCE_ID}
local-hostname: ${VM_NAME}
EOF

# Match NICs by MAC — q35 virtio-net names are enp0s2/enp0s3, not ens3/ens4.
# /16 on the IMDS NIC makes 169.254.169.1 and .254 on-link.
cat >"${VM_DIR}/network-config" <<EOF
version: 2
ethernets:
  usernet:
    match:
      macaddress: "${USERNET_MAC}"
    dhcp4: true
    dhcp4-overrides:
      route-metric: 100
  imds:
    match:
      macaddress: "${IMDS_MAC}"
    dhcp4: false
    addresses:
      - ${IMDS_GUEST_IP}/${IMDS_PREFIX}
EOF

cloud-localds -N "${VM_DIR}/network-config" "${VM_DIR}/seed.iso" "${VM_DIR}/user-data" "${VM_DIR}/meta-data"

cat >"${VM_DIR}/guest.env" <<EOF
VM_NAME=${VM_NAME}
USERNET_MAC=${USERNET_MAC}
IMDS_MAC=${IMDS_MAC}
IMDS_GUEST_IP=${IMDS_GUEST_IP}
DISK=${VM_DIR}/disk.qcow2
SEED=${VM_DIR}/seed.iso
SHARED=${SHARED_DIR}
EOF

echo "Guest prepared in ${VM_DIR}"
echo "  disk:   ${VM_DIR}/disk.qcow2"
echo "  seed:   ${VM_DIR}/seed.iso"
echo "  shared: ${SHARED_DIR}"
echo "  imds:   ${IMDS_MAC} / ${IMDS_GUEST_IP}"
