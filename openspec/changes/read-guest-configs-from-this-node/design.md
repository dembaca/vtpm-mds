# Design

## Context

Evidence gathered on `hogan` on 2026-10-17, from the running system and the
Proxmox sources, not from documentation:

- `/etc/pve/nodes/` holds `hogan`, `holger` and `horst`. `/etc/pve/local` is a
  symlink to `nodes/hogan` (`ls -la /etc/pve/local`).
- `PVE/QemuServer.pm`: `check_type` dies with "unknown setting" for a key not
  in the schema; on the non-strict path the key is warned about and **not
  stored**, and `write_vm_config` serialises only the parsed hash. A
  hand-written key is therefore dropped on the next write.
- **Section order, from the writer.** `write_vm_config` (`QemuServer.pm`
  around line 2135) emits the live configuration first, then `[PENDING]` if
  anything is pending, then each `[special:<name>]` (cloud-init state), then
  one `[<snapname>]` per snapshot. `parse_vm_config` (around line 1944) starts
  in the `main` section and switches on any `[…]` header line. The live
  configuration is therefore always the text before the first header.
- `internal/proxmox/vmconfig.go` is unchanged since it was written; the
  package has no tests.

## Goals

- A caller is bound only by MACs of the VM's live configuration on this node.
- A record's raw configuration and name come from the live configuration only.
- The Proxmox backend offers no `public-ipv4` until #33 settles a source.
- The parser can be tested against a constructed tree.

## Non-Goals

- **Non-virtio NIC models.** Only `virtio=` contributes a MAC today and that is
  specified; widening it is its own change.
- **Cluster-wide inventory.** The IMDS bridge `vmbr_imds` has
  `bridge-ports none`; a guest on another node cannot reach this service at
  all. Serving this node's guests is the topology, not a policy choice, and a
  cluster-wide map would only widen the set of MACs that can collide.
- **Validating other keys**, or keeping only keys from the Proxmox schema. Only
  `public-ipv4` gets special treatment, because it is the one key a reader in
  this project acts on that Proxmox can never have written.
- **`[PENDING]` semantics.** Pending changes are not applied until restart, so
  they do not describe the running guest; they are ignored like snapshots.

## Decisions

### D1 — How the node is found

**`/etc/pve/local`**, and nothing else. It is maintained by `pmxcfs` and
points at this node's directory, which is exactly the question. No scan and no
fallback: if `/etc/pve/local/qemu-server` does not exist, the load fails with
an error naming that path, and the cache is left unchanged as today. A fallback
to "first node" would reintroduce the defect on precisely the hosts where
`local` is missing for a reason worth seeing.

### D2 — Where the live section ends

At the **first line whose trimmed form starts with `[`**. Everything after it
is ignored, whatever the section is called. Proxmox writes the live
configuration first and every other block after it, so "stop at the first
section header" needs no list of section names and cannot be fooled by a new
section type.

### D3 — `public-ipv4` on Proxmox

**Not stored.** The key is dropped from the raw configuration map of every
Proxmox record, so `instance-metadata` neither lists nor serves a public
address for a Proxmox VM. The YAML backend is unchanged. When #33 decides a
Proxmox-native source, that change will define how it populates the key.

### D4 — Making the root testable

`ParseVMConfigs(nodeName string)` becomes **`ParseVMConfigs(pveRoot string)`**:
the directory that plays `/etc/pve`, with `""` meaning `/etc/pve`. The only
caller passes `""` today and keeps doing so, so `server.go` does not change.
The `nodeName` parameter is removed rather than kept: nothing passes it, and a
caller that names a node is the scan problem in another form.

## Risks

- **A host where `/etc/pve/local` is missing** stops loading Proxmox records
  where the scan used to find some node. On a Proxmox host that path exists
  whenever `pmxcfs` is running, so its absence means `/etc/pve` is not usable
  and reading some other node's directory would hide that. The error must name
  the path.
- **A config with text before the first section that is not the live config**
  does not exist in Proxmox's format; if it ever did, D2 would read it as live.
