#!/usr/bin/env bash
# Download (and cache) an Ubuntu cloud image for the QEMU lab guest.
set -euo pipefail

IMAGE_DIR="${MDS_LAB_IMAGE_DIR:-/var/lib/mds-lab/images}"
IMAGE_NAME="${MDS_LAB_IMAGE_NAME:-ubuntu-24.04-server-cloudimg-amd64.img}"
IMAGE_URL="${MDS_LAB_IMAGE_URL:-https://cloud-images.ubuntu.com/releases/noble/release/ubuntu-24.04-server-cloudimg-amd64.img}"
IMAGE_PATH="${IMAGE_DIR}/${IMAGE_NAME}"

mkdir -p "$IMAGE_DIR"

if [[ -f "$IMAGE_PATH" ]]; then
  echo "Cloud image already present: $IMAGE_PATH"
  ls -lh "$IMAGE_PATH"
  exit 0
fi

echo "Downloading cloud image to $IMAGE_PATH"
tmp="${IMAGE_PATH}.partial"
curl -fL --retry 3 --retry-delay 2 -o "$tmp" "$IMAGE_URL"
mv "$tmp" "$IMAGE_PATH"
ls -lh "$IMAGE_PATH"
