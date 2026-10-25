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

- [ ] 1.1 Confirm the format of `/etc/pve/.vmlist` on one node (root, or a
      `systemd-run` unit with the daemon's `User=` and
      `SupplementaryGroups=www-data`), recording only its structure, not VM
      names; verify the agent can describe the parser against it
- [ ] 1.2 Add `mds.cluster_pin_dir` (default `/etc/pve/vtpm-ek`) and
      `mds.require_cluster_pin` (default `false`) with the load error for
      `require_cluster_pin: true` and an empty directory (D-j); verify with
      config tests: defaults when omitted, strict decode, the load error
- [ ] 1.3 Read the caller's cluster pin with the strict format (D-d) and put
      it between inventory and local pin (D-c); verify with unit tests on a
      constructed directory: valid pin enforced, mismatch body, upper case /
      second line / empty / non-numeric name are no pin with the
      `invalid cluster pin` line, cluster pin over a differing local pin with
      the `local pin differs` line and the local file unchanged
- [ ] 1.4 Fallback and switch (D-e): no local pin is created when a cluster
      pin exists; the `no cluster pin … using local pin` line once per leg;
      `require_cluster_pin` refuses both legs with `401` and creates nothing;
      verify with handler tests
- [ ] 1.5 Extend the cross-VM scan with the cluster directory and the
      `.vmlist` filter (D-g), inside the existing `pinCreateMu` for finish;
      verify with unit tests: clone refused by a cluster pin, destroyed VM's
      local and cluster pins ignored, unreadable `.vmlist` ignores nothing and
      logs, YAML backend unchanged, the concurrent-finish test still passes
      with `-race`
- [ ] 1.6 Fail closed (D-f): unlistable cluster directory or unreadable pin
      file answers `500` `pin store unavailable`; missing directory is empty;
      verify with unit tests (mode `0300`, skipped as uid 0)
- [ ] 1.7 Document in `debian/vtpm-mds.8` (EK PINS: cluster pins, precedence,
      the two keys, the log lines, the reused-ID remedy, turning on
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
- [ ] 2.4 Time one leg with the cluster directory and `.vmlist` as they are
      on hogan, from the access log
- [ ] 2.5 Read the same pin from holger's view (P1-w or cursor, read-only):
      the file is the same on every node

## Pre-review

- [ ] Fable review of this proposal before the maintainer's go
- [ ] Review of the implementation by a stronger model before the PR is
      called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change
