#!/bin/bash
# Sync code to Proxmox for testing
# Usage: ./scripts/sync-to-proxmox.sh [proxmox-host]

set -e

PROXMOX_HOST="${1:-hogan}"
PROXMOX_PATH="/opt/prox-mds"

echo "🔄 Syncing code to $PROXMOX_HOST:$PROXMOX_PATH..."

# Sync files (exclude git, bin, etc.)
rsync -avz \
  --exclude '.git' \
  --exclude 'bin' \
  --exclude '*.deb' \
  --exclude 'deb-packages' \
  --exclude '.devcontainer' \
  --exclude 'node_modules' \
  ./ "$PROXMOX_HOST:$PROXMOX_PATH/"

echo "✅ Sync complete!"
echo ""
echo "Building on remote host..."
ssh "$PROXMOX_HOST" "cd $PROXMOX_PATH && make build"

echo ""
echo "✅ Build complete!"
echo "Run with: ssh $PROXMOX_HOST 'cd $PROXMOX_PATH && sudo ./bin/prox-mds'"

