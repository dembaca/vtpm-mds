# Tasks

Written on 2026-10-18 against `main` at `24087c4` by `agent-mac-vtpm-mds-687cc7`,
at the coordinator's assignment, after the maintainer's answers of the same day.
**Code only after the coordinator's ACK of this record.**

## 1. Decide

- [x] 1.1 **Maintainer** — answer the coordinator's five questions, and verify the answers are recorded in `design.md` under a dated heading
      **Done, 2026-10-18:** on the agent bus; recorded under "Decided (maintainer,
      2026-10-18)".
- [x] 1.2 **Maintainer, optional** — overrule or accept D-a to D-h in `design.md`, and verify the outcome is recorded there
      The ones most likely to be wanted differently: the prefix matched without
      regard to case (D-b), the 63 and 255 character limits (D-a), and `404` for a
      VM without tags (D-d).
      **Closed at the archive, 2026-10-18:** the maintainer did not overrule any of
      D-a to D-h; he merged #46 with them as written.
- [x] 1.3 **Maintainer** — answer the open question in `design.md` (does each Kubernetes node read its own zone from the service), and verify the sentence is recorded as a goal or dropped
      **Dropped from this change, 2026-10-18,** not answered: the choice is
      about the consumer, outside this repository, and this record never
      depended on it. What was learned about the consumer is under "Open
      question" in `design.md`.
- [x] 1.4 **Coordinator** — ACK the record and the deltas, and verify the ACK is on the bus before any code is written
      **Done, 2026-10-18:** ACK on the bus for #46 at `2669a0b`, with two notes
      (a log line for former declarations, and the lab naming) taken into D-h and
      "What the lab no longer shows".

## 2. Implement

- [x] 2.1 Parse instance tags from the live `tags` key into a `Tags` field on `inventory.VMConfig`, and remove the placement tags, and verify with unit tests on constructed `/etc/pve` trees for every scenario of `vm-inventory`
      **Done, 2026-10-18** (`c06a72a`): `internal/proxmox/vmconfig.go`
      `instanceTags`, `internal/inventory/tags.go` and `VMConfig.Tags`. Twelve tests
      in `vmconfig_test.go`, one per Proxmox scenario: a tag, dots in a value, the
      prefix in capitals, the three separators, two tags with one key, an invalid
      tag beside a valid one, a single invalid tag (five shapes), the limits, a
      snapshot, none carries nil, a former placement tag (logged), a hand-written
      `zone:` line. `go test ./internal/proxmox` ok.
      Each scenario of the Proxmox requirement that concerns tags is one test:
      dots in a value, the prefix in capitals, two tags with one key, an invalid
      tag, a tag in a snapshot, a former `mds.zone.` tag, a hand-written `zone:`
      line.
- [x] 2.2 Read the node description under `topology.vtpm_mds.io/`, and verify the existing host-placement tests pass with the new prefix and that a `topology.kubernetes.io/` line is ignored
      **Done, 2026-10-18:** the description prefix is `topology.vtpm_mds.io/`; the
      host-placement tests run with it and `TestTheFormerDescriptionPrefixIsNotRead`
      shows `topology.kubernetes.io/` lines alone give the node and cluster names.
- [x] 2.3 Add `tags` to the YAML inventory and remove `zone` and `region`, and verify with unit tests for every YAML scenario
      **Done, 2026-10-18:** YAML `tags` mapping, `zone` and `region` read only to
      log. Tests: `TestLoadYAMLTags`, `TestLoadYAMLIgnoresAnInvalidTag`,
      `TestLoadYAMLFormerZoneAndRegionPlaceNothing`, `TestValidTagKeyAndValue`.
- [x] 2.4 Serve `tags/`, `tags/instance` and `tags/instance/{key}`, list `tags/` in the index only for a VM that has some, and make placement the host's for every caller, and verify with handler tests for every `instance-metadata` scenario, including that an unknown key is not the index
      **Done, 2026-10-18:** `imds/tags.go` (`HandleTags`, `HandleInstanceTagKeys`,
      `HandleInstanceTag`), three routes in `internal/server/server.go`, placement
      from `inventory.HostPlacement()` only. `imds/tags_test.go` (eight tests),
      the placement tests rewritten, and `TestTagsPathsAreRegisteredNotTheIndex` in
      `internal/server`, which shows the paths are not the index and that
      `tags/other` still is.
- [x] 2.5 For each new test, run it against the code before this change or mutate the new code, and verify it fails
      **Done, 2026-10-18**, by mutation, each applied to the committed code and
      restored: tags never parsed (6 tests fail), case-sensitive prefix (2), first
      duplicate wins (2), value cut at its last dot (1), index always lists `tags/`
      (3), unknown key served `200` (2), placement read from the VM record again
      (1), tags in the identity document (2). A first attempt at the unknown-key
      mutation did not compile and counted for nothing; it was redone.
      Tests that cannot compile against the old code are shown by mutation, as in
      `serve-zone-and-region-per-vm`.
- [x] 2.6 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass
      **Done, 2026-10-18:** `go vet` and `go test` for `.`, `./cmd/devid-enroll`,
      `./imds` and `./internal/...` pass; `openspec validate --all --strict` →
      6 passed; a trial `openspec archive` on a scratch copy applies the deltas
      (+3 ~2 -2) and leaves 5 passed. `cmd/prox-mds` is a local ignored leftover
      that does not build, so `go test ./...` is not used.

