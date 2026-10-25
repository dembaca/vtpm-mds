# Design

## Context

See proposal.md. Today the effective pin for a caller is its YAML inventory
`ek_sha256` (always empty on Proxmox), else `$STATE_DIRECTORY/ek-pins/<vmid>`,
created with `O_EXCL` on the first successful `enroll/finish`
(`internal/devid/pin.go`). Since `refuse-an-ek-pinned-to-another-vm` both legs
also scan that directory and the other VMs' inventory pins for the caller's
EK.

Facts this design rests on, from bgl-infra P1-a (2026-10-25, all three nodes):
`/etc/pve` is `755 root:www-data`, files pmxcfs creates are
`640 root:www-data` and cannot be `chmod`ed, `priv/` is `700`; a probe file in
`/etc/pve/vtpm-ek/` was readable by a `systemd-run` unit with the daemon's
`User=`, `SupplementaryGroups=www-data` and `ProtectSystem=strict`; the
existing local pins are 64 lowercase hex plus `\n`; all nodes were quorate.
`/etc/pve/.vmlist` is `440 root:www-data` on hogan (`ls -la`, 2026-10-25;
its content was not read).

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
  treated as absent, so the fallback (D-e) decides. The hook and this reader
  agreed on the format in bgl-infra `6q89czhg…`.
- **D-e. Fallback is the maintainer's "A".** No valid cluster pin and
  `require_cluster_pin: false` (default): today's behaviour, the local pin is
  enforced or created on first finish, plus
  `devid enroll: no cluster pin vm=<id>, using local pin` once per leg.
  `require_cluster_pin: true`: both legs answer `401`
  `no cluster pin for this VM` before any challenge, and finish creates no
  local pin. A cluster pin present means finish creates no local pin either:
  the cluster file is the record, and a second copy could only drift.
- **D-f. Quorum and absence.** pmxcfs stays readable without quorum, so the
  reader is unaffected; only the hook's write fails, and that VM then has no
  cluster pin (D-e decides). A missing `cluster_pin_dir` is an empty store
  (no cluster and plain-QEMU hosts); one that exists but cannot be listed, or
  a listed pin that cannot be read, answers `500` `pin store unavailable`, as
  the local store does (fail closed). pmxcfs not mounted at all is outside
  this: the unit is `PartOf=pve-cluster.service`.
- **D-g. Cross-VM scan, cluster-wide, without stale pins.** The scan of
  `refuse-an-ek-pinned-to-another-vm` adds every valid cluster pin. On a
  Proxmox host it also reads `/etc/pve/.vmlist` once per leg and ignores pins,
  cluster and local, whose VM ID is not in it: such a VM was destroyed, its
  vTPM is gone or restored elsewhere, and its pin must not refuse the VM
  restored from its backup (#89's "restore under a new ID"). If `.vmlist`
  cannot be read or parsed, nothing is ignored (refusing too much is the safe
  side) and one line is logged per leg. The YAML backend has no `.vmlist`;
  its scan is unchanged.
- **D-h. Migration of today's local pins is the backfill's.** The daemon
  cannot write the cluster directory (D-a), so it does not migrate. P1-w's
  one-off root run writes a cluster pin for each VM ID whose local pins agree
  on every node and that is in `.vmlist`, and reports disagreements. After it,
  the local pins stay as they are: shadowed for their own VM by the cluster
  pin (D-c), still counted by the scan.
- **D-i. Destroying a VM deletes nothing.** Proxmox has no destroy hook. A
  destroyed VM's pins stop refusing others through D-g; a VM that reuses the
  ID gets a fresh pin from the hook at its first vTPM initialisation (D-a).
  A reused ID whose vTPM was created without the hook (another node, hook not
  installed) is refused with `EK certificate does not match cluster pin`;
  the remedy is deleting that one file as root, documented in
  `vtpm-mds(8)`. An optional reaper for pins of VM IDs not in `.vmlist` is
  P1-w's, later.
- **D-j. Config.** Two keys under `mds`, decoded strictly like the rest:
  `cluster_pin_dir` (string, default `/etc/pve/vtpm-ek`, empty disables the
  cluster source, and then `require_cluster_pin: true` fails the load with an
  error naming both keys) and `require_cluster_pin` (bool, default `false`).

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
- **`.vmlist` format.** Expected to be pmxcfs's JSON with an `ids` object keyed
  by VM ID; **not yet seen here** (the agent user is not in `www-data`), so
  task 1.1 confirms it on a node before the parser is written. It is not a
  documented Proxmox interface. A format change makes D-g ignore nothing,
  which is the safe failure.
- Every leg lists two directories and reads `.vmlist`; measured in task 2.4.
