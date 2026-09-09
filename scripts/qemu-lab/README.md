# QEMU MDS Lab

Nested-guest harness for developing **qemu-mds** without Proxmox.

The Cloud Agent / lab host acts as the hypervisor: `qemu-mds` listens on
`169.254.169.1:80`, bridge `br-imds` also owns `169.254.169.254` (DNAT → `:80`),
and callers are identified by **ARP MAC → YAML inventory**.

## Layout

| Script | Purpose |
|--------|---------|
| `cloud-install.sh` / `cloud-start.sh` | Cloud Agent environment bootstrap |
| `setup-host.sh` | Bridge, DNAT, lab config + inventory |
| `gen-lab-pki.sh` | Lab DevID CA + EK trust material |
| `download-image.sh` | Ubuntu cloud image cache |
| `setup-guest-tpm.sh` | Manufacture + start guest swtpm (QEMU ctrl socket) |
| `create-guest.sh` | Disk, cloud-init seed, 9p share, DevID oneshot |
| `start-guest.sh` / `stop-guest.sh` | QEMU with IMDS NIC + vTPM + 9p |
| `e2e-netns.sh` | Fast IMDS smoke (no full guest boot) |
| `e2e-imds.sh` | IMDS via guest SSH (TCG may be flaky) |
| `e2e-devid-guest.sh` | Full path: boot guest → DevID → SPIRE blobs |

## Recommended smoke test (fast)

```bash
./scripts/qemu-lab/cloud-install.sh
./scripts/qemu-lab/cloud-start.sh
sudo ./bin/qemu-mds -config /etc/prox-mds/config.lab.yaml &
./scripts/qemu-lab/e2e-netns.sh
```

`e2e-netns.sh` attaches a netns with the lab guest MAC/IP to `br-imds` and checks
IMDSv2 token + `instance-id` (`i-100`).

## DevID guest e2e (SPIRE materials)

Boots a QEMU guest with vTPM, runs a **cloud-init oneshot** that:

1. Brings up the IMDS NIC **by MAC** (q35 names are `enp0s*`, not `ens4`)
2. Runs `devid-enroll` against `http://169.254.169.254`
3. Authenticates with **MAC inventory + `X-qemu-mds-ek-cert`**
4. Writes SPIRE `tpm_devid` materials onto the 9p share

```bash
make build
sudo ./bin/qemu-mds -config /etc/prox-mds/config.lab.yaml &
sudo ./scripts/qemu-lab/e2e-devid-guest.sh
# materials: /var/lib/mds-lab/vms/guest100/shared/devid-out/
#   devid.crt.pem  devid.priv.blob  devid.pub.blob
```

Notes:

- Nested KVM currently hits a host `kvm` BUG (`vmx_vcpu_create`); the lab defaults to **TCG**.
- Guest SSH via user-net hostfwd can be flaky under TCG; prefer `e2e-netns.sh` for IMDS-only checks and `e2e-devid-guest.sh` (9p DONE file) for DevID.
- Guest SSH (when working): `ssh -p 2222 ubuntu@127.0.0.1`
- Do **not** broad-`pkill` patterns containing `qemu`/`swtpm` on Cloud Agent hosts.

## Inventory (no `/etc/pve`)

```yaml
vms:
  - id: "100"
    name: mds-lab-guest
    macs:
      - "52:54:00:a1:b2:c3"
    # optional: pin TPM EK certificate (hex SHA-256 of DER)
    # ek_sha256: "..."
```

Set `mds.inventory_path` (lab config writes this automatically). When unset, the
service falls back to Proxmox qemu-server configs under `/etc/pve`.

## Lab config paths

| Path | Role |
|------|------|
| `/etc/prox-mds/config.lab.yaml` | MDS listen + inventory + DevID CA paths |
| `/etc/prox-mds/devid-ca.pem` / `devid-ca-key.pem` | Lab DevID issuing CA |
| `/etc/prox-mds/ek-chain.pem` | Trust store for guest EK certs (swtpm-localca) |
| `/var/lib/mds-lab/inventory/lab.yaml` | YAML VM inventory |
| `/var/lib/mds-lab/vms/guest100/` | Disk, seed, shared/, tpm/ |
