#!/usr/bin/env bash
# End-to-end smoke: from the nested guest, obtain an IMDSv2 token and instance-id.
set -euo pipefail

SSH_PORT="${MDS_LAB_SSH_PORT:-2222}"
SSH_OPTS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 -p "$SSH_PORT")
IMDS_URL="${MDS_LAB_IMDS_URL:-http://169.254.169.254}"
EXPECTED_ID="${MDS_LAB_EXPECTED_INSTANCE_ID:-i-100}"

echo "Waiting for guest SSH on port ${SSH_PORT}..."
for i in $(seq 1 90); do
  if ssh "${SSH_OPTS[@]}" ubuntu@127.0.0.1 'echo ok' >/dev/null 2>&1; then
    echo "SSH ready after ~${i}s"
    break
  fi
  if [[ "$i" -eq 90 ]]; then
    echo "ERROR: guest SSH did not become ready" >&2
    exit 1
  fi
  sleep 2
done

# Ensure IMDS NIC addressing inside guest (cloud-init may race)
ssh "${SSH_OPTS[@]}" ubuntu@127.0.0.1 "sudo bash -lc '
  ip link set ens4 up || true
  ip addr replace 169.254.169.10/16 dev ens4 || true
  ping -c1 -W1 169.254.169.1 >/dev/null || true
'"

echo "Requesting IMDSv2 token from guest via ${IMDS_URL}..."
TOKEN="$(ssh "${SSH_OPTS[@]}" ubuntu@127.0.0.1 \
  "curl -fsS -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' ${IMDS_URL}/latest/api/token")"

if [[ -z "$TOKEN" ]]; then
  echo "ERROR: empty token" >&2
  exit 1
fi
echo "Token acquired (${#TOKEN} bytes)"

INSTANCE_ID="$(ssh "${SSH_OPTS[@]}" ubuntu@127.0.0.1 \
  "curl -fsS -H \"X-aws-ec2-metadata-token: ${TOKEN}\" ${IMDS_URL}/latest/meta-data/instance-id")"

echo "instance-id=${INSTANCE_ID}"
if [[ "$INSTANCE_ID" != "$EXPECTED_ID" ]]; then
  echo "ERROR: expected ${EXPECTED_ID}, got ${INSTANCE_ID}" >&2
  exit 1
fi

echo "E2E IMDS smoke test PASSED"
