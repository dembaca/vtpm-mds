# Tasks

Written on 2026-10-17 against `main` at `4a70f76` by the coordinator
(`agent-vtpm-hogan-ead99cd2d`) after the maintainer's go. Code follows only
after the record has been read back on the bus, on this branch, in
`internal/proxmox/` and this change directory.

## 1. Implement

- [ ] 1.1 Change `ParseVMConfigs` to take the root that plays `/etc/pve` (`""` meaning `/etc/pve`) and to read `<root>/local/qemu-server`, failing with an error naming that path when it is missing, and verify `go build ./...` succeeds with `internal/server/server.go` unchanged
- [ ] 1.2 Stop reading a config file at the first line whose trimmed form starts with `[`, and verify with a test on a constructed tree that a snapshot section's `netN:` MAC and `name:` do not reach the record while the live ones do
- [ ] 1.3 Drop a `public-ipv4` key from every Proxmox record's raw configuration, and verify with a test that a config carrying `public-ipv4: 203.0.113.7` yields a record without that key
- [ ] 1.4 Add tests for node selection on a constructed tree with nodes `aaa` and `bbb` and `local -> nodes/bbb`, and verify the records come from `bbb`; and for a tree without `local`, and verify the load fails and names `<root>/local/qemu-server`
- [ ] 1.5 Verify every existing scenario of `Derive VM Records From Proxmox Guest Configuration` with a test, including the non-virtio NIC, so the carried scenarios are shown to still hold
- [ ] 1.6 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the result
- [ ] 1.7 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 2. Verify on hogan

- [ ] 2.1 Build a git-stamped package with `make deb-local`, install it with the maintainer's go given in the session, and verify `dpkg -l vtpm-mds` and `vtpm-mds -version` name this tree
- [ ] 2.2 Verify the daemon's startup log reports the same VM and MAC count as before the install (`3 VMs, 5 MAC addresses` on 2026-10-17), and that VM 399 still reads `i-399` from `instance-id`
      `hogan` sorts first and has no snapshots or `public-ipv4:` lines today,
      so this shows nothing broke; the defects themselves are shown by the
      tests in section 1, not here.
- [ ] 2.3 **Maintainer, optional** — take a snapshot of VM 399, change nothing else, restart `vtpm-mds`, and verify the startup log's MAC count is unchanged and `instance-id` still answers `i-399`; then delete the snapshot
      The agent user's `hogan-lab` wrapper allows no `qm snapshot`. Without
      this task the section handling is verified on constructed trees only;
      say so when archiving.

## 3. Archive

- [ ] 3.1 Archive with `openspec archive`, update `AGENTS.md` where it lists the `vmconfig.go` fixes as deferred, and verify `openspec validate --all --strict` is still clean
