#!/usr/bin/env bash
# Cloud Agent install: refresh Go deps, cache guest image, build binary.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# This is the cloud lab bootstrap. Its apt-get installs Debian's QEMU packages,
# which on a Proxmox host would displace pve-qemu-kvm and take every guest with
# it until someone reinstalls. Refuse there. The test on a Proxmox host is a
# real guest plus the installed package, not this lab — see README.md.
if command -v qemu-system-x86_64 >/dev/null 2>&1 \
   && dpkg -S "$(command -v qemu-system-x86_64)" 2>/dev/null | grep -q '^pve-qemu-kvm:'; then
  cat >&2 <<'EOF'
ERROR: qemu-system-x86_64 on this host comes from pve-qemu-kvm.

cloud-install.sh is the bootstrap for the cloud lab host; its apt-get would
install Debian's QEMU packages over the Proxmox ones. Nothing was changed.

On a Proxmox host, test vtpm-mds against a real guest and the installed
package instead — see scripts/qemu-lab/README.md.
EOF
  exit 1
fi

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

# System packages needed for the QEMU lab (idempotent). Install when ANY tool
# the lab scripts call is missing, so a host that already has QEMU but lacks
# e.g. swtpm_setup or ping is completed rather than skipped.
LAB_TOOLS=(
  qemu-system-x86_64 qemu-img cloud-localds genisoimage
  swtpm swtpm_setup swtpm_localca certtool
  ip iptables ping ssh-keygen
  curl jq openssl python3
)
missing=()
for tool in "${LAB_TOOLS[@]}"; do
  command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done
if ((${#missing[@]})); then
  echo "Installing lab packages (missing: ${missing[*]})"
  sudo apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    qemu-system-x86 qemu-utils qemu-kvm ovmf \
    iproute2 bridge-utils iptables iputils-ping \
    cloud-image-utils genisoimage \
    swtpm swtpm-tools gnutls-bin openssh-client \
    curl jq openssl python3 cpu-checker
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
