# Proposal

## Why

**This is a correction to code that is already merged, not a planned
follow-up.** `serve-a-public-address-or-omit-the-key` (#29, merged as
`2dd0335`) serves `GET /latest/meta-data/public-ipv4` from
`RawConfig["public-ipv4"]` of the caller's bound VM record. With the YAML
inventory the operator declares it as `public_ipv4`. With the Proxmox backend
the record was expected to get it from a `public-ipv4: <address>` line in
`/etc/pve/qemu-server/<vmid>.conf`, because `internal/proxmox` stores every
`key: value` line of that file verbatim.

**Proxmox itself never writes such a line, and it drops one it finds.** The
maintainer asked on 2026-10-17 which Proxmox circumstance would put the field
there. The answer, read from the source rather than from memory by two agents
independently, is none:

- `PVE::QemuServer::parse_vm_config` matches the line with
  `^([a-z][a-z_\-]*\d*):\s*(.+?)\s*$` and passes it to `check_type`, which dies
  with `unknown setting 'public-ipv4'` because the key is in no schema. Outside
  strict mode `$handle_error` turns that into
  `warn "vm <vmid> - unable to parse value of 'public-ipv4' - …"`, and the key
  is **not stored** in the parsed configuration: the assignment
  `$conf->{$key} = $value` sits in the branch that runs only when `check_type`
  succeeded.
- `write_vm_config` serialises the parsed hash. A line that was never parsed is
  therefore **gone after the next write by Proxmox**: `qm set`, any edit in the
  web UI, a snapshot or a migration.

So the Proxmox half of #29 works only as long as nobody touches the VM through
Proxmox. Until then every parse of the file logs a warning. A declaration that
can disappear silently is the kind of loss this project has already paid for
once (`AGENTS.md`, "Never leave work uncommitted").

The maintainer has also stated a constraint: nothing should go into a VM
config that Proxmox would not put there itself. The YAML half of #29 is
unaffected and stays as it is.

## What Changes

**Not decided yet.** This record sets out four ways for an operator on a
Proxmox host to declare a public address, with their real costs, for the
maintainer to choose from. `design.md` has the detail and a recommendation. The
spec delta and the implementation tasks are written once the choice is made.

1. **Proxmox Notes**, the VM's `description`. This is native Proxmox data,
   kept across every write and editable in the web UI as "Notes".
2. **Proxmox tags**, `tags:`. Also native and kept, but the tag syntax does
   not hold a key and a value.
3. **A file this service owns**, keyed by VM ID, outside `/etc/pve`.
4. **No public address from the Proxmox backend.** Only the YAML inventory
   declares one.

Whichever is chosen, one thing changes regardless: a hand-written
`public-ipv4:` line in a Proxmox guest config must stop being honoured.
Otherwise the fragile mechanism keeps working until it silently does not.

## Capabilities

### Modified Capabilities

- `vm-inventory` — `Derive VM Records From Proxmox Guest Configuration`, and the
  clause in `Load VM Records From The YAML Inventory` that names the Proxmox
  backend as a source of `public-ipv4`. Option 3 may add a requirement for the
  new file. The exact shape depends on the option.
- `instance-metadata` — only if the chosen option changes what
  `GET /latest/meta-data/public-ipv4` reads. Options 1, 2 and 3 can keep the
  `RawConfig["public-ipv4"]` contract and change only how the Proxmox backend
  fills it.

## Impact

- `internal/proxmox/vmconfig.go` — options 1, 2 and 4.
- `internal/inventory/` and `internal/config/` — option 3.
- `openspec/changes/serve-a-public-address-or-omit-the-key/` — still open; its
  `vm-inventory` delta describes the Proxmox `public-ipv4:` line as a supported
  source. **Ordering:** this change applies after that one is archived, and
  modifies the text it leaves.
- Operators on Proxmox hosts: anyone who has already written a `public-ipv4:`
  line into a guest config — none is known; hogan's VM 399 deliberately has
  none — gets a migration note.
- `README.md`, `debian/vtpm-mds.8`: where the declaration is documented.
