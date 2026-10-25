# Design

## Context

See proposal.md. Today the effective pin for a caller is its YAML inventory
`ek_sha256` (always empty on Proxmox), else `$STATE_DIRECTORY/ek-pins/<vmid>`,
created by an exclusive create on the first successful `enroll/finish`
(`internal/devid/pin.go`). Since `refuse-an-ek-pinned-to-another-vm` both legs
also scan that directory and the other VMs' inventory pins for the caller's
EK.

**This change supersedes D-e of `refuse-an-ek-pinned-to-another-vm`** ("the
service never decides a pin is stale") for the cross-VM scan only: under D-g
a pin of a VM ID that no longer exists in the cluster stops refusing others.
It still never deletes or rewrites a pin, and a VM's own pin is enforced as
before.

Facts this design rests on, from bgl-infra P1-a (2026-10-25, all three nodes):
`/etc/pve` is `755 root:www-data`, files pmxcfs creates are
`640 root:www-data` and cannot be `chmod`ed, `priv/` is `700`; a probe file in
`/etc/pve/vtpm-ek/` was readable by a `systemd-run` unit with the daemon's
`User=`, `SupplementaryGroups=www-data` and `ProtectSystem=strict`; the
existing local pins are 64 lowercase hex plus `\n`; all nodes were quorate.
`/etc/pve/nodes/<node>/qemu-server/` is `755 root:www-data` on every node
(P1-a). `/etc/pve/vtpm-ek` does not exist on hogan (`ls`, 2026-10-25; P1-a's
probe directory was removed on all three nodes).

## Goals / Non-Goals

**Goals:** one EK pin per VM for the whole cluster, read by every node; a
migrated VM is checked against its pin on arrival; a clone is refused on
every node; the fallback to local first-use pins is visible and can be
switched off.

**Non-Goals:** writing to `/etc/pve` from this service; the hook, the backfill
and any reaper (bgl-infra P1-w); deleting pins; a `/health` field for cluster
pins; the YAML inventory's `ek_sha256` semantics; any guest change.

## Decisions

- **D-a. Who writes: only root, outside this service.** The daemon runs as
  `vtpm-mds` and pmxcfs lets only root write, so it cannot create a pin there,
  and giving it that right would let a compromised daemon pin any EK for any
  VM cluster-wide. The writer is the bgl-infra hook (P1-w): on a vTPM's first
  initialisation it hashes the EK certificate it wrote into NV `0x01c00002`
  and replaces `/etc/pve/vtpm-ek/<vmid>` atomically (temporary file in the
  same directory, then `rename`). The hook replaces on purpose, since a
  first initialisation under a known VM ID means a new vTPM (a reused VM ID,
  #89); so `O_EXCL` is the wrong primitive there, and the daemon needs none
  because it does not write. Whether pmxcfs `rename` within a directory is
  atomic is the hook's check (P1-w), not assumed here.
- **D-b. Read at enrollment time, not through the inventory.** The coordinator's
  first sketch filled the inventory's `ek_sha256` from the Proxmox backend.
  That backend reads only this node's guest configs (`read-guest-configs-from-this-node`),
  so the cross-VM scan would still miss VMs on other nodes, and the 5 s
  reload fingerprint would have to cover a directory that grows with the
  cluster. Instead the enroller reads `<cluster_pin_dir>/<vmid>` for the
  caller and lists the directory for the scan, on each leg, as it already
  does for the local store. pmxcfs serves reads from memory; enrollment is
  rare and limited to 6 per peer per 60 s. Task 2.4 measures a leg.
  The `Enroller` knows no backend, so `internal/server/server.go` sets its
  new fields `ClusterPinDir` and `GuestConfigRoot` (`/etc/pve/nodes`) only in
  the Proxmox branch; with the YAML inventory both stay empty, the cluster
  source and the D-g filter are off, and `require_cluster_pin: true` fails
  the load with an error naming it and `inventory_path`.
- **D-b2. Where in the leg.** The cluster pin is read in
  `AuthenticateEnrollCaller` where the own-pin check sits today: after the EK
  certificate chain is verified and bound to the endorsement key, before the
  cross-VM scan and the session and request checks. `no cluster pin for this
  VM` (D-e) is answered there too, so only a bound caller with a trusted EK
  learns it.
- **D-c. Precedence.** Own pin: YAML inventory `ek_sha256` (an explicit
  operator value), then the cluster pin, then the local pin. A cluster pin
  that disagrees with the local one wins and logs
  `devid enroll: local pin differs from cluster pin vm=<id>` once per leg,
  without fingerprints; the local file is left as it is (the service never
  rewrites pins).
- **D-d. Format, strictly.** A cluster pin is valid only when its name is a
  VM ID (digits only, `isVMIDName`) and its content is exactly 64 characters
  `[0-9a-f]` with an optional single trailing `\n`. Anything else, an empty
  file included, is not a pin: it is skipped by the scan and, for the
  caller's own VM, logged as `devid enroll: invalid cluster pin vm=<id>` and
  treated as absent, so the fallback (D-e) decides; that leg then logs both
  `invalid cluster pin` and `no cluster pin … using local pin`. The hook and
  this reader agreed on the format in bgl-infra `6q89czhg…`. Local pins stay
  as tolerant as today (`readStoredPin` trims and lowercases); the asymmetry
  is intended, since only cluster pins come from another program.
- **D-e. Fallback is the maintainer's "A".** No valid cluster pin and
  `require_cluster_pin: false` (default): today's behaviour, the local pin is
  enforced or created on first finish, plus
  `devid enroll: no cluster pin vm=<id>, using local pin` once per leg, but
  only when `cluster_pin_dir` exists, so a node without the hook rollout and
  a YAML host stay quiet.
  `require_cluster_pin: true`: both legs answer `401`
  `no cluster pin for this VM` before any challenge, and finish creates no
  local pin. A cluster pin present means finish creates no local pin either:
  the cluster file is the record, and a second copy could only drift.
- **D-f. Quorum and absence.** pmxcfs stays readable without quorum, so the
  reader is unaffected; only the hook's write fails, and that VM then has no
  cluster pin (D-e decides). The hook runs once per vTPM and is not retried,
  so that VM stays without a cluster pin until P1-w's backfill or a root
  writes it; the daemon's warning line is the only trace on this side. A missing `cluster_pin_dir` is an empty store
  (no cluster and plain-QEMU hosts); one that exists but cannot be listed, or
  a listed pin that cannot be read, answers `500` `pin store unavailable`, as
  the local store does (fail closed). pmxcfs not mounted at all is outside
  this: the unit is `PartOf=pve-cluster.service`.
- **D-g. Cross-VM scan, cluster-wide, without stale pins.** The scan of
  `refuse-an-ek-pinned-to-another-vm` adds every valid cluster pin. On a
  Proxmox host (`GuestConfigRoot` set, D-b) it also ignores pins, cluster and
  local, of a VM ID that has no guest config anywhere in the cluster, that is
  no `<GuestConfigRoot>/*/qemu-server/<vmid>.conf` and no
  `<GuestConfigRoot>/*/lxc/<vmid>.conf`: such a VM was destroyed, and its pin
  must not refuse the VM restored from its backup (#89's "restore under a
  new ID"). Containers count as well, because a VM ID is unique across both
  types; the check is on the ID, not on the type. Existence is looked up only
  for the VM IDs whose pin matches the caller's EK, so a leg costs at most a
  few `stat`s. If `GuestConfigRoot` cannot be listed, nothing is ignored
  (refusing too much is the safe side) and one line is logged per leg. The
  YAML backend leaves `GuestConfigRoot` empty; its scan is unchanged.
  The guest config files were chosen over `/etc/pve/.vmlist`, which holds
  the same list as JSON but is pmxcfs-internal and undocumented; the config
  paths are the interface this service already reads.
- **D-h. Migration of today's local pins is the backfill's.** The daemon
  cannot write the cluster directory (D-a), so it does not migrate. P1-w's
  one-off root run writes a cluster pin for each VM ID whose local pins agree
  on every node and that still has a guest config, and reports disagreements. After it,
  the local pins stay as they are: shadowed for their own VM by the cluster
  pin (D-c), still counted by the scan.
- **D-i. Destroying a VM deletes nothing.** Proxmox has no destroy hook. A
  destroyed VM's pins stop refusing others through D-g; a VM that reuses the
  ID gets a fresh pin from the hook at its first vTPM initialisation (D-a).
  A reused ID whose vTPM was created without the hook (another node, hook not
  installed) is refused with `EK certificate does not match cluster pin`;
  the remedy is deleting that one file as root, documented in
  `vtpm-mds(8)`. A VM restored under a new ID, or cloned with a fresh vTPM
  outside the hook, has no first initialisation the hook sees, so it never
  gets a cluster pin from it: with the default it falls back to a local pin,
  with `require_cluster_pin: true` it is refused with `no cluster pin for
  this VM` until root writes `/etc/pve/vtpm-ek/<newid>` by hand (one line of
  64 lowercase hex, the `ek=` value of that VM's EK; pmxcfs sets the owner),
  also documented in `vtpm-mds(8)`. An optional reaper for pins of VM IDs
  without a guest config is P1-w's, later. The SPIRE side of #89 (stale
  agents under `cn:<vmid>`, eight under `cn:399` on 2026-10-25) is not
  touched by this change; #89 stays open.
- **D-j. Config.** Two keys under `mds`, decoded strictly like the rest:
  `cluster_pin_dir` (string, default `/etc/pve/vtpm-ek`, empty disables the
  cluster source, and then `require_cluster_pin: true` fails the load with an
  error naming both keys, as it does with `inventory_path` set, D-b) and `require_cluster_pin` (bool, default `false`).

## Risks / Trade-offs

- **Trust moves to root on every node.** Anyone with root on any node can
  write a cluster pin for any VM. Root on a node already holds the DevID CA
  key (`run-the-daemon-as-its-own-user`), so this adds no new holder.
- **A wrong hook value locks a VM out cluster-wide** until the file is fixed;
  `EK certificate does not match cluster pin` in the guest and the host log
  says which file.
- **Fallback is silent to the guest.** With the default, a VM the hook missed
  still enrolls on first use, as today; the warning line is the only signal.
  `require_cluster_pin` exists for that.
- **A destroyed VM's vTPM becomes enrollable again** (D-g, superseding the
  old D-e). Once a VM is destroyed, any VM that holds a copy of its vTPM
  state (a backup, a leftover clone) enrolls under its own ID and, with the
  default, pins that EK locally. Guests cannot create guest configs, so a
  guest cannot cause this; it is a deliberate narrowing of the refusal, for
  the restore case.
- **Two cluster pins with one EK lock out both VMs** through the scan. That
  is a hook or backfill defect, visible as `pinned=` naming the other VM.
- Every leg lists two pin directories and `stat`s the guest configs of the
  matching VM IDs; measured in task 2.4.
