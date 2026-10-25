# Proposal

## Why

EK pins are per node today: `$STATE_DIRECTORY/ek-pins/<vmid>` on the node
where a VM first enrolled (`pin-the-ek-per-vm-on-proxmox`). Three gaps follow,
all on a three-node cluster:

- **Migration is trust on first use again.** A VM migrated to a node that
  never saw it enrolls there as a first enrollment; whoever presents its MAC
  first on that node pins its own EK.
- **The clone refusal is per node** (`refuse-an-ek-pinned-to-another-vm`,
  Impact): a clone that kept its source's vTPM state is refused only on a
  node where the source is pinned.
- **A reused VM ID** (Issue #89) inherits the old VM's pin on the nodes where
  it enrolled, and only there.

The maintainer decided on 2026-10-25 (bgl-infra thread `6q89czhg…`, ask
`449qb6u9…`, answer "A") that pins become cluster-wide through pmxcfs,
written by a root hook in bgl-infra (P1-w) and read by this service (P1-r),
with the local pin kept as a logged fallback and a switch
`require_cluster_pin` that turns the fallback off.

## What Changes

- The service reads cluster EK pins from `/etc/pve/vtpm-ek/<vmid>`
  (`mds.cluster_pin_dir`, that default; empty turns the source off). A pin
  file holds one line of 64 lowercase hex, the SHA-256 of the EK certificate
  DER, the value `ek=` in `devid issued` already logs. The service **never
  writes** there.
- The caller's own effective pin becomes: YAML inventory `ek_sha256`, else its
  cluster pin, else its local pin, else none. A cluster pin that does not
  match answers `401` `EK certificate does not match cluster pin`.
- With a cluster pin present, `enroll/finish` creates no local pin. Without
  one it keeps today's local first-use pin and logs one warning line per leg.
- `mds.require_cluster_pin` (default `false`): when `true`, a VM without a
  valid cluster pin is refused on both legs with `401`
  `no cluster pin for this VM`, and no local pin is created.
- The cross-VM refusal of `refuse-an-ek-pinned-to-another-vm` also scans
  every cluster pin, so a clone is refused on every node. Pins of a VM ID that
  is not in the cluster's VM list (`/etc/pve/.vmlist`) no longer refuse
  anyone, local or cluster, so a destroyed VM's stale pin cannot block a VM
  restored from its backup.

## Impact

- `internal/devid/auth.go`, `internal/devid/pin.go`,
  `internal/config/config.go`, `internal/server/server.go` and their tests;
  `debian/config.yaml` (the two keys, commented), `debian/vtpm-mds.8` (EK PINS
  section), `debian/changelog`. Host package only; no guest change.
- Spec: `devid-enrollment` modifies "Persist An EK Pin Per VM After First
  Successful Enrollment", "Enforce The Stored EK Pin When Inventory Has None"
  and "Refuse An EK Pinned To Another VM", and adds "Read Cluster EK Pins From
  pmxcfs".
- **Depends on bgl-infra P1-w** for any effect: the hook that writes
  `/etc/pve/vtpm-ek/<vmid>` on a vTPM's first initialisation, and the one-off
  backfill of today's local pins. Without them the directory is absent, which
  counts as an empty store, and behaviour is today's plus the warning line.
- `require_cluster_pin: true` refuses every VM that the hook has not pinned,
  including all VMs created before the hook and not backfilled. Turn it on per
  node only after the backfill report is clean.
- Reads only: `/etc/pve` is `root:www-data`, readable by the daemon's
  supplementary group `www-data` under `ProtectSystem=strict`
  (`run-the-daemon-as-its-own-user`; checked for a probe file in
  `/etc/pve/vtpm-ek/` on all three nodes by bgl-infra P1-a). No unit change.
