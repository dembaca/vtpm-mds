# Design

## Context

See proposal.md — Why.

`RefreshVMConfigCache` (`internal/server/server.go`) reloads either the YAML
inventory (`mds.inventory_path`) or the Proxmox backend
(`proxmox.ParseVMConfigs`, `proxmox.HostTopology`), swaps the cache under
`vmConfigCacheMu`, and logs `Cache refresh complete (<source>): N VMs, M MAC
addresses cached`. `main.go` calls it on `SIGHUP`; the unit's `ExecReload`
sends `SIGHUP`.

The Proxmox backend reads, from `/etc/pve`:

- `local/qemu-server/*.conf` (through the `local` symlink to this node),
- `local/config` (node description, topology lines),
- `corosync.conf` (`cluster_name`).

Measured on `hogan`, 2026-10-19 (see proposal): inotify on pmxcfs reports
writes made on this node only.

## Goals / Non-Goals

**Goals:**

- A guest created on, migrated to or recovered onto this node is bound
  within seconds, without an operator action, whichever node wrote its config.
- Changed MACs, tags and node topology take effect the same way.
- No added log volume while nothing changes.

**Non-Goals:**

- Re-binding connections that are already open; the existing requirement
  binds once per connection and stays.
- Re-reading `config.yaml`; `SIGHUP` does not, and neither does this.
- Reacting to ARP changes; binding still resolves ARP per connection.
- `/health` reporting the last reload; that is `report-readiness-honestly`.

## Decisions

- **D-a. Poll inside the daemon, not a `.path` unit.** A path unit is inotify
  and misses every remote write (measured). A `.timer` sending `SIGHUP` every
  few seconds would reload unconditionally and write the summary line each
  time, against `quiet-the-log-and-record-issuance`. Watching pmxcfs over its
  IPC socket would tie the daemon to an undocumented interface. Polling a
  fingerprint is backend-agnostic and works for the YAML file too.
- **D-b. Fingerprint the bytes, not mtimes.** The fingerprint is a SHA-256
  over the sorted file names and contents of exactly the files the loader
  reads. pmxcfs serves them from memory, the files are small, and content
  comparison does not depend on how pmxcfs reports mtimes for replicated
  writes. A reload happens only when the fingerprint differs from the one of
  the last successful load.
- **D-c. Interval 5 s, a package constant.** First boot of a new VM takes
  longer than 5 s to reach cloud-init; a guest that asks earlier retries.
  Constant, like the throttling limits (no config key in this change);
  alternative: `mds.inventory_poll_interval`.
- **D-d. One code path.** The loop calls `RefreshVMConfigCache`, so success,
  failure, logging and the swap behave exactly as for `SIGHUP`. A `SIGHUP`
  also records the new fingerprint, so the next tick does not reload again.
- **D-e. Errors once.** A failing fingerprint or load logs one line when the
  error first appears or its text changes, and nothing while it persists; the
  previous inventory stays in effect.
- **D-f. Lifecycle.** The loop starts after the startup load and stops when
  the server shuts down, so a clean stop still exits 0
  (`A Requested Shutdown Is Not A Failure`).

## Risks / Trade-offs

- A partially written config (a `qm set` mid-write) could be read. pmxcfs
  writes are atomic per file, and the next tick re-reads; at worst one tick
  serves the half-state that `SIGHUP` would also have served.
- During a pmxcfs restart the directory may vanish briefly: the check fails,
  the old inventory stays (D-e), and `PartOf=pve-cluster.service` restarts
  the daemon anyway.
- 5 s polling of three to a few hundred small files is negligible on a node;
  a node with thousands of guests would want a longer interval (D-c).

## Migration

None. `systemctl reload` keeps working and becomes optional.

## Open Questions

- D-c: 5 s as a constant, or a configuration key — the maintainer's call;
  the record proceeds with the constant unless overruled.
