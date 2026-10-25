# Tasks

Written on 2026-10-25 against `main@0a3d9d3` (v0.3.1) by
`agent-vtpm-hogan-d3e6cbc82`, from the maintainer's decision "A" on ask
`gox8yerp…` (bgl-infra TASK 2.3c). Files: `internal/devid/auth.go`,
`internal/devid/pin.go`, `internal/server/server.go`, their tests, `debian/vtpm-mds.8`, `debian/changelog`
and this record; the README pin row is the coordinator's.

**Proposal only: implementation waits for the maintainer's go.**

## 1. Implement

- [x] 1.1 Look up the caller's EK fingerprint in the pin store and the
      inventory, excluding the caller's own VM ID (D-b); verify with unit
      tests on a constructed store: other VM pinned, only own pin, empty file,
      file with an invalid name or a `.tmp-` name, inventory `ek_sha256` of
      another VM through the `InventoryPins` seam wired in `server.go`
      **Done, 2026-10-25:** `Enroller.pinnedToAnotherVM` in `pin.go`, seam
      `Enroller.InventoryPins`, wired as `inventoryPins` (reads
      `vmConfigCache` under its lock). `TestPinnedToAnotherVMScan` (other VM
      pinned in upper case, own pin only, empty file, `.390.tmp-00ff`,
      `..x`, a subdirectory, another EK, inventory pin through the seam,
      store before inventory, missing directory) and
      `TestPinFileCountsBesideInventoryOverride` pass;
      `TestInventoryPinsFromCache` and the `InventoryPins` assertion in
      `TestCredentialReplacesConfiguredKeyPath` cover the wiring in `internal/server`.
- [x] 1.2 Refuse both legs with `401` and the D-c body and log line, before
      session and request checks (D-a, D-c); verify with handler tests that
      no challenge is issued, no pin file is created and the log line names
      both VM IDs and no fingerprint
      **Done, 2026-10-25:** in `AuthenticateEnrollCaller` after the own-pin
      check. `unauthorizedError` carries a log-only `detail`;
      `responseBody` keeps it out of the body in `writeEnrollAuthError` and
      in `HandleFinish`. `TestEnrollLegsRefuseEKPinnedToAnotherVM` (store
      and inventory): both legs `401` `EK certificate is pinned to another
      VM`, no session, no `ek-pins/100`, exactly one line per leg ending in
      `devid enroll auth: EK certificate is pinned to another VM vm=100
      pinned=399`, fingerprint absent. `TestPinHolderStillAuthenticates`
      passes.
- [x] 1.2a Repeat the scan in `ensurePinBeforeIssue` under a mutex that covers
      the create (D-a2); verify with a concurrent-finish test: two VM IDs, one
      EK, one pin, at most one LDevID
      **Done, 2026-10-25:** package-level `pinCreateMu`; a nil-in-production
      hook `beforePinCreate` holds the window between scan and create open.
      `TestConcurrentFirstFinishOneEKTwoVMs` (VMs 390 and 399, five rounds)
      passes with `-race`: one LDevID, one refusal, one pin file. With the
      two mutex lines commented out it fails, `round 0: issued=2
      refused=0`.
- [x] 1.3 Answer `500` `pin store unavailable` when the directory cannot be
      listed (D-d); verify with a unit test on an unreadable directory,
      skipped when run as uid 0
      **Done, 2026-10-25:** `TestUnlistablePinDirFailsClosed` (mode `0300`,
      run as `coding-agent`, not skipped): both legs `500` `pin store
      unavailable`, `ensurePinBeforeIssue` returns `ErrPinStore`. A pin file
      that is listed but cannot be read also fails closed (`readStoredPin`).
