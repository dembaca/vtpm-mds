# Proposal

## Why

The service decides who a caller is from the caller's MAC address: peer IP,
then `/proc/net/arp`, then the inventory record that lists that MAC ("Bind A
Caller To A VM Record By ARP Lookup"). Nothing on the IMDS bridge stops a guest
from sending frames, or ARP replies, with another guest's MAC. A guest that
does so is answered as that other guest: its instance id, its tags, and an
LDevID enrollment under its VM id once the EK check is passed. The MAC is the
identity, and today every guest on the bridge can claim any MAC.

On `hogan` on 2026-10-19 the maintainer decided (E1(a), path B) to close this
with an nftables `bridge` table that admits, on each guest's port, only the MAC
in that guest's configuration. It was written by hand for VM 399 and loaded at
15:55 (`/etc/nftables.d/vtpm-mds.nft`, table `bridge vtpm_mds_bind`). From
inside VM 399, `instance-id` answered `200` with its own MAC, timed out with a
foreign one (`02:00:00:00:03:99`), and answered `200` again after the MAC was
restored. The `ether saddr` rule counted the 2 ARP requests sent with the
foreign MAC; the ARP-sender rule stayed at `0` because the first rule already
caught every frame. That proved the mechanism for one hand-written guest.

A hand-written rule does not scale and does not follow the guests: a VM that
is created, migrated in, or given a new MAC is unprotected until someone edits
the file, and a rule for a VM that left still admits its MAC on a port another
VM may get. The rules have to come from the same source as the inventory and
change when it changes.

## What Changes

- A new, separate host component writes the `bridge vtpm_mds_bind` table from
  this node's guest configurations: for every NIC of a local guest that is
  attached to the IMDS bridge, frames entering from that NIC's tap port with a
  source MAC other than the configured one are dropped, and so are ARP packets
  whose sender MAC is another one.
- The table is rewritten when the guest configurations change (the same
  fingerprint and interval as `reload-when-guest-configs-change`), and
  re-applied when it has gone missing, e.g. after `systemctl restart nftables`
  flushed the ruleset.
- The metadata daemon itself is unchanged and gains no privilege: it keeps
  only `CAP_NET_BIND_SERVICE` and no netlink socket. Writing nftables needs
  `CAP_NET_ADMIN`, which the new component has and the network-facing daemon
  must not.
- The host package ships the component and its unit. The hand-written VM 399
  block in `/etc/nftables.d/vtpm-mds.nft` is removed once the generated table
  replaces it; that file is managed by an Ansible role outside this
  repository, so the removal is handed to the maintainer.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `vm-inventory`: ADDED "Admit Only The Configured MAC On Each Guest Port Of
  The IMDS Bridge" (Proxmox backend).
- `debian-packaging`: MODIFIED "Host Package Ships The Daemon Only" to include
  the binder binary and its unit.

## Impact

- New `cmd/vtpm-mds-bind/` (or a mode of an existing binary, see design D-b),
  new `internal/bridgebind/` (port list from `/etc/pve/local/qemu-server`,
  rendering of the nft text), tests.
- `internal/proxmox/`: a reader of `netN` lines that keeps the NIC index, the
  bridge and the MAC for every NIC model, not only `virtio` (D-e). The
  inventory parser itself is not changed.
- `debian/`: `vtpm-mds.install`, a new `vtpm-mds-bind.service`, `rules` for
  enabling it, man page.
- Depends on `reload-when-guest-configs-change` (#62) for
  `proxmox.SourceFingerprint`; code starts after #62 is merged.
- Operators: one more unit per node; `/etc/nftables.d/vtpm-mds.nft` loses its
  hand-written block (Ansible role, maintainer).
- `README.md`: the coordinator adds the unit to the install section.
