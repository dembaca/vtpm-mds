# Tasks

Written on 2026-10-18 against `main` at `24087c4` by `agent-mac-vtpm-mds-687cc7`,
at the coordinator's assignment, after the maintainer's answers of the same day.
**Code only after the coordinator's ACK of this record.**

## 1. Decide

- [x] 1.1 **Maintainer** — answer the coordinator's five questions, and verify the answers are recorded in `design.md` under a dated heading
      **Done, 2026-10-18:** on the agent bus; recorded under "Decided (maintainer,
      2026-10-18)".
- [ ] 1.2 **Maintainer, optional** — overrule or accept D-a to D-h in `design.md`, and verify the outcome is recorded there
      The ones most likely to be wanted differently: the prefix matched without
      regard to case (D-b), the 63 and 255 character limits (D-a), and `404` for a
      VM without tags (D-d).
- [ ] 1.3 **Maintainer** — answer the open question in `design.md` (does each Kubernetes node read its own zone from the service), and verify the sentence is recorded as a goal or dropped
- [ ] 1.4 **Coordinator** — ACK the record and the deltas, and verify the ACK is on the bus before any code is written

## 2. Implement

- [ ] 2.1 Parse instance tags from the live `tags` key into a `Tags` field on `inventory.VMConfig`, and remove the placement tags, and verify with unit tests on constructed `/etc/pve` trees for every scenario of `vm-inventory`
      Each scenario of the Proxmox requirement that concerns tags is one test:
      dots in a value, the prefix in capitals, two tags with one key, an invalid
      tag, a tag in a snapshot, a former `mds.zone.` tag, a hand-written `zone:`
      line.
- [ ] 2.2 Read the node description under `topology.vtpm_mds.io/`, and verify the existing host-placement tests pass with the new prefix and that a `topology.kubernetes.io/` line is ignored
- [ ] 2.3 Add `tags` to the YAML inventory and remove `zone` and `region`, and verify with unit tests for every YAML scenario
- [ ] 2.4 Serve `tags/`, `tags/instance` and `tags/instance/{key}`, list `tags/` in the index only for a VM that has some, and make placement the host's for every caller, and verify with handler tests for every `instance-metadata` scenario, including that an unknown key is not the index
- [ ] 2.5 For each new test, run it against the code before this change or mutate the new code, and verify it fails
      Tests that cannot compile against the old code are shown by mutation, as in
      `serve-zone-and-region-per-vm`.
- [ ] 2.6 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass

## 3. Verify on the real systems

- [ ] 3.1 **Hogan** — install the git-stamped package, tag VM 399 with `vtpm_mds.io.owner.team-a` and `vtpm_mds.io.rack.b.2`, reload, and verify from the guest that `tags/instance` lists both keys, each value is served with its dots, and the index lists `tags/`; then remove the tags
      Changes a shared resource, VM 399's config, so it needs the maintainer's go
      in the session of whoever runs it.
- [ ] 3.2 **Hogan** — put `topology.vtpm_mds.io/zone: TEST_Z` into the node notes, reload, and verify a guest reads `TEST_Z` through the key and the document, with the region from the cluster name, and that the old `topology.kubernetes.io/zone` line alone gives the node name
- [ ] 3.3 **Hogan** — verify a VM without tags gets `404` with an empty body on `tags/instance`, and that an unbound caller (the host itself) gets the same
- [ ] 3.4 **Cloud lab** — declare `tags` for one lab inventory entry, and verify a guest reads the keys and values through `tags/instance` and that an entry without tags gets `404`; say which layout was used
      Declare them in the inventory outside the repository.
- [ ] 3.5 Say what was not verified: a data-centre node's real description, a Kubernetes consumer, EC2's answer for an instance without tags

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology and tags, and verify `openspec validate --all --strict` is still clean
