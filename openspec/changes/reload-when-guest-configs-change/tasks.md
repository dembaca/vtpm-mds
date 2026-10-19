# Tasks

Written on 2026-10-19 against `main` at `364bffe` (rebased onto `14be2a8`,
which contains `report-readiness-honestly`, before the code) by
`agent-vtpm-hogan-d3e6cbc82`, at the coordinator's assignment ("reload on new
VMs" from the production-readiness list). **Code only after the coordinator's
ACK of this record.** Files: `internal/server/server.go` (loop start/stop),
a new `internal/server/reload.go`, fingerprint functions in
`internal/proxmox/` and `internal/inventory/`, tests. Not `debian/`, not
`internal/config/` or the `/health` handler (`report-readiness-honestly`).

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-f in `design.md` (D-c, constant or configuration key, is the one most likely to move), and verify the outcome is recorded there
- [x] 1.2 **Coordinator** — ACK the record and the delta on the bus before any code is written, and verify the ACK names this change
      **Done, 2026-10-19 10:53:** `agent-mac-vtpm-mds-36505b`, "ACK on the
      record `reload-when-guest-configs-change` @ `e4ca16b`", draft PR #62,
      with the file split against #61 and "#61 merges first"; D-c stays a
      5 s constant.

## 2. Establish the constraint

- [x] 2.1 Watch `/etc/pve/nodes/*` with inotify on `hogan` and verify whether writes from other nodes produce events
      Done 2026-10-19, 25 s watch on `nodes/{hogan,holger,horst}`: 25 events
      for `nodes/hogan` (local `lrm_status` rewrites), 0 for `holger` and
      `horst`, whose `lrm_status` mtimes advanced during the watch. Remote
      writes are invisible to inotify, so a `.path` unit is ruled out (D-a).

## 3. Implement

- [x] 3.1 Add a fingerprint over exactly the files each backend reads (D-b), and verify with unit tests on constructed `/etc/pve` trees and YAML files that it changes when a `.conf`, the node description, `corosync.conf` or the YAML file changes, and not otherwise
      `internal/inventory/fingerprint.go` (builder, length-prefixed fields,
      a file adds its content or a missing/unreadable marker, so it never
      fails) and `internal/proxmox/fingerprint.go` (`SourceFingerprint`: the
      `local` link target, `local/qemu-server/*.conf`, `local/config`,
      `corosync.conf`). Tests: `TestYAMLFingerprintFollowsContent`,
      `TestFingerprintFieldsDoNotRunTogether`,
      `TestFingerprintUnreadableFileDiffersFromMissing`,
      `TestSourceFingerprintFollowsWhatTheLoaderReads` (a changed, added and
      migrated-out `.conf`, the description, `corosync.conf`, the link; and
      another node's files and `lrm_status` do **not** change it),
      `TestSourceFingerprintFollowsTheLinkTarget`,
      `TestSourceFingerprintWithoutALocalLink`.
- [x] 3.2 Add the polling loop (D-c, D-d, D-e, D-f) that calls `RefreshVMConfigCache` on a changed fingerprint, and verify with tests covering every scenario of the delta that does not need a cluster
      `internal/server/reload.go`: `reloadIfChanged` (one load attempt per
      fingerprint, through `RefreshVMConfigCache`), `reloadNow` for startup
      and `SIGHUP` (records the fingerprint first), and the ticker started in
      `Start` and stopped, and waited for, in `Shutdown`. `server.go` gains
      only the `poll` field and those four call sites. Tests:
      `TestAnAddedVMIsLoadedWithoutSIGHUP` (added VM and MAC),
      `TestNothingChangesNothingIsLogged`,
      `TestABrokenSourceKeepsTheInventoryAndIsLoggedOnce` (one log line over
      four checks, old inventory kept, fixed file reloaded),
      `TestSIGHUPRecordsTheFingerprint`, `TestThePollerReloadsAndStops`. The
      remote-write scenario is `TestSourceFingerprintFollowsWhatTheLoaderReads`
      at unit level; on the cluster it is 4.3.
- [x] 3.3 For each new test, run it against the code before this change or mutate the new code, and verify it fails
      Twelve mutants, all red: never reload after start; reload on every
      check; `reloadNow` not recording; recording only successful loads
      (retries a broken file); `stop` not cancelling (hangs until the test
      timeout); content not hashed; unreadable equal to missing; no length
      prefix; `corosync.conf`, the description, or the link target not
      hashed; a fixed node instead of `local`. Two survived the first round
      (length prefix, link target) and got sharper tests:
      `TestFingerprintFieldsDoNotRunTogether`'s forged-field case and
      `TestSourceFingerprintFollowsTheLinkTarget`. After the Fable pre-review
      two more, removing the poller start from `Start` and the stop from
      `Shutdown`, survived; `TestStartRunsThePollerAndShutdownStopsIt` now
      turns both red (14 of 14).
- [x] 3.4 Run `go vet ./...`, `go test ./...` and `openspec validate --all --strict`, and verify all pass
      `go vet ./...` clean; `go test -race -count=1 ./...` all `ok`;
      `openspec validate --all --strict` 6 passed; `gofmt -l` clean on every
      file this change touches.

## 4. Verify on hogan

- [ ] 4.1 With the package installed, verify that over two minutes without changes the journal shows no reload line
- [ ] 4.2 Change a tag on VM 399 with `qm set` (needs the maintainer's go, or the maintainer runs it), and verify the new `tags/instance/<key>` is served to VM 399 within one interval without `systemctl reload`
- [ ] 4.3 Verify the remote-write case: a config written by another node (a migration to `hogan` by the maintainer, or a `qm set` on another node through the API) is picked up without a reload, and say which was used
- [ ] 4.4 Say what was not verified (e.g. a newly created guest, if none was created; the YAML half if the lab did not run it)

## 5. Verify in the lab

- [ ] 5.1 **Cloud lab** — with the YAML backend, add a VM record, verify it is bound within one interval; make the file unparseable, verify one error line and the old inventory; fix it, verify the reload

## 6. Archive

- [ ] 6.1 After merge, the coordinator archives with `openspec archive reload-when-guest-configs-change` and verifies `openspec validate --all --strict` stays clean
