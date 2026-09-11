#!/usr/bin/env bash
# Cloud Agent install: refresh Go deps, cache guest image, build binary.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

chmod +x scripts/qemu-lab/*.sh

# Durable lab directories (snapshot-friendly)
sudo mkdir -p /var/lib/mds-lab/{images,vms,run,inventory} /etc/mds-lab /etc/vtpm-mds /var/lib/vtpm-mds
sudo chown -R "$(id -u):$(id -g)" /var/lib/mds-lab /etc/mds-lab /var/lib/vtpm-mds || true

# Ensure kvm device node is present/accessible for hosts where nested KVM works.
# Cloud Agent VMs currently hit a host kvm BUG on vcpu create; lab defaults to TCG.
if [[ -e /dev/kvm ]]; then
  if ! getent group kvm >/dev/null 2>&1; then
    sudo groupadd -f kvm
  fi
  sudo usermod -aG kvm "$(id -un)" 2>/dev/null || true
  sudo chown root:kvm /dev/kvm || true
  sudo chmod 666 /dev/kvm || true
fi

# System packages needed for the QEMU lab (idempotent)
if ! command -v qemu-system-x86_64 >/dev/null 2>&1 || ! command -v cloud-localds >/dev/null 2>&1; then
  sudo apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    qemu-system-x86 qemu-utils qemu-kvm ovmf \
    iproute2 bridge-utils iptables \
    cloud-image-utils genisoimage \
    curl jq cpu-checker
fi

go mod download
make build

./scripts/qemu-lab/download-image.sh
cp -f testdata/vm-inventory/lab.yaml /var/lib/mds-lab/inventory/lab.yaml
ln -sfn /var/lib/mds-lab/inventory/lab.yaml /etc/mds-lab/inventory.yaml

echo "vtpm-mds cloud install complete"
./bin/vtpm-mds -h 2>&1 | head -5 || true
qemu-system-x86_64 --version | head -1
ls -lh /var/lib/mds-lab/images/
