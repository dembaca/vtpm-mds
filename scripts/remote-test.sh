#!/bin/bash
# Run tests on remote Proxmox host
# Usage: ./scripts/remote-test.sh [proxmox-host]

set -e

PROXMOX_HOST="${1:-hogan}"
PROXMOX_PATH="/opt/prox-mds"

echo "🧪 Running tests on $PROXMOX_HOST..."

ssh "$PROXMOX_HOST" "cd $PROXMOX_PATH && make test"

echo "✅ Tests complete!"