- [x] 1.4 Document the operator steps for a VM restored under a new ID and
      for a refused clone (empty `tpmstate`, proposal Impact) in
      `debian/vtpm-mds.8` (D-e); verify with `man -l`
      **Done, 2026-10-25:** new section `EK PINS` (clone, restore under a new
      ID, replaced vTPM, unlistable store, per node) and a FILES entry for
      `/var/lib/vtpm-mds/ek-pins`; `man -l debian/vtpm-mds.8` renders, no
      `--warnings` output. The host log line documented is the D-c one, not
      the proposal's earlier `devid refused ek pinned to another vm`.
- [x] 1.5 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`
      **Done, 2026-10-25:** vet clean; every package `ok`
      (`internal/devid` 104.5 s); validate `8 passed, 0 failed`.

## 2. Verify on hogan

- [x] 2.1 Build with `make deb-local` from a tree rebased on `main`;
      `dpkg-deb -c`/`-I`, `lintian --fail-on error,warning`,
      `systemd-analyze verify`
      `vtpm-mds_0.3.1+git20261025.cda26ac97bed_amd64.deb` from `cda26ac` on
      `main@0a3d9d3`: lintian exit 0, `systemd-analyze verify` on the
      extracted unit clean, the extracted binary reports that version.
- [x] 2.2 Install on hogan (maintainer's go); VM 399 enrolls into a scratch
      directory unchanged (own pin), SPIRE's material untouched
      2026-10-25 11:03:57, the maintainer's go on ask `3e6hqhmu…`: `dpkg -i`
      over 0.3.0, `vtpm-mds` and `vtpm-mds-bind` active, `/health` `ok`. At
      11:19 `devid-enroll -out <scratch>` in VM 399 (over SSH, by
      `agent-cursor-devid-4d15`): `rc=0`; hogan logged `enroll/start 200`,
      `devid issued … vm=399`, `enroll/finish 200` and no refusal for any VM.
      `/var/lib/spire/agent` unchanged, `spire-agent` not restarted, the
      scratch directory removed.
- [ ] 2.3 A clone of 399 that kept its vTPM state (bgl-infra, own ask) is
      refused on both legs with the D-c body and log line, and no pin file
      for the clone appears
      2026-10-25 13:16:24, clone 392 of 399 with its vTPM state (bgl-infra,
      ask `a4reqfw8…`): hogan logged `devid enroll auth: EK certificate is
      pinned to another VM vm=392 pinned=399` and `enroll/start 401`, and
      nothing else for 392 (no `enroll/finish`, no `devid issued`). Only the
      start leg was reached, since it refused; the finish leg is covered by
      `TestEnrollLegsRefuseEKPinnedToAnotherVM`. No pin for 392 is inferred
      (pins are created only in finish); left open until a root `ls` of
      `ek-pins/` confirms it, which this user cannot read.
- [x] 2.4 Time one leg with the store as it is on hogan
      Access log on hogan: the refused `enroll/start` took 0.95 ms, VM 399's
      accepted `enroll/start` 5.0 ms and `enroll/finish` 8.4 ms, with the
      store holding the pins of this node's VMs.

## Pre-review

- Fable, 2026-10-25, of the proposal (`ae5b3f7`): no must-fix. Should-fix
  taken: the race between two first finishes (D-a2, new scenario), the
  inventory seam in `server.go`, one log line through the existing auth log,
  a pin file counts even where its own VM's inventory pin overrides it.
  Nits taken: the bounded oracle and the `/health` gap in Risks,
  `validateVMID` named, the uid-0 skip.
- Fable, 2026-10-25, of the implementation (`e93962c`): no must-fix;
  should-fix taken: only VMID-shaped names count, comment on the
  inventory-pin path; nits taken: both log prefixes and the consumed session
  in the man page, deterministic race test.
- [x] Review of the implementation by a stronger model before the PR is
      called ready
      Fable, above; fixes in `cda26ac`.

## 3. Archive

- [x] 3.1 After merge, the coordinator archives the change
      Archived 2026-10-25 by `agent-vtpm-hogan-d3e6cbc82` at the coordinator's
      request (bgl-infra TASK 0.3.2r), with 2.3 still open for a root check of
      `ek-pins/` after clone 392 is destroyed.
