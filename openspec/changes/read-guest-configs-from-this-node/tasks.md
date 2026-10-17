# Tasks

Written on 2026-10-17 against `main` at `4a70f76` by the coordinator
(`agent-vtpm-hogan-ead99cd2d`) after the maintainer's go. Code follows only
after the record has been read back on the bus, on this branch, in
`internal/proxmox/` and this change directory.

## 1. Implement

- [x] 1.1 Change `ParseVMConfigs` to take the root that plays `/etc/pve` (`""` meaning `/etc/pve`) and to read `<root>/local/qemu-server`, failing with an error naming that path when it is missing, and verify `go build ./...` succeeds with `internal/server/server.go` unchanged
      **Done.** `ParseVMConfigs(pveRoot string)`, `DefaultRoot = "/etc/pve"`,
      reads `<root>/local/qemu-server` after an explicit `os.ReadDir` on it (Glob
      alone returns no error for a missing directory). `go build ./...` succeeds and
      `git diff` shows `internal/server/server.go` untouched.
- [x] 1.2 Stop reading a config file at the first line whose trimmed form starts with `[`, and verify with a test on a constructed tree that a snapshot section's `netN:` MAC and `name:` do not reach the record while the live ones do
      **Done.** Parsing stops at the first line starting with `[`.
      `TestASnapshotDoesNotBindARemovedNIC` and
      `TestPendingAndSpecialSectionsAreNotLiveConfiguration` pass.
- [x] 1.3 Drop a `public-ipv4` key from every Proxmox record's raw configuration, and verify with a test that a config carrying `public-ipv4: 203.0.113.7`, and one carrying `public-ipv4=203.0.113.7`, each yield a record without that key
      **Done.** `TestAHandWrittenPublicAddressIsNotStored` covers both the `:`
      and the `=` form and checks that other keys survive.
- [x] 1.4 Add tests for node selection on a constructed tree with nodes `aaa` and `bbb` and `local -> nodes/bbb`, and verify the records come from `bbb`; and for a tree without `local`, and verify the load fails and names `<root>/local/qemu-server`
      **Done.** `TestThisNodeIsReadNotTheFirstNodeByName` (`aaa`, `bbb`,
      `local -> nodes/bbb`) and `TestNoProxmoxNodeDirectory` (error names
      `<root>/local/qemu-server`) pass.
- [x] 1.5 Verify every scenario of `Derive VM Records From Proxmox Guest Configuration` with a test, including the non-virtio NIC and the unreadable config file, so the carried scenarios are shown to still hold
      **Done.** One test per scenario, eight in all, including
      `TestANonVirtioNICYieldsNoMAC` and `TestAnUnreadableConfigFileIsSkipped`
      (skipped when run as root, which reads any mode).
      **Counter-check:** with the scan, the missing `break` and the missing
      `public-ipv4` filter put back into a copy of the new code, exactly the five
      tests for new behaviour fail (`NoProxmoxNodeDirectory`,
      `ThisNodeIsReadNotTheFirstNodeByName`, `ASnapshotDoesNotBindARemovedNIC`,
      `PendingAndSpecialSectionsAreNotLiveConfiguration`,
      `AHandWrittenPublicAddressIsNotStored`) and the three carried ones pass.
- [x] 1.6 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the result
      **Done.** `go vet ./...` clean; `go test -count=1 ./...`: `ok` for imds,
      internal/config, internal/devid, internal/inventory, internal/proxmox,
      internal/server.
- [x] 1.7 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass
      **Done.** `openspec validate --all --strict` → `6 passed, 0 failed`.
      `scripts/openspec-validate.py --strict` reports one problem, by design:
      this change's own unticked section 2 and 3 tasks (`7/11`); structure and
      "every task states a verification" pass.

## 2. Verify on hogan

- [x] 2.1 Build a git-stamped package with `make deb-local`, install it with the maintainer's go given in the session, and verify `dpkg -l vtpm-mds` and `vtpm-mds -version` name this tree
      **Done 2026-10-17 on hogan.** `make deb-local` from `5efb34b` (untracked
      settings file moved aside, so no `.dirty`), lintian clean, installed through
      `sudo hogan-lab dpkg-install` with the maintainer's go given in the session
      for this version, unit started by hand (`--no-start`). `dpkg -l vtpm-mds` →
      `ii vtpm-mds 0.2.0+git20261017.5efb34ba374a`; `vtpm-mds -version` and the
      journal's version line name the same tree.
- [x] 2.2 Verify the daemon's startup log reports the same VM and MAC count as before the install (`3 VMs, 5 MAC addresses` on 2026-10-17), and that VM 399 still reads `i-399` from `instance-id`
      `hogan` sorts first and has no snapshots or `public-ipv4:` lines today,
      so this shows nothing broke; the defects themselves are shown by the
      tests in section 1, not here.
      **Done.** Startup log: `Cache refresh complete (proxmox:/etc/pve): 3 VMs,
      5 MAC addresses cached`, identical to the previous package's line at 12:10.
      From VM 399 with a token, `GET /latest/meta-data/instance-id` → `200 i-399`,
      `instance-identity/document` → `200`. As expected, hogan shows no change:
      it sorts first and has no snapshots or `public-ipv4:` lines.
- [ ] 2.3 **Maintainer, optional** — take a snapshot of VM 399, change nothing else, restart `vtpm-mds`, and verify the startup log's MAC count is unchanged and `instance-id` still answers `i-399`; then delete the snapshot
      The agent user's `hogan-lab` wrapper allows no `qm snapshot`. Without
      this task the section handling is verified on constructed trees only;
      say so when archiving.

## 3. Archive

- [ ] 3.1 Archive with `openspec archive`, update `AGENTS.md` where it lists the `vmconfig.go` fixes as deferred, and verify `openspec validate --all --strict` is still clean
