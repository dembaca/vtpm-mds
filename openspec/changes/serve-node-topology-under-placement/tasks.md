# Tasks

Written on 2026-10-18 against `main` at `407bfca` by `agent-mac-vtpm-mds-3502f6`,
at the coordinator's assignment, after the maintainer's decision of the same day.
**Code only after the coordinator's ACK of this record.**

## 1. Decide

- [x] 1.1 **Maintainer** — decide that any key under `topology.vtpm_mds.io/` is served under `placement/<key>`, `zone` is `availability-zone`, `cluster` and `node` default from the host, and verify the answers are recorded in `design.md` under a dated heading
      **Done, 2026-10-18:** on the agent bus; recorded under "Decided
      (maintainer, 2026-10-18)".
- [x] 1.2 **Maintainer, optional** — overrule or accept D-a to D-i in `design.md`, and verify the outcome is recorded there
      **Done, 2026-10-18:** the maintainer: D-g OK, no strong opinion on D-f,
      "otherwise everything is OK for the go"; D-a to D-e, D-h and D-i stand. The
      outcome is under "Decided (maintainer, 2026-10-18)" in `design.md`.
      The ones most likely to be wanted differently: the YAML topology mapping
      (D-f), the `placement/` listing (D-g), the limit of 64 keys (D-c).
- [x] 1.3 **Coordinator** — ACK the record and the deltas, and verify the ACK is on the bus before any code is written
      **Done, 2026-10-18:** ACK on the bus for #48 at `10eebb2`, with two
      additions (the redirect of `placement`, and case-sensitivity) taken into D-j,
      D-k and the deltas, and with D-f (no YAML mapping) and D-g (listing kept)
      decided as recommended.

## 2. Implement

- [x] 2.1 Read every `topology.vtpm_mds.io/<key>` line of the node description into the host topology, with `zone` as `availability-zone`, the defaults for four keys, the key and value syntax, the duplicate rule and the limit, and verify with unit tests on constructed `/etc/pve` trees for every scenario of `vm-inventory`
      **Done, 2026-10-18** (`eca8ccf`): `internal/proxmox/vmconfig.go`
      `HostTopology` and `descriptionTopology`, `internal/inventory/placement.go`
      (`Topology`). 19 tests in `vmconfig_test.go` for the host topology, one per
      scenario: names and cluster, no cluster file, no cluster name, capitals,
      invalid names, description zone and region, one key, no config file, an
      invalid value, two lines, a key of its own, no rack, long form, short and long
      together, node and cluster overridden, case-sensitive keys and prefix,
      invalid keys and values, the 64-key limit, the former prefix.
- [x] 2.2 Serve `placement/<key>` and `placement/`, answer an absent key `404` with an empty body, and keep the identity document's two fields, and verify with handler tests for every scenario of `instance-metadata`, including that an absent key is not the index
      **Done, 2026-10-18:** `imds/placement.go` (`HandlePlacementKey`,
      `HandlePlacementKeys`, `HandlePlacementRedirect`), routes in
      `internal/server/server.go`. `imds/placement_keys_test.go` (nine tests) and
      `TestPlacementPathsAreRegisteredNotTheIndex` in `internal/server`, which shows
      the key, the listing, the empty `404`, the two EC2 keys still served, and the
      `301`.
- [x] 2.3 For each new test, run it against the code before this change or mutate the new code, and verify it fails
      **Done, 2026-10-18**, by mutation, each applied to the committed code and
      restored: first of two lines wins (2 tests fail), long form not aliased (4),
      key case folded (1), no key limit (1), no node default (4), cluster invented
      as `local` (3), description does not override (5), absent key served `200`
      (1), keys in reverse order (listing test), the document carrying `rack` (3),
      redirect `302` (1). A first attempt at the sort mutation did not compile and
      counted for nothing; it was redone.
      Tests that cannot compile against the old code are shown by mutation, as in
      `serve-vm-tags-as-instance-metadata`.
- [x] 2.4 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass
      **Done, 2026-10-18:** `go vet` and `go test` for `.`, `./cmd/devid-enroll`,
      `./imds` and `./internal/...` pass; `openspec validate --all --strict` →
      6 passed; a trial `openspec archive` on a scratch copy applies the deltas
      (~2) and leaves 5 passed.

## 3. Verify on the real systems

- [ ] 3.1 **Hogan** — install the git-stamped package, put `topology.vtpm_mds.io/rack: keller01` and `topology.vtpm_mds.io/host-id: h-01` into the node notes, reload, and verify from VM 399 that `placement/rack` and `placement/host-id` answer them, `placement/node` and `placement/cluster` answer the host's names, `placement/` lists the keys, and `placement/nothing` answers `404` with an empty body
      Changes a shared resource, the node notes, so it needs the maintainer's go
      in the session of whoever runs it.
- [ ] 3.2 **Hogan** — verify that a description with `zone` and `availability-zone` both set gives the node name and a log line, that the former prefix `topology.kubernetes.io/` alone changes nothing, and that the identity document still has exactly its fixed keys
- [ ] 3.3 **Cloud lab** — verify the YAML backend's defaults and the `404` of `placement/rack` and `placement/node`, and, if D-f is accepted, a `topology` mapping; say which layout was used
- [ ] 3.4 Say what was not verified: a data-centre node's real description, a Kubernetes consumer, the 64-key limit on the live stack

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists topology, and verify `openspec validate --all --strict` is still clean
