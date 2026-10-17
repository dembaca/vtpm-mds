# Proposal

## Why

**This is an exploration record, and in part a correction to merged code.** It
started as the question of where a Proxmox host may declare a VM's public
address. The maintainer widened it on 2026-10-17 into the question it really
is: **where does per-VM data come from that the service cannot observe and
Proxmox does not know, and where does the service derive what it can?** The
first three cases are `public-ipv4`, region and availability zone.

Four facts brought it here.

1. **The Proxmox half of `serve-a-public-address-or-omit-the-key` (#29, in
   `main`) has no stable carrier.** It reads `RawConfig["public-ipv4"]`, which
   `internal/proxmox` fills from a hand-written `public-ipv4:` line in
   `/etc/pve/qemu-server/<vmid>.conf`. Proxmox rejects that key as an unknown
   setting, warns on every parse, does not store it, and drops the line on its
   next write. The evidence is in `design.md`. The maintainer's constraint
   follows from it: nothing goes into a VM config that Proxmox would not put
   there itself.
2. **"The VM's IPv4 address" means three different things**, and only one of
   them is easy:
   - (a) The address the guest uses to reach this service. It is observable
     and already served as `local-ipv4`, but it is a link-local address on the
     metadata bridge.
   - (b) The guest's address on its real network. It is observable on the host
     side only where Proxmox SDN with IPAM and DHCP hands it out, which is not
     the case on `hogan` (`/etc/pve/sdn/` is empty). It has no key in the
     metadata tree today.
   - (c) A public address in front of NAT. It is not observable anywhere,
     because the NAT is outside Proxmox. AWS's `public-ipv4` means (c).
3. **The maintainer wants data-centre topology in the guest.** A guest should
   learn its region and availability zone so that it can be used for
   Kubernetes scheduling. Today the service serves constants:
   `placement/availability-zone` is `proxmox`, and the identity document says
   region `local`, both pinned by `instance-metadata`. `placement/region`
   does not exist.
4. **Topology is partly observable, unlike a public address.** Which Proxmox
   node hosts a VM, and which cluster that node belongs to, are host-side
   facts. One prerequisite is not met today: the service does not identify its
   own node. It reads the alphabetically first node directory of the cluster.
   That finding belongs to the coordinator's separate change.

## What Changes

**Not decided.** This record asks the maintainer to settle the category before
any key is implemented, so that the carrier is chosen once instead of once per
key. The questions are in `design.md`:

- **Labels vs. values.** Are short enumerable identifiers (zone, region,
  later rack or instance type) and structured values (addresses, IDs) one
  mechanism or two?
- **Carrier per category.** The candidates are Proxmox tags, Proxmox Notes, a
  file this service owns, and YAML inventory fields.
- **Derived defaults.** Should zone and region default to the hosting node and
  the cluster, with declarations as overrides?
- **Who may declare,** and what happens with a malformed or conflicting
  declaration.
- **The consumer.** How does a Kubernetes node actually obtain topology
  labels? This must be checked against current documentation before anything
  is designed for it.

Independent of all of that, one thing should happen soon: a hand-written
`public-ipv4:` line in a Proxmox guest config must stop being honoured, so the
fragile mechanism does not keep working until it silently stops.

## Capabilities

### Modified Capabilities

- `vm-inventory` — how each backend fills declared and derived per-VM data.
- `instance-metadata` — `placement/availability-zone`, a new
  `placement/region`, `public-ipv4`, and the `region` and `availabilityZone`
  fields of the identity document. Today the spec pins the constants
  `proxmox` and `local`; making them real changes specified behaviour.

The deltas are written once the maintainer has decided.

## Impact

Depends on the decision. Likely:
- `internal/proxmox/`, `internal/inventory/`, `imds/` and `internal/config/`;
- `README.md`, `debian/vtpm-mds.8` and `config.yaml`.

**Ordering:**
- After `serve-a-public-address-or-omit-the-key` is archived, because this
  modifies text its `vm-inventory` delta introduces.
- Any node-derived default also depends on the coordinator's
  `internal/proxmox` change (node selection and section handling).
