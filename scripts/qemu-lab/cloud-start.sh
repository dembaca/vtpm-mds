#!/usr/bin/env bash
# Cloud Agent start: reconcile IMDS bridge + DNAT each boot (no long-running servers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

chmod +x scripts/qemu-lab/*.sh
./scripts/qemu-lab/setup-host.sh

# Do not auto-start the guest or prox-mds here: agents start them on demand.
# Verify bridge exists and exits successfully.
ip -br link show br-imds >/dev/null
ip -br addr show br-imds | grep -q 169.254.169.1
echo "mds-lab start reconciliation OK"
