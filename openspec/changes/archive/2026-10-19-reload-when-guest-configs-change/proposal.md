# Proposal

## Why

The daemon loads its inventory once at startup and again only on `SIGHUP`
(`systemctl reload vtpm-mds`). Nothing sends that signal when a guest is
created, migrated in, has a NIC added or its MAC changed, or gains or loses a
`vtpm_mds.io.*` tag. Until someone reloads by hand:

- a **new or migrated-in VM** is an unbound caller: with
  `mds.require_vm_identity: true` it is refused metadata and cannot enroll a
  DevID — on first boot, which is exactly when cloud-init and `devid-enroll`
  need it;
- a **changed MAC** keeps binding to the old record, or to none;
- **tags and node topology** (`placement/…`) stay at the values of the last
  load.

It is on the production-readiness list of 2026-10-18 ("reload on new VMs").

The obvious fix — a systemd `.path` unit on `/etc/pve/local/qemu-server` that
triggers `systemctl reload` — does not work on a cluster, and this was
measured rather than assumed. On `hogan` on 2026-10-19 an inotify watch on
`/etc/pve/nodes/{hogan,holger,horst}` for 25 seconds saw 25 events for
`nodes/hogan` and **none** for `nodes/holger` or `nodes/horst`, although the
`lrm_status` files of both changed during the watch. pmxcfs is a FUSE file
system: the kernel reports writes made through this node's mount, not writes
that arrive from another node over corosync. A migration moves the guest's
config into `nodes/<target>/qemu-server/` from the **source** node, and HA
recovery from whichever node runs the CRM, so the cases that matter most are
the ones a path unit (inotify underneath) never sees.

## What Changes

- The daemon checks its inventory source periodically and reloads when the
  source has changed, through the same code path as `SIGHUP`.
- An unchanged source causes no reload and no log line; a change writes the
  existing `Cache refresh complete …` line once.
- A failed check keeps the previous inventory, as a failed `SIGHUP` reload
  does, and logs the error once until it changes or clears, not on every tick.
- `SIGHUP` stays, for an immediate reload.
- Both backends: the Proxmox backend (`/etc/pve/local/qemu-server/*.conf`, the
  node description, `corosync.conf`) and the YAML inventory file.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `vm-inventory`: gains "Reload The Cache When The Inventory Source Changes";
  "Load The Cache At Startup And Refresh It On SIGHUP" is unchanged.

## Impact

- `internal/server/server.go`: a background loop started with the server and
  stopped with it, calling `RefreshVMConfigCache` when the fingerprint of the
  source changes. This is product code, named here as the delta requires.
- `internal/proxmox/vmconfig.go` and `internal/inventory/`: a fingerprint
  function per backend over exactly the files each loader reads.
- Tests on constructed `/etc/pve` trees and YAML files.
- No change to `debian/`, the unit file, the configuration schema or any
  endpoint. `systemctl reload` keeps working.
- Load: reading a handful of small files from pmxcfs (served from memory)
  every few seconds.
