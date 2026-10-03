# QEMU MDS Lab

Nested-guest harness for developing **vtpm-mds** without Proxmox.

The Cloud Agent / lab host acts as the hypervisor: `vtpm-mds` listens on
`169.254.169.1:80`, bridge `br-imds` also owns `169.254.169.254` (DNAT → `:80`),
and callers are identified by **ARP MAC → YAML inventory**.

## Where this lab belongs — and where it does not

These scripts are for an agent cloud host that has nothing else on it. They
claim `169.254.169.1` and `169.254.169.254` outright, create `br-imds`, and
DNAT the classic IMDS address.

**Do not run them on a Proxmox host.** A Proxmox host that serves vtpm-mds
already owns those addresses on `vmbr_imds`. Adding `br-imds` produces two
connected routes for `169.254.0.0/16`, and replies to a lab guest leave
through whichever interface the kernel picked — the guest then sees only
connection timeouts, minutes after the point where the mistake could have been
reported. `cloud-install.sh` would additionally install Debian's QEMU packages
over `pve-qemu-kvm`, taking every guest on the host with it.

`setup-host.sh` and `cloud-install.sh` now refuse in both cases and change
nothing. Do not work around them; there is a better test on that host.

**On a Proxmox host, test against a real guest instead.** That is the stronger
test anyway: real `/etc/pve` inventory, real ARP binding, a real vTPM, and the
daemon from the installed package rather than `make build`. Drive the guest
through the guest agent, for example:

```bash
sudo hogan-lab qm config 399 | grep -E '^(name|net[0-9]|tpmstate)'
sudo hogan-lab qm guest exec 399 -- /usr/bin/python3 -c '...'
```

The reason this warning exists is that the checkout lives on the Proxmox host,
so whoever works in the repository reaches for `scripts/qemu-lab/` without
leaving it.

## Supported cloud environments

The same scripts serve both agent clouds; there is no per-environment split.

| | Cursor cloud | Claude cloud |
|---|---|---|
| Base image | prepared, lab tools preinstalled | bare Ubuntu 24.04, root |
| `USER` | set | may be unset (scripts default it to `id -un`) |
| `/dev/kvm` | unusable, TCG | absent, TCG |
| Bootstrap | `cloud-install.sh` is a no-op for tools | `cloud-install.sh` installs what is missing |

`cloud-install.sh` installs only when a tool the lab calls is missing, so
running it on a prepared image changes nothing.

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
sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.lab.yaml &
./scripts/qemu-lab/e2e-netns.sh
```

`e2e-netns.sh` attaches a netns with the lab guest MAC/IP to `br-imds` and checks
IMDSv2 token + `instance-id` (`i-100`).

## DevID guest e2e (SPIRE materials)

Boots a QEMU guest with vTPM, runs a **cloud-init oneshot** that:

1. Brings up the IMDS NIC **by MAC** (q35 names are `enp0s*`, not `ens4`)
2. Runs `devid-enroll` against `http://169.254.169.254`
3. Authenticates with **MAC inventory + `X-vtpm-mds-ek-cert`**
4. Writes SPIRE `tpm_devid` materials onto the 9p share

```bash
make build
sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.lab.yaml &
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
| `/etc/vtpm-mds/config.lab.yaml` | MDS listen + inventory + DevID CA paths |
| `/etc/vtpm-mds/devid-ca.pem` / `devid-ca-key.pem` | Lab DevID issuing CA |
| `/etc/vtpm-mds/ek-chain.pem` | Trust store for guest EK certs (swtpm-localca) |
| `/var/lib/mds-lab/inventory/lab.yaml` | YAML VM inventory |
| `/var/lib/mds-lab/vms/guest100/` | Disk, seed, shared/, tpm/ |
