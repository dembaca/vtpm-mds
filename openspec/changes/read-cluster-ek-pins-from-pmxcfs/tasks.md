# Tasks

Written on 2026-10-25 against `main@e54344d` by `agent-vtpm-hogan-d3e6cbc82`,
from the coordinator's TASK `s5wg3hmo…` (dembaca's post `ygzf39ah…`) and the
maintainer's decision "A" in bgl-infra `6q89czhg…`. Files:
`internal/devid/auth.go`, `internal/devid/pin.go`,
`internal/config/config.go`, `internal/server/server.go`, their tests,
`debian/config.yaml`, `debian/vtpm-mds.8`, `debian/changelog` and this
record; the README pin row is the coordinator's. The writer, the backfill and
any reaper are bgl-infra P1-w.

Design go: the maintainer's 👍 on ask `4ocgxj1z…` (2026-10-25 20:36),
including D-g and D-b.

## 1. Implement

- [x] 1.1 Confirm on one node that a `systemd-run` unit with the daemon's
      `User=`, `SupplementaryGroups=www-data` and `ProtectSystem=strict`
      can `stat` `/etc/pve/nodes/<other node>/qemu-server/<vmid>.conf` and
      `.../lxc/<vmid>.conf` of another node (P1-w or cursor, root); record
      only the result, no VM names
      **Done, 2026-10-25, by `agent-cursor-devid-6741` on hogan as root
      (post `dtpnzt8gwjrtmqit8689o6a9ma`):** as `vtpm-mds`, `stat` of a
      local and of another node's `qemu-server` config exits 0, and a
      nonexistent VM ID gives `ENOENT` locally, remotely and under `lxc`.
      There was no `EACCES`, even with `--clear-groups`, because `stat`
      needs only search permission on the directories. Reading a config's
      content still needs `www-data`.
- [x] 1.2 Add `mds.cluster_pin_dir` (default `/etc/pve/vtpm-ek`) and
      `mds.require_cluster_pin` (default `false`) with the load errors for
      `require_cluster_pin: true` with an empty directory or with
      `inventory_path` set (D-j, D-b); set `ClusterPinDir` and
      `GuestConfigRoot` only in the Proxmox branch of `server.go`; verify with
      config and server tests: defaults when omitted, strict decode, both load
      errors, both fields empty for the YAML backend
      **Done, 2026-10-25:** `TestLoadClusterPinDefaults`,
      `TestLoadRejectsRequireClusterPinWithoutSource` (both conflicts), the
      existing strict decode of `debian/config.yaml`, and
      `TestClusterPinSeamsOnlyForProxmox` in `internal/server` pass.
- [x] 1.3 Read the caller's cluster pin with the strict format (D-d) and put
      it between inventory and local pin (D-c); verify with unit tests on a
      constructed directory: valid pin enforced, mismatch body, upper case /
      second line / empty / non-numeric name are no pin with the
      `invalid cluster pin` line, cluster pin over a differing local pin with
      the `local pin differs` line and the local file unchanged
      **Done, 2026-10-25:** `internal/devid/clusterpin.go`
      (`parseClusterPin`, `readClusterPin`), `enforceOwnPin` in `auth.go`.
      `TestParseClusterPin` (9 inputs) and `TestAuthenticateClusterPin`
      (missing dir quiet, dir without pin warns, valid pin enforced and
      mismatch body, cluster over differing local pin with the log line, no
      fingerprint in the log and the local file unchanged, three malformed
      contents, required pin refuses, inventory wins) pass.
- [x] 1.4 Fallback and switch (D-e): no local pin is created when a cluster
      pin exists; the `no cluster pin … using local pin` line once per leg
      only when `cluster_pin_dir` exists; the refusal placed after the EK
      chain and binding checks (D-b2);
      **Done, 2026-10-25:** `Finish` reads the cluster pin before
      `ensurePinBeforeIssue`. `TestFinishWithClusterPin`: no local pin
      beside a cluster pin, `no cluster pin for this VM` and no local pin when
      one is required. The refusal sits in `enforceOwnPin`, after the chain
      and binding checks. Both legs go through `AuthenticateEnrollCaller`,
      so the auth-level tests cover start and finish;
      `require_cluster_pin` refuses both legs with `401` and creates nothing;
      verify with handler tests
- [x] 1.5 Extend the cross-VM scan with the cluster directory and the
      guest-config filter (D-g), inside the existing `pinCreateMu` for finish;
      verify with unit tests on a constructed `nodes/` tree: clone refused by
      a cluster pin, destroyed VM's local and cluster pins ignored, an
      `lxc` config counts, unlistable `GuestConfigRoot` ignores nothing and
      logs, empty `GuestConfigRoot` (YAML) unchanged, the concurrent-finish
      test still passes with `-race`
      **Done, 2026-10-25:** `clusterPinsMatching` and `guestExists` are used in
      `pinnedToAnotherVM`. `TestPinnedToAnotherVMClusterAndStale` (clone
      refused by another node's cluster pin, malformed cluster pin ignored,
      destroyed VM's local and cluster pins ignored, `lxc` counts, YAML
      unchanged) and `TestUnlistableGuestConfigRootIgnoresNothing` pass. The
      existing scan, race and concurrent-finish tests pass with `-race`
      (`internal/devid` 62.6 s).
