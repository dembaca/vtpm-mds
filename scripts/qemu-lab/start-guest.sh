#!/usr/bin/env bash
# Start the MDS lab QEMU guest (SSH on host port 2222, IMDS NIC on br-imds).
set -euo pipefail

LAB_DIR="${MDS_LAB_DIR:-/var/lib/mds-lab}"
VM_NAME="${MDS_LAB_VM_NAME:-guest100}"
VM_DIR="${LAB_DIR}/vms/${VM_NAME}"
RUN_DIR="${LAB_DIR}/run"
BRIDGE="${MDS_LAB_BRIDGE:-br-imds}"
SSH_PORT="${MDS_LAB_SSH_PORT:-2222}"
CPUS="${MDS_LAB_CPUS:-2}"
MEM="${MDS_LAB_MEM:-2048}"
IMDS_MAC="${MDS_LAB_IMDS_MAC:-52:54:00:a1:b2:c3}"

# shellcheck disable=SC1091
source "${VM_DIR}/guest.env"

mkdir -p "$RUN_DIR"
PIDFILE="${RUN_DIR}/${VM_NAME}.pid"
QMP="${RUN_DIR}/${VM_NAME}.qmp"
SERIAL="${RUN_DIR}/${VM_NAME}.serial.log"
TAP_NAME="tap-${VM_NAME}"

if [[ -f "$PIDFILE" ]] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "Guest ${VM_NAME} already running (pid $(cat "$PIDFILE"))"
  exit 0
fi

if ! ip link show "$BRIDGE" >/dev/null 2>&1; then
  echo "Bridge $BRIDGE missing; run setup-host.sh first" >&2
  exit 1
fi

# Create a persistent tap attached to the IMDS bridge
if ! ip link show "$TAP_NAME" >/dev/null 2>&1; then
  sudo ip tuntap add dev "$TAP_NAME" mode tap user "$USER"
  sudo ip link set "$TAP_NAME" master "$BRIDGE"
fi
sudo ip link set "$TAP_NAME" up

ACCEL_ARGS=()
# Nested KVM on Cloud Agent hosts currently triggers a host kvm BUG in vmx_vcpu_create.
# Prefer TCG unless MDS_LAB_ACCEL=kvm is explicitly requested and /dev/kvm is usable.
ACCEL_MODE="${MDS_LAB_ACCEL:-tcg}"
if [[ "$ACCEL_MODE" == "kvm" && -r /dev/kvm && -w /dev/kvm ]]; then
  ACCEL_ARGS=(-enable-kvm -cpu host)
  echo "Using KVM acceleration (MDS_LAB_ACCEL=kvm)"
else
  ACCEL_ARGS=(-accel tcg,thread=multi -cpu max)
  if [[ "$ACCEL_MODE" == "kvm" ]]; then
    echo "WARNING: KVM requested but /dev/kvm not usable; falling back to TCG" >&2
  else
    echo "Using TCG emulation (set MDS_LAB_ACCEL=kvm to try nested KVM)"
  fi
fi

OVMF_CODE="${OVMF_CODE:-/usr/share/OVMF/OVMF_CODE_4M.fd}"
OVMF_VARS_TEMPLATE="${OVMF_VARS_TEMPLATE:-/usr/share/OVMF/OVMF_VARS_4M.fd}"
OVMF_VARS="${VM_DIR}/OVMF_VARS.fd"
if [[ ! -f "$OVMF_CODE" ]]; then
  echo "ERROR: OVMF code missing at $OVMF_CODE (install ovmf)" >&2
  exit 1
fi
if [[ ! -f "$OVMF_VARS" ]]; then
  cp "$OVMF_VARS_TEMPLATE" "$OVMF_VARS"
fi

# Avoid qemu -daemonize: the parent can hang under this environment's process supervision.
qemu-system-x86_64 \
  "${ACCEL_ARGS[@]}" \
  -machine q35 \
  -smp "$CPUS" \
  -m "$MEM" \
  -drive "if=pflash,format=raw,readonly=on,file=${OVMF_CODE}" \
  -drive "if=pflash,format=raw,file=${OVMF_VARS}" \
  -drive "file=${DISK},if=virtio,format=qcow2,cache=writeback" \
  -drive "file=${SEED},if=virtio,format=raw,readonly=on" \
  -netdev "user,id=net0,hostfwd=tcp::${SSH_PORT}-:22" \
  -device "virtio-net-pci,netdev=net0,mac=52:54:00:11:22:33" \
  -netdev "tap,id=net1,ifname=${TAP_NAME},script=no,downscript=no" \
  -device "virtio-net-pci,netdev=net1,mac=${IMDS_MAC}" \
  -display none \
  -serial "file:${SERIAL}" \
  -qmp "unix:${QMP},server,nowait" \
  -pidfile "$PIDFILE" \
  >>"${RUN_DIR}/${VM_NAME}.qemu.log" 2>&1 &

# Wait briefly for pidfile
for _ in $(seq 1 50); do
  if [[ -f "$PIDFILE" ]] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

if [[ ! -f "$PIDFILE" ]] || ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "ERROR: guest failed to start; see ${RUN_DIR}/${VM_NAME}.qemu.log" >&2
  exit 1
fi

echo "Guest ${VM_NAME} started"
echo "  pid:    $(cat "$PIDFILE")"
echo "  ssh:    ssh -p ${SSH_PORT} ubuntu@127.0.0.1"
echo "  serial: ${SERIAL}"
echo "  tap:    ${TAP_NAME} -> ${BRIDGE} (MAC ${IMDS_MAC})"
