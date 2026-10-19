# Design

## Context

See proposal.md — Why.

What the hand-written rule on `hogan` looks like (loaded 2026-10-19 15:55):

```
table bridge vtpm_mds_bind {
    chain from_guests {
        type filter hook prerouting priority -200; policy accept;
        iifname "tap399i1" ether saddr != bc:24:11:06:1d:b2 counter drop;
        iifname "tap399i1" ether type arp arp saddr ether != bc:24:11:06:1d:b2 counter drop;
    }
}
```

Facts from `hogan` that shape the design:

- Proxmox names the tap of `net<n>` of VM `<vmid>` `tap<vmid>i<n>`. A NIC
  with `firewall=1` is not a port of its bridge directly: the tap is a port of
  `fwbr<vmid>i<n>`, and `fwpr<vmid>p<n>` is the port on the real bridge. On
  `hogan`, `vmbr_imds` has `tap399i1` (no firewall) and `fwpr300p1` (VM 300,
  `firewall=1`) as ports.
- `netfilter`'s `bridge` family hooks every bridge, so a rule on
  `iifname "tap300i1"` should see VM 300's frames as they enter `fwbr300i1`,
  before they reach `vmbr_imds`. **Not yet verified**; task 2.3 checks it in a
  netns with the same two-bridge layout.
- `internal/proxmox` reads `net<n>` lines only for `virtio=`
  (`netPattern`), and keeps neither the index nor the bridge.
- The daemon runs as `vtpm-mds` with `CAP_NET_BIND_SERVICE` only and
  `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`
  (`harden-the-service-unit`, `run-the-daemon-as-its-own-user`).
- `/etc/nftables.d/vtpm-mds.nft` is written by an Ansible role outside this
  repository; `nftables.service` loads it after `flush ruleset`.

## Goals / Non-Goals

**Goals:**

- On every node, each local guest NIC on the IMDS bridge admits only its
  configured MAC, as frame source and as ARP sender, without an operator
  editing a file per guest.
- Follows creation, migration, removal and MAC changes within one interval.
- The network-facing daemon keeps its privileges as they are.

**Non-Goals:**

- NICs on other bridges. Guests there may legitimately use other MACs
  (VRRP, bonding, nested virtualisation); binding them is the Proxmox
  firewall's `macfilter`, which E1 path A rejected for now.
- LXC containers (`veth<ctid>i<n>`). The inventory does not read them, so
  they are never bound, but one on the IMDS bridge could still claim a VM's
  MAC. None exists on `hogan`; recorded as a known gap.
