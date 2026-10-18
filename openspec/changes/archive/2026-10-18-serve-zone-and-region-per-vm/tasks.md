# Tasks

Written on 2026-10-17 against `main` at `983eb61` by `agent-mac-vtpm-mds-687cc7`,
at the coordinator's assignment, after the maintainer's "M1–M5 alle wie
empfohlen". **Code only after the coordinator's ACK of this record.**

## 1. Decide

- [x] 1.1 **Maintainer** — M1 to M5 decided as recommended, and verify the answers are recorded in `design.md` under a dated heading
      **Done, 2026-10-17:** "M1–M5 alle wie empfohlen" on the agent bus; recorded
      under "Decided (maintainer, 2026-10-17)" in `design.md`.
- [x] 1.2 **Maintainer, optional** — overrule or accept D-a to D-i in `design.md`, and verify the outcome is recorded there
      Silence is acceptance only once the coordinator has read them back.
      **Closed 2026-10-18 by the coordinator.** The maintainer did not overrule
      D-a to D-i. Two of them were changed by his later decisions, already in
      `design.md`: the node description became the middle source, and names keep
      the case of their source (no lowercasing), which reshaped D-a and D-h.

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
      **Superseded by 2.8:** this task lowercased names before the check; the
      maintainer then decided against any case change, and 2.8 removed it.
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
      `TestTheDocumentAndThePlacementKeysAgree`; no lowercasing failed
      `TestHostNamesAreLowercased`, a test 2.8 replaced. With the real code all
      pass.
- [x] 2.6 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the result
      **Done:** `go vet` clean and `go test -count=1` `ok` for `cmd/devid-enroll`,
      `imds`, `internal/...` and the root, with the untracked `cmd/prox-mds`
      leftover left out.
- [x] 2.7 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass
- [x] 2.8 Read the node description as the first source of the host placement and keep every name as spelled, and verify with tests on constructed trees that a description gives zone and region ahead of the names, that one kind leaves the other to the names, that an invalid or doubled line falls back, and that no source's case is changed
      **Done, 2026-10-17,** after the maintainer's "ja" and "kein lc() uc()":
      `descriptionPlacement` in `internal/proxmox/vmconfig.go`, the name syntax
      allows capitals, and the lowercasing of task 2.2 is gone. Tests:
      `TestTheNodeDescriptionGivesTheHostPlacement`,
      `TestADescriptionWithOneKindTakesTheOtherFromTheNames`,
      `TestANodeWithoutAConfigFileUsesTheNames`,
      `TestAnInvalidDescriptionValueFallsBackToTheNames`,
      `TestTwoDescriptionLinesOfOneKindDeclareNothing`, `TestHostNamesKeepTheirCase`,
      `TestACapitalInATagIsKept`. By mutation, as in 2.5: a description step that
      reads nothing, the first of two lines winning, lowercasing put back and no
      URL decoding each turn the matching tests red, and are taken out again.

## 3. Verify on real artifacts

      **Done:** `openspec validate --all --strict` → 7 passed, 0 failed. The
      fallback validator's problems are `all tasks ticked (--strict)` for the
      tasks of sections 3 and 4, and for the EK change still in `changes/`.
- [x] 3.1 **hogan, read-only, before any code** — verify as the service's user that `/etc/pve/corosync.conf` is readable and names a cluster, that `/etc/pve/local` links to the node, and record both names
      Decides whether the region default is a cluster name or `local` there.
      **Partly done, 2026-10-17, coordinator on hogan, read-only:** the service runs
      as `root` (unit with `ProtectHome=true` and `NoNewPrivileges=true`, no
      `ProtectSystem` or `InaccessiblePaths` on `/etc/pve`); `/etc/pve/corosync.conf`
      is `0640 root:www-data`, so **readable for the service**; `/etc/pve/local` →
      `nodes/hogan`, **node `hogan`**. **Not read:** the cluster name itself; the
      agent user is neither root nor in `www-data`. It becomes visible in 3.2 on
      the installed service. Not ticked until then.
      **Completed by 3.2, 2026-10-17:** the service reports the cluster name with
      its **capitals kept**, as the maintainer decided; the name itself is not
      written here. Node `hogan`.
