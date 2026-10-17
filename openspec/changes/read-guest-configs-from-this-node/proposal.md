# Proposal

## Why

**The Proxmox backend can bind a caller to the wrong VM.** `internal/proxmox`
parses guest configuration in a way that ignores two facts about how Proxmox
writes it, and the living `vm-inventory` spec records both as current
behaviour. Both were found on `hogan` on 2026-10-17, while exploring where a
public address could come from (`declare-per-vm-metadata`, #33).

- **It reads the first node alphabetically, not this node.** `ParseVMConfigs`
  scans `/etc/pve/nodes/` and takes the first directory with a `qemu-server`
  subdirectory. `hogan` is a three-node cluster (`hogan`, `holger`, `horst`),
  and `os.ReadDir` sorts by name, so it reads `hogan` only because that name
  sorts first. On `holger` or `horst` the same package would read `hogan`'s
  guests. Proxmox already says which node this is: `/etc/pve/local` is a
  symlink to `nodes/<this node>`, maintained by Proxmox itself.
- **It treats snapshot sections as the live configuration.** A Proxmox VM
  config holds the live configuration first, then one `[<name>]` section per
  snapshot and possibly `[PENDING]` and `[special:cloudinit]`. The parser has
  no notion of sections: every `netN:` line in every section adds a MAC, and a
  key repeated in a later section overwrites the live value — `name` included.
  A NIC removed from the VM but still in a snapshot therefore still binds a
  caller to that VM, and a snapshot's old name replaces the current one.

Identity is the one thing this service exists to get right, and both defects
attach a caller to a record by data that does not describe the running guest.

A third, smaller point lives in the same file. **A hand-written `public-ipv4:`
line is honoured.** Proxmox rejects unknown keys (`check_type` dies with
"unknown setting"), so no Proxmox tool ever writes one; a line added by hand
survives only until the next `qm set` or GUI edit rewrites the file. Today the
parser stores it, and `instance-metadata` serves it as the VM's public address.
The maintainer decided on 2026-10-17 not to write anything into a VM config
that Proxmox itself would not put there, and deferred where a public address
should come from to #33. Until that is settled, the Proxmox backend should not
offer a source that disappears on the next edit.

## What Changes

- The Proxmox backend reads **this node's** guest configuration through
  `/etc/pve/local/qemu-server`, and fails the load with an error when that
  path does not exist. It no longer scans `/etc/pve/nodes/`.
- Only the **live section** of a config file is read: parsing stops at the
  first line that opens a section (`[`…`]`). Snapshot, pending and cloud-init
  sections contribute neither keys nor MACs.
- The Proxmox backend **does not store a `public-ipv4` key**, so a Proxmox VM
  has no public address until #33 decides where one comes from.
- The configuration root becomes a parameter, so tests can point the parser at
  a directory tree instead of `/etc/pve`.

## Capabilities

### Modified Capabilities

- `vm-inventory`:
  - `Derive VM Records From Proxmox Guest Configuration` — node selection,
    sections and the `public-ipv4` key. Its three scenarios are kept; the
    third is reworded for `/etc/pve/local`. Four scenarios are added.
  - `Load VM Records From The YAML Inventory` — only the paragraph that
    explains why the Proxmox backend carries `public-ipv4` verbatim. All its
    scenarios are carried unchanged.

## Impact

- `internal/proxmox/vmconfig.go`, and a new `internal/proxmox/vmconfig_test.go`
  (the package has no tests today).
- `internal/server/server.go` calls `proxmox.ParseVMConfigs("")`; the call site
  changes only if the signature does (see `design.md`).
- Docs that say "first node" or describe the scan, if any: `ARCHITECTURE.md`
  and `debian/vtpm-mds.8` mention only `/etc/pve` and need no change unless the
  implementation shows otherwise.
- **Behaviour on `hogan` should not change**, because `hogan` sorts first and
  no config there has snapshots or a `public-ipv4:` line today. That is exactly
  why the verification needs constructed config trees, not just the live host.
- **Not in scope:** the rule that only `virtio=` NICs contribute a MAC. It is
  specified behaviour (`A non-virtio NIC yields no MAC`) and a separate
  question; it stays as it is here.