## 3. Verify on the real systems

- [x] 3.1 **Hogan** — install the git-stamped package, tag VM 399 with `vtpm_mds.io.owner.team-a` and `vtpm_mds.io.rack.b.2`, reload, and verify from the guest that `tags/instance` lists both keys, each value is served with its dots, and the index lists `tags/`; then remove the tags
      **Done, 2026-10-18, hogan (coordinator), against `0.2.0+git20261018.eaed1d1b54b6`,
      with the maintainer's go:** passed, in two rounds. VM 399 carried the tags
      `owner` as `team-a`, `rack` as `b.2` and two more the maintainer had set
      (an `fqdn` and a `public_ipv4`, each a `vtpm_mds.io.<key>.<value>` tag), one
      per entry of the tag field. From the guest: the index lists `tags/`;
      `tags/` answers `instance`; `tags/instance` lists the four keys; `owner`
      reads `team-a` and `rack` reads `b.2`, the dot kept, and values with dots and
      `-` come through unchanged; an unknown key answers `404` with an empty body,
      not the router's; the identity document carries no tag field. The log
      reported the former placement tags of VM 300 as `ignored legacy placement
      tag … topology now comes from the node` (D-h). The tags stay in the VM's
      config until the maintainer removes them (3.3).
      Changes a shared resource, VM 399's config, so it needs the maintainer's go
      in the session of whoever runs it.
- [x] 3.2 **Hogan** — put `topology.vtpm_mds.io/zone: TEST_Z` into the node notes, reload, and verify a guest reads `TEST_Z` through the key and the document, with the region from the cluster name, and that the old `topology.kubernetes.io/zone` line alone gives the node name
      **Done, 2026-10-18, hogan (coordinator), against `eaed1d1`, in three
      rounds:** passed. (1) The former line `topology.kubernetes.io/zone: TEST_Z`
      alone no longer counts: the zone is the node name. (2) The maintainer put
      `topology.vtpm_mds.io/region: bgl` into the notes and the service served
      `bgl`; with two `topology.vtpm_mds.io/zone` lines (`hogan` and `TEST_Z`) the
      log read `host placement: 2 zone lines in the description of
      /etc/pve/local/config, none used` and the zone fell back to the node name, as
      "Two description lines of one kind declare nothing" says. (3) With exactly
      one zone line, `keller`, a value that is not the node name,
      `placement/availability-zone` answered `keller` and `placement/region`
      answered `bgl`, with no host-placement warning in the log: the zone is read
      from the description and the region too.
- [x] 3.3 **Hogan** — verify a VM without tags gets `404` with an empty body on `tags/instance`, and that an unbound caller (the host itself) gets the same
      **Done, 2026-10-18, hogan, in two rounds:** passed. The host as an unbound
      caller got `404` with an empty body on `tags/` and `tags/instance`, and its
      index had no `tags/`. After the maintainer removed every tag from VM 399
      (the `tags:` line is left empty and parses), the index of VM 399 listed
      `instance-id`, `local-hostname`, `local-ipv4`, `placement/` and `services/`
      and no `tags/`; `tags/`, `tags/instance` and `tags/instance/no-such-key`
      each answered `404` with an empty body; `instance-id` stayed `i-399` and the
      identity document kept its fixed keys.
- [x] 3.4 **Cloud lab** — declare `tags` for one lab inventory entry, and verify a guest reads the keys and values through `tags/instance` and that an entry without tags gets `404`; say which layout was used
      **Done, 2026-10-18, `agent-cursor-vtpm-22e0`, against `eaed1d1`:** passed.
      Layout `swtpm-localca` and `/usr/share/OVMF/`; the inventory is
      `/var/lib/mds-lab/inventory/lab.yaml`, outside the repository. A YAML entry
      with `tags` `owner`, `rack` and `fqdn` listed `tags/` in the index and served
      each key and value (`team-a`, `b.2`, a lab name) with `200`. Placement was
      the host's only, `proxmox` and `local`. After a SIGHUP without tags the index
      had no `tags/` and every tags path answered `404` with an empty body.
      **Not shown in the lab:** the Proxmox half, which only hogan can show.
      Declare them in the inventory outside the repository.
- [x] 3.5 Say what was not verified: a data-centre node's real description, a Kubernetes consumer, EC2's answer for an instance without tags
      **Done, 2026-10-18.** Not verified: a data-centre node's real description
      (the zone and region lines were read on hogan's own notes only); a
      Kubernetes consumer of the tags or of the topology; EC2's answer for an
      instance without tags (the `404` is design decision D-d, not a measured
      EC2 behaviour); a VM whose tag field holds a value over 255 characters on
      the live stack (covered by unit tests only); the Proxmox tag format on the
      installed version beyond the tags that were actually set.

## 4. Archive

- [x] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology and tags, and verify `openspec validate --all --strict` is still clean
      **Done, 2026-10-18,** by the coordinator from `main` at `210e906`, after
      #46 was merged: `openspec archive` applied `instance-metadata` +1 ~2 and
      `vm-inventory` +2 -2. `AGENTS.md` and the README feature table updated.
