# QEMU MDS Lab

Nested-guest harness for developing the metadata service without Proxmox.

The Cloud Agent host acts as the hypervisor: `prox-mds` listens on `169.254.169.1:80`, bridge `br-imds` also owns `169.254.169.254` (DNAT → `:80`), and callers are identified by ARP MAC → YAML inventory.

## Recommended smoke test (fast)

```bash
./scripts/qemu-lab/cloud-install.sh
./scripts/qemu-lab/cloud-start.sh
sudo ./bin/prox-mds -config /etc/prox-mds/config.lab.yaml &
./scripts/qemu-lab/e2e-netns.sh
```

`e2e-netns.sh` attaches a netns with the lab guest MAC/IP to `br-imds` and checks IMDSv2 token + `instance-id` (`i-100`).

## Optional full QEMU guest

```bash
./scripts/qemu-lab/create-guest.sh
./scripts/qemu-lab/start-guest.sh   # defaults to TCG
./scripts/qemu-lab/e2e-imds.sh      # requires guest SSH
```

Notes:
- Nested KVM currently hits a host `kvm` BUG (`vmx_vcpu_create`); the lab defaults to **TCG**.
- Guest SSH via user-net hostfwd can be flaky under TCG; prefer `e2e-netns.sh` for service validation.
- Guest SSH (when working): `ssh -p 2222 ubuntu@127.0.0.1`

## Inventory (no `/etc/pve`)

```yaml
vms:
  - id: "100"
    name: mds-lab-guest
    macs:
      - "52:54:00:a1:b2:c3"
```

Set `mds.inventory_path`. When unset, the service falls back to Proxmox qemu-server configs.
