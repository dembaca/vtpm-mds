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

- [x] 2.1 Parse the `tags` key of the live section into `zone` and `region` in `internal/proxmox/vmconfig.go`, with the rules of the delta, and verify with tests on constructed trees that each Proxmox scenario of the delta holds, including two tags, an invalid tag, a snapshot tag and a hand-written `zone:`/`region:` line
      **Done:** `placementFromTags` and `placementOfKind` in
      `internal/proxmox/vmconfig.go`; `zone` and `region` lines are no longer
      stored. Tests, one per scenario: `TestAZoneTagBecomesTheVMsZone`,
      `TestTagsAreSplitAtSemicolonCommaAndSpace`, `TestTwoZoneTagsDeclareNothing`,
      `TestAnInvalidTagNextToAValidOneDeclaresNothing`,
      `TestAnInvalidPlacementTagIsIgnored`, `TestAPlacementTagInASnapshotDoesNotCount`,
      `TestAHandWrittenZoneAndRegionAreNotStored` (both `:` and `=`).
- [x] 2.2 Determine the host placement (node from `/etc/pve/local`, cluster from `/etc/pve/corosync.conf`) on every load and carry it with the cache, and verify with tests on constructed trees for a cluster file, no cluster file, a name that is not a valid placement name, and a SIGHUP reload
      **Done:** `proxmox.HostPlacement(pveRoot)` (link target, `cluster_name`),
      `inventory.SetHostPlacement`/`HostPlacement`, set by
      `Server.RefreshVMConfigCache` after every successful load (YAML: the
      default), so a failed reload keeps the old value and SIGHUP refreshes it.
      Tests: `TestTheHostPlacementIsThisNodeAndThisCluster`,
      `TestAHostWithoutAClusterFileHasRegionLocal`,
      `TestAClusterFileWithoutAClusterNameHasRegionLocal`, `TestHostNamesAreLowercased`,
      `TestAnInvalidHostNameFallsBackToTheDefault`, `TestASuccessfulLoadSetsTheHostPlacement`.
      **Added while implementing:** names are lowercased before the check (D-h,
      the delta has the scenario).
- [x] 2.3 Add `zone` and `region` to the YAML loader, and verify with tests that a valid value reaches the record, an invalid one is logged and ignored, and an invalid one does not stop the load
      **Done:** `zone` and `region` in the YAML loader, logged and ignored when
      invalid. Tests: `TestLoadYAMLZoneAndRegion`, `TestLoadYAMLIgnoresAnInvalidZone`,
      `TestValidPlacementName`.
- [x] 2.4 Serve `placement/region`, resolve zone and region per caller in `imds/handlers.go` and `internal/server/server.go`, and make the identity document use the same two functions, and verify with handler tests for every scenario of the `instance-metadata` delta, including an unbound caller and one value declared
      **Done:** `HandleRegion`, `placementOf`, the route in `server.go`; the
      document uses the same two functions. Tests in `imds/placement_test.go`
      (declared, nothing declared, one declared, unbound, invalid value,
      document agrees, document keeps `local`/`proxmox`, region needs a token) and
      `TestPlacementRegionIsARegisteredKey` against the real server's routes.
- [x] 2.5 Verify that the new tests fail on `main`'s code, by running them against the old files, so they are shown to catch the defect
      **Done by mutation, not by the old files** (the new tests do not compile
      against them): four deliberate defects, each put in, run and taken out again.
      A raw `zone` line stored fails `TestAHandWrittenZoneAndRegionAreNotStored`;
      the first of several tags winning fails `TestTwoZoneTagsDeclareNothing` and
      `TestAnInvalidTagNextToAValidOneDeclaresNothing`; a handler that ignores the
      record fails `TestADeclaredZoneAndRegionAreServed`,
      `TestOneValueDeclaredTheOtherFromTheHost` and
      `TestTheDocumentAndThePlacementKeysAgree`; no lowercasing fails
      `TestHostNamesAreLowercased`. With the real code all pass.
- [x] 2.6 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the result
      **Done:** `go vet` clean and `go test -count=1` `ok` for `cmd/devid-enroll`,
      `imds`, `internal/...` and the root, with the untracked `cmd/prox-mds`
      leftover left out.
- [x] 2.7 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on real artifacts

      **Done:** `openspec validate --all --strict` → 7 passed, 0 failed. The
      fallback validator's problems are `all tasks ticked (--strict)` for the
      tasks of sections 3 and 4, and for the EK change still in `changes/`.
- [ ] 3.1 **hogan, read-only, before any code** — verify as the service's user that `/etc/pve/corosync.conf` is readable and names a cluster, that `/etc/pve/local` links to the node, and record both names
      Decides whether the region default is a cluster name or `local` there.
      **Partly done, 2026-10-17, coordinator on hogan, read-only:** the service runs
      as `root` (unit with `ProtectHome=true` and `NoNewPrivileges=true`, no
      `ProtectSystem` or `InaccessiblePaths` on `/etc/pve`); `/etc/pve/corosync.conf`
      is `0640 root:www-data`, so **readable for the service**; `/etc/pve/local` →
      `nodes/hogan`, **node `hogan`**. **Not read:** the cluster name itself; the
      agent user is neither root nor in `www-data`. It becomes visible in 3.2 on
      the installed service. Not ticked until then.
- [ ] 3.2 **hogan** — with a git-stamped package, verify that VM 399 without tags answers `placement/availability-zone` with the node name and `placement/region` with the cluster name, and that its identity document agrees
- [ ] 3.3 **Maintainer's go needed** — add `mds.zone.<name>` and `mds.region.<name>` tags to VM 399, send SIGHUP, and verify both keys and the document show them; then remove the tags
      Changes a shared resource, VM 399's config, so it needs the maintainer's go
      in the session of whoever runs it. Without it the tag path is verified on
      constructed trees only; say so when archiving.
- [x] 3.4 **Cloud lab** — declare `zone` and `region` for one entry of the lab inventory, and verify a guest reads them through both keys and the document, and that an entry declaring neither reads `proxmox` and `local`
      Say which layout was used (`swtpm-localca`, `/usr/share/OVMF/`).
      **Done, 2026-10-17, `agent-cursor-vtpm-22e0`, against `99cc9f4`:** passed.
      Layout `swtpm-localca` and `/usr/share/OVMF/OVMF_CODE_4M.fd`. Pure
      verification: `zone` and `region` were declared only in the lab inventory
      outside the repository (`/var/lib/mds-lab/inventory/lab.yaml`), nothing
      under `scripts/qemu-lab/` was touched. A guest, `i-100` through
      `169.254.169.254`, read `zone: lab-zone-a` and `region: lab-region-west`
      through `placement/availability-zone` and `placement/region`, and the
      identity document agreed. After a SIGHUP with neither declared, it read
      `proxmox` and `local`, again agreeing with the document.
      **Not shown in the lab:** the Proxmox half (tags, host placement), which only
      hogan can show.

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology as open, and verify `openspec validate --all --strict` is still clean