- [x] 3.2 **hogan** — with a git-stamped package, verify that VM 399 without tags answers `placement/availability-zone` with the node name and `placement/region` with the cluster name, and that its identity document agrees
      **Done, 2026-10-17, coordinator on hogan, against `7918274`:** passed. Package
      `vtpm-mds 0.2.0+git20261017.7918274ef686` installed with the maintainer's go and
      started; `dpkg -l` and the start log name the version, and the log shows `3
      VMs, 5 MAC addresses`, as before. VM 399 has no tags and hogan has no node
      `config`, so the names apply: `placement/availability-zone` answered `200`
      `hogan`, `placement/region` `200` with the cluster name in its original
      spelling, `instance-id` `200` `i-399`, and the identity document `200` with
      the same zone and region. **Not shown:** the description step (no node config
      on hogan; task 3.5) and the tag path (task 3.3).
      **Observation, out of scope:** `GET /latest/meta-data/placement/` answers the
      index of `meta-data/`, not a listing of `availability-zone` and `region`, as
      it did before: the index handler answers unknown paths. AWS lists the keys
      there. A possible follow-up record, not part of this change.
- [x] 3.3 **Maintainer's go needed** — add `mds.zone.<name>` and `mds.region.<name>` tags to VM 399, send SIGHUP, and verify both keys and the document show them; then remove the tags
      Changes a shared resource, VM 399's config, so it needs the maintainer's go
      in the session of whoever runs it. Without it the tag path is verified on
      constructed trees only; say so when archiving.
      **Done, 2026-10-17, hogan (coordinator), against `7918274`:** passed, with
      the maintainer's go. VM 399 carried `tags: mds.region.test_r;mds.zone.test_a`
      (set by the maintainer; Proxmox sorts them), the service reloaded with
      `systemctl reload` (SIGHUP, `3 VMs, 5 MAC addresses`). From VM 399:
      `placement/availability-zone` `test_a`, `placement/region` `test_r`,
      `instance-id` `i-399`, and the document `availabilityZone='test_a'
      region='test_r'` agreed. Before, the same VM read the host values, so the
      tag wins over the host placement, and `_` is kept.
      **Found:** Proxmox rejects `-` in a tag (`tags: invalid format - invalid
      characters in tag`), so the first attempt with `test-a` failed. D-a assumed
      otherwise; it is corrected there. The maintainer removes the tags afterwards.
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
      **Repeated with capitals, 2026-10-17, `agent-cursor-vtpm-22e0`, against
      `7918274`:** passed. Layout `swtpm-localca` and `/usr/share/OVMF/`, inventory
      outside the repository. `zone: DUS4` and `region: DUS` were served as
      `DUS4` and `DUS` through both keys and the document, case unchanged; after
      a SIGHUP with neither declared, `proxmox` and `local`.

- [x] 3.5 **A host whose node has a description, maintainer's go needed** — put `topology.kubernetes.io/zone` and `.../region` lines in a node's description (or run on a data-centre node that has them), restart or SIGHUP the service, and verify a guest without tags reads them through both keys and the document, with their case unchanged
      hogan has no node config, so 3.2 cannot show this step. Without 3.5 the
      description step is verified on constructed trees only, with the format
      taken from Proxmox's source; say so when archiving.
      **Done, 2026-10-17, hogan (coordinator), against `7918274`:** passed, with
      the maintainer's go. The maintainer put one line, `topology.kubernetes.io/zone:
      TEST_Z`, in hogan's node notes (`/etc/pve/local/config`, 39 bytes) and
      removed the tags from VM 399 (an empty `tags:` line remains; the parser
      copes). After `systemctl reload`, VM 399 read `TEST_Z` through
      `placement/availability-zone` (step 2, case kept), the region from the
      cluster name (step 3, the description names only the zone), and the
      document agreed; no error in the log. Zone and region resolve one by one,
      as the delta says. This is the description step on a real node. Whether the
      test line stays in the notes is the maintainer's choice.

## 4. Archive

- [x] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology as open, and verify `openspec validate --all --strict` is still clean
      **Done 2026-10-18 by the coordinator** from `main` at `bce1a1a` (the #44
      merge). Results of `openspec archive` and validation are in the archive
      commit; `AGENTS.md` no longer lists zone and region as open.