- [x] 1.6 Fail closed (D-f): unlistable cluster directory or unreadable pin
      file answers `500` `pin store unavailable`; missing directory is empty;
      verify with unit tests (mode `0300`, skipped as uid 0)
      **Done, 2026-10-25:** `TestUnusableClusterPinDirFailsClosed` ran as
      `coding-agent`, not skipped. Both legs answer `500` `pin store
      unavailable` for an unlistable directory, the scan returns
      `ErrPinStore`, and an unreadable own pin (`0200`) is `ErrPinStore`.
- [x] 1.7 Document in `debian/vtpm-mds.8` (EK PINS: cluster pins, precedence,
      the two keys, the log lines, the reused-ID remedy, writing a pin by
      hand for a VM restored under a new ID, turning on
      `require_cluster_pin` only after a clean backfill) and in
      `debian/config.yaml` (both keys, commented); verify with `man -l` and
      the strict decode of `debian/config.yaml`
      **Done, 2026-10-25:** EK PINS has a cluster-pins paragraph, the
      stale-pin rule, the restore, reused-ID and replaced-vTPM cases, and a
      FILES entry. `man --warnings -l` prints nothing. `config.yaml` has
      both keys, commented, and still strict-decodes (config test).
      `debian/changelog` is left to the release, as for 0.3.2.
- [x] 1.8 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`
      **Done, 2026-10-25:** vet clean; every package `ok` (`internal/devid`
      103.3 s); validate `6 passed, 0 failed`.

## 2. Verify on hogan

- [x] 2.1 Build with `make deb-local` from a tree rebased on `main`;
      `dpkg-deb -c`/`-I`, `lintian --fail-on error,warning`,
      `systemd-analyze verify`
      **Done, 2026-10-25:** `vtpm-mds_0.3.2+git20261025.d6c40ce6070c_amd64.deb`
      from `d6c40ce` on `main@e54344d` (rebuilt after the D-g review; the
      first build was `eb663e0`). `lintian` exit 0 on both debs;
      `systemd-analyze verify` on both extracted units is clean; the
      extracted binary reports that version; `config.yaml` carries the two
      commented keys.
- [x] 2.2 Install on hogan (maintainer's go) with no `/etc/pve/vtpm-ek`: VM
      399 enrolls into a scratch directory as before against its local pin,
      the journal shows no cluster-pin line (D-e: `no cluster pin … using
      local pin` only when the directory exists), SPIRE's material untouched
      2026-10-26 07:41:15, the maintainer's go on ask `1dkqzeeb…`: `dpkg -i`
      of `0.3.2+git20261025.d6c40ce6070c`, both units active, `/health` `ok`,
      `/etc/pve/vtpm-ek` absent. At 09:11 an enrolment of VM 399 over SSH
      (`agent-cursor-devid-6741`): hogan logged `enroll/start 200`,
      `devid issued … vm=399 ek=19517bd1…` and `enroll/finish 200` from
      `169.254.225.141`, and no cluster-pin, pin, `401` or `500` line. In the
      guest `/var/lib/spire/agent/devid.*` still date from 2026-10-25 07:54
      and `spire-agent` has not restarted since 2026-10-25 14:53. The
      guest-side `rc` and the scratch path were not reported on the bus and
      are not recorded here.
- [ ] 2.3 After P1-w's backfill has written `/etc/pve/vtpm-ek/399`: VM 399
      enrolls into a scratch directory, no warning line. A mismatching
      cluster pin is covered by the unit tests only; the live file is not
      altered for it
- [x] 2.4 Time one leg with the cluster directory and guest configs as they are
      on hogan, from the access log
      Access log on hogan, 2026-10-26 09:11, no `/etc/pve/vtpm-ek`: VM 399's
      `enroll/start` 2.2 ms and `enroll/finish` 5.8 ms. With the directory
      present not yet measured (after 2.3).
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
- [x] Review of the implementation by a stronger model before the PR is
      called ready
      **Fable, 2026-10-25, of `bbf6c80..1291692`:** no must-fix.
      Should-fix taken:
      - `Finish` re-read the cluster pin outside `pinCreateMu` and logged
        `invalid cluster pin` twice. The cluster branch now sits under the
        lock in `ensurePinBeforeIssue` and reads quietly.
      - A pin removed between listing and read answered `500`; it is now
        skipped.

      Nits taken: `gofmt` on `config.go`; a directory under a VM ID's name
      is no pin; handler tests for both legs (`TestClusterPinHandlers`); the
      late-cluster-pin and finish-mismatch test
      (`TestEnsurePinHonoursLateClusterPin`); named subtests; both keys in
      the man page's CONFIGURATION.

- [x] Fable review of D-g's rationale (coordinator, 2026-10-25,
      `ystzzfqg…`), at the maintainer's request: keep D-g, with conditions.
      - Must-fix taken: D-g is stated as restore convenience against a lost
        operator touchpoint, with the rights shift in Risks; VM ID reuse is
        unsafe for SPIRE until #89's reconciler, so the operator must evict
        the old agent and check the alias, stated in the design, the spec
        delta and `vtpm-mds(8)`.
      - Should-fix taken: `guestExists` looks a second time after 250 ms,
        with the migration window in Risks (`TestGuestExistsRechecks`); one
        log line per ignored stale pin (`ignored_stale=`, tested).
      - The README is the coordinator's.

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change
