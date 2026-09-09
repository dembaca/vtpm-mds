#!/usr/bin/env bash
# Fast IMDS e2e without relying on guest SSH.
# Creates a netns that looks like the QEMU guest (same MAC/IP on br-imds).
set -euo pipefail

NS="${MDS_LAB_NETNS:-mds-guest100}"
MAC="${MDS_LAB_IMDS_MAC:-52:54:00:a1:b2:c3}"
GUEST_IP="${MDS_LAB_GUEST_IP:-169.254.169.10}"
IMDS_URL="${MDS_LAB_IMDS_URL:-http://169.254.169.254}"
EXPECTED_ID="${MDS_LAB_EXPECTED_INSTANCE_ID:-i-100}"
BRIDGE="${MDS_LAB_BRIDGE:-br-imds}"

if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
  echo "Bridge $BRIDGE missing; run setup-host.sh first" >&2
  exit 1
fi

cleanup() {
  sudo ip netns del "$NS" 2>/dev/null || true
  sudo ip link del veth-mds0 2>/dev/null || true
}
trap cleanup EXIT

cleanup
sudo ip netns add "$NS"
sudo ip link add veth-mds0 type veth peer name veth-mds1
sudo ip link set veth-mds0 master "$BRIDGE"
sudo ip link set veth-mds0 up
sudo ip link set veth-mds1 netns "$NS"
sudo ip netns exec "$NS" ip link set lo up
sudo ip netns exec "$NS" ip link set veth-mds1 address "$MAC"
sudo ip netns exec "$NS" ip addr add "${GUEST_IP}/16" dev veth-mds1
sudo ip netns exec "$NS" ip link set veth-mds1 up
sudo ip netns exec "$NS" ping -c1 -W2 169.254.169.1 >/dev/null

TOKEN="$(sudo ip netns exec "$NS" curl -fsS -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' "${IMDS_URL}/latest/api/token")"
INSTANCE_ID="$(sudo ip netns exec "$NS" curl -fsS -H "X-aws-ec2-metadata-token: ${TOKEN}" "${IMDS_URL}/latest/meta-data/instance-id")"

echo "instance-id=${INSTANCE_ID}"
if [[ "$INSTANCE_ID" != "$EXPECTED_ID" ]]; then
  echo "ERROR: expected ${EXPECTED_ID}, got ${INSTANCE_ID}" >&2
  exit 1
fi

echo "E2E netns IMDS smoke test PASSED"
