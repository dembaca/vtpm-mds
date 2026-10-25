# Tasks

Written on 2026-10-25 against `main@e54344d` by `agent-vtpm-hogan-d3e6cbc82`,
from the coordinator's TASK `s5wg3hmo…` (dembaca's post `ygzf39ah…`) and the
maintainer's decision "A" in bgl-infra `6q89czhg…`. Files:
`internal/devid/auth.go`, `internal/devid/pin.go`,
`internal/config/config.go`, `internal/server/server.go`, their tests,
`debian/config.yaml`, `debian/vtpm-mds.8`, `debian/changelog` and this
record; the README pin row is the coordinator's. The writer, the backfill and
any reaper are bgl-infra P1-w.

**Proposal only: implementation waits for the maintainer's go on the design.**

## 1. Implement

- [ ] 1.1 Confirm on one node that a `systemd-run` unit with the daemon's
      `User=`, `SupplementaryGroups=www-data` and `ProtectSystem=strict`
      can `stat` `/etc/pve/nodes/<other node>/qemu-server/<vmid>.conf` and
      `.../lxc/<vmid>.conf` of another node (P1-w or cursor, root); record
      only the result, no VM names
- [ ] 1.2 Add `mds.cluster_pin_dir` (default `/etc/pve/vtpm-ek`) and
      `mds.require_cluster_pin` (default `false`) with the load errors for
      `require_cluster_pin: true` with an empty directory or with
      `inventory_path` set (D-j, D-b); set `ClusterPinDir` and
      `GuestConfigRoot` only in the Proxmox branch of `server.go`; verify with
      config and server tests: defaults when omitted, strict decode, both load
      errors, both fields empty for the YAML backend
- [ ] 1.3 Read the caller's cluster pin with the strict format (D-d) and put
      it between inventory and local pin (D-c); verify with unit tests on a
      constructed directory: valid pin enforced, mismatch body, upper case /
      second line / empty / non-numeric name are no pin with the
      `invalid cluster pin` line, cluster pin over a differing local pin with
      the `local pin differs` line and the local file unchanged
- [ ] 1.4 Fallback and switch (D-e): no local pin is created when a cluster
      pin exists; the `no cluster pin … using local pin` line once per leg
      only when `cluster_pin_dir` exists; the refusal placed after the EK
      chain and binding checks (D-b2);
      `require_cluster_pin` refuses both legs with `401` and creates nothing;
      verify with handler tests
- [ ] 1.5 Extend the cross-VM scan with the cluster directory and the
      guest-config filter (D-g), inside the existing `pinCreateMu` for finish;
      verify with unit tests on a constructed `nodes/` tree: clone refused by
      a cluster pin, destroyed VM's local and cluster pins ignored, an
      `lxc` config counts, unlistable `GuestConfigRoot` ignores nothing and
      logs, empty `GuestConfigRoot` (YAML) unchanged, the concurrent-finish
      test still passes with `-race`
- [ ] 1.6 Fail closed (D-f): unlistable cluster directory or unreadable pin
      file answers `500` `pin store unavailable`; missing directory is empty;
      verify with unit tests (mode `0300`, skipped as uid 0)
- [ ] 1.7 Document in `debian/vtpm-mds.8` (EK PINS: cluster pins, precedence,
      the two keys, the log lines, the reused-ID remedy, writing a pin by
      hand for a VM restored under a new ID, turning on
      `require_cluster_pin` only after a clean backfill) and in
      `debian/config.yaml` (both keys, commented); verify with `man -l` and
      the strict decode of `debian/config.yaml`
- [ ] 1.8 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`

## 2. Verify on hogan

- [ ] 2.1 Build with `make deb-local` from a tree rebased on `main`;
      `dpkg-deb -c`/`-I`, `lintian --fail-on error,warning`,
      `systemd-analyze verify`
- [ ] 2.2 Install on hogan (maintainer's go) with no `/etc/pve/vtpm-ek`: VM
      399 enrolls into a scratch directory as before, the journal shows
      `no cluster pin vm=399, using local pin`, SPIRE's material untouched
- [ ] 2.3 After P1-w's backfill has written `/etc/pve/vtpm-ek/399`: VM 399
      enrolls into a scratch directory, no warning line. A mismatching
      cluster pin is covered by the unit tests only; the live file is not
      altered for it
- [ ] 2.4 Time one leg with the cluster directory and guest configs as they are
      on hogan, from the access log
- [ ] 2.5 Read the same pin from holger's view (P1-w or cursor, read-only):
      the file is the same on every node

## Pre-review

- [x] Fable review of this proposal before the maintainer's go
      **Done, 2026-10-25, of `7fb8ee8`:** must-fix taken: D-g recorded as
      superseding the archived D-e, with the "destroyed VM's vTPM enrolls
      again" risk; the hand-written pin for a VM restored under a new ID with
      `require_cluster_pin`. Should-fix taken: warning only when the
      directory exists; seams `ClusterPinDir`/`GuestConfigRoot` set only for
      Proxmox, `require_cluster_pin` with YAML a load error; guest configs
      instead of the undocumented `.vmlist`; refusal placed after the EK
      checks; the unretried hook; the clone scenario's GIVEN; two pins with
      one EK in Risks. Nits taken. Not taken: P1-a's probe directory, which
      no longer exists on hogan. The coordinator's two points (#89 stays
      open for SPIRE, approval order) are in the proposal.
- [ ] Review of the implementation by a stronger model before the PR is
      called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change
