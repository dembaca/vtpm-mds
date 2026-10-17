# Tasks

Written on 2026-10-17 against `main` at `983eb61` by `agent-mac-vtpm-mds-687cc7`,
at the coordinator's assignment, after the maintainer's "M1–M5 alle wie
empfohlen". **Code only after the coordinator's ACK of this record.**

## 1. Decide

- [x] 1.1 **Maintainer** — M1 to M5 decided as recommended, and verify the answers are recorded in `design.md` under a dated heading
      **Done, 2026-10-17:** "M1–M5 alle wie empfohlen" on the agent bus; recorded
      under "Decided (maintainer, 2026-10-17)" in `design.md`.
- [ ] 1.2 **Maintainer, optional** — overrule or accept D-a to D-i in `design.md`, and verify the outcome is recorded there
      Silence is acceptance only once the coordinator has read them back.

## 2. Implement

- [ ] 2.1 Parse the `tags` key of the live section into `zone` and `region` in `internal/proxmox/vmconfig.go`, with the rules of the delta, and verify with tests on constructed trees that each Proxmox scenario of the delta holds, including two tags, an invalid tag, a snapshot tag and a hand-written `zone:`/`region:` line
- [ ] 2.2 Determine the host placement (node from `/etc/pve/local`, cluster from `/etc/pve/corosync.conf`) on every load and carry it with the cache, and verify with tests on constructed trees for a cluster file, no cluster file, a name that is not a valid placement name, and a SIGHUP reload
- [ ] 2.3 Add `zone` and `region` to the YAML loader, and verify with tests that a valid value reaches the record, an invalid one is logged and ignored, and an invalid one does not stop the load
- [ ] 2.4 Serve `placement/region`, resolve zone and region per caller in `imds/handlers.go` and `internal/server/server.go`, and make the identity document use the same two functions, and verify with handler tests for every scenario of the `instance-metadata` delta, including an unbound caller and one value declared
- [ ] 2.5 Verify that the new tests fail on `main`'s code, by running them against the old files, so they are shown to catch the defect
- [ ] 2.6 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the result
- [ ] 2.7 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on real artifacts

- [ ] 3.1 **hogan, read-only, before any code** — verify as the service's user that `/etc/pve/corosync.conf` is readable and names a cluster, that `/etc/pve/local` links to the node, and record both names
      Decides whether the region default is a cluster name or `local` there.
- [ ] 3.2 **hogan** — with a git-stamped package, verify that VM 399 without tags answers `placement/availability-zone` with the node name and `placement/region` with the cluster name, and that its identity document agrees
- [ ] 3.3 **Maintainer's go needed** — add `mds.zone.<name>` and `mds.region.<name>` tags to VM 399, send SIGHUP, and verify both keys and the document show them; then remove the tags
      Changes a shared resource, VM 399's config, so it needs the maintainer's go
      in the session of whoever runs it. Without it the tag path is verified on
      constructed trees only; say so when archiving.
- [ ] 3.4 **Cloud lab** — declare `zone` and `region` for one entry of the lab inventory, and verify a guest reads them through both keys and the document, and that an entry declaring neither reads `proxmox` and `local`
      Say which layout was used (`swtpm-localca`, `/usr/share/OVMF/`).

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology as open, and verify `openspec validate --all --strict` is still clean