- IPv6 neighbour discovery: the service binds IPv4 callers only.
- Making the daemon depend on the table (fail closed), see Risks.
- The EK pin per VM (`pin-the-ek-per-vm-on-proxmox`, #65), which is the
  second half of E1.

## Decisions

- **D-a. A separate component, not the daemon.** Writing nftables needs
  `CAP_NET_ADMIN` and a netlink socket. Giving those to the process that
  parses guest HTTP would undo `harden-the-service-unit` for the one thing it
  protects. The binder listens on nothing and talks to no guest.
- **D-b. Its own binary, `vtpm-mds-bind`, in the host package.** A mode of
  `vtpm-mds` would share the binary but blur which unit holds which
  capability in `systemd-analyze security`; a separate `cmd/` keeps both
  units' surfaces readable. Alternative: `vtpm-mds -bind`.
- **D-c. Apply through `nft -f -`.** The binder renders the whole table as
  text, in the declare/delete/define form proven idempotent on `hogan`, and
  pipes it to `/usr/sbin/nft -f -`, so each apply is one atomic transaction.
  The last applied text is kept in `/run/vtpm-mds-bind/vtpm_mds_bind.nft`
  for inspection. Alternative: a netlink library (`google/nftables`); no new
  dependency is the reason against it.
- **D-d. Rule shape: two drops per bound NIC, by tap name.** Exactly the two
  rules above, generated for `tap<vmid>i<n>` for every local guest whose
  `net<n>` is on the IMDS bridge, at `prerouting` priority `-200` (`filter`).
  Keying on the tap name covers `firewall=1` NICs too, because the bridge hook
  sees the tap's frames on `fwbr`. Rules are sorted by VM id and NIC index,
  so an unchanged source renders byte-identical text.
- **D-e. All NIC models.** The port list parses `net<n>: <model>=<MAC>,…,
  bridge=<name>` for any model, not only `virtio`. A guest with an `e1000`
  NIC on the IMDS bridge is not in the inventory today, but its port must be
  bound all the same, or it could claim another VM's MAC. The inventory parser
  stays as it is; widening it is its own change.
- **D-f. The IMDS bridge name is an argument of the unit.** `ExecStart=…
  -bridge vmbr_imds`, overridable by a drop-in. Not a `config.yaml` key: the
  daemon decodes that file strictly and has no use for the name. Not derived
  from the interface holding `listen_addr`: that ties the binder to the
  daemon's config and to the address being up.
- **D-g. Same trigger as the inventory.** Every 5 s the binder computes
  `proxmox.SourceFingerprint` (#62); on a change it re-renders and applies.
  On every tick it also lists the table and compares it with the listing
  taken after its last apply; a missing or different table (a `systemctl
  restart nftables`, a hand edit, the old hand-written block) is re-applied
  and logged once. Nothing is logged while nothing changes.
- **D-h. Stop leaves the table; removal deletes it.** Stopping the unit keeps
  the last rules in force, which stay correct until a guest changes. `postrm
  remove` deletes `bridge vtpm_mds_bind`. The table name stays
  `vtpm_mds_bind`, so the generated table replaces the hand-written one in
  place.
- **D-i. Unit.** `User=vtpm-mds`, `SupplementaryGroups=www-data`,
  `AmbientCapabilities=CAP_NET_ADMIN` and the same bounding set,
  `RestrictAddressFamilies=AF_NETLINK AF_UNIX`, `IPAddressDeny=any`,
  `ProtectSystem=strict`, `RuntimeDirectory=vtpm-mds-bind`,
  `After=nftables.service pve-cluster.service`,
  `PartOf=pve-cluster.service`. Handled by the package exactly like
  `vtpm-mds.service`: enabled, not started on a first installation, restarted
  on upgrade only if it was running (`restart-the-service-on-upgrade`). The
  `debian-packaging` requirement that says so ("Installing The Host Package
  Leaves systemd In Step") is being archived by #66; its MODIFIED block is
  added to this record against the archived text once #66 is merged (task
  1.3), not copied from the change record, so the two edits cannot overwrite
  each other.

## Risks / Trade-offs

- **Fail open.** If the binder is stopped or failing, the last table stays
  (D-h); if it never ran, there is none, which is today's state. The daemon
  does not check for the table. Making it refuse to bind without one would
  couple a read-only, unprivileged daemon to netfilter state; left for the
  maintainer if wanted.
- **A window of one interval.** A VM migrated in, or a NIC hot-plugged, has
  no rule until the next tick (≤ 5 s). The inventory reloads on the same tick,
  so the guest is not bound to its own record before that either; a spoofed
  MAC in that window would still be answered. A new VM is covered before it
  starts, because its config exists before its tap.
- **Live migration: up to one interval fail open on the target.** During a
  live migration the guest's tap exists on the target, and the guest resumes
  there, before its config is moved into `nodes/<target>/qemu-server`. Until
  the move and the next tick, the target has no rule for that port (≤ 5 s
  after the move). On the source, the rule stays until its next tick, on a
  tap that no longer exists, which is harmless. VM 399 cannot migrate, so the
  window is not measured on `hogan` (task 4.6 names it); the lab cannot
  migrate either.
- **Hand-written block and binder side by side.** Until task 5.2 removes the
  VM 399 block, two writers own `bridge vtpm_mds_bind`, and each replaces the
  whole table, so the last one wins:
  - Binder last: the table holds every bound port on the node, including the
    VM 399 rules the block has. Nothing the block protected is lost.
  - `nftables.service` last (a `systemctl restart nftables`, a reboot before
    the binder starts): `flush ruleset` removes everything, then the file
    restores only the VM 399 pair. VM 399 stays protected; every other port
    is open until the binder's next tick restores the full table (D-g, one
    log line). If the binder is not running at all, they stay open, which is
    today's state for them.
  - The block names only `tap399i1`. VM IDs are unique in the cluster, so on
    another node it either matches nothing or matches VM 399 after a
    migration, with the same MAC. A node never keeps another node's rules:
    each binder reads only `/etc/pve/local`, and `flush ruleset` empties the
    node's own ruleset before the file is read.
  Harmless, but the role should drop the block (task 5.2).
- **ARP-sender rule unproven.** The test of 2026-10-19 never reached it
  (rule 1 catches frames with a foreign source first). It needs its own test
  with a correct frame MAC and a forged ARP sender (task 4.4).
- **Recovery after a guest restores its own MAC took ~7.5 s** in that test
  (neighbour state on host and guest). Only a guest that changed its own MAC
  sees it.

## Migration

The binder takes over the table name the hand-written rules use. After its
first apply on a node, the block in `/etc/nftables.d/vtpm-mds.nft` is
redundant; the maintainer removes it from the Ansible role (task 5.2). The
idempotency lines for `inet vtpm_mds` stay.

## Open Questions

- D-b (own binary or a mode) and D-f (unit argument or config key): the
  maintainer's call; the record proceeds as written unless overruled.
- Fail open or closed (Risks): proceeds as fail open.
