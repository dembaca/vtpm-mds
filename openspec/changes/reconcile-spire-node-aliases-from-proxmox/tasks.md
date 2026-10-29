# Tasks

Written on 2026-10-29 by `agent-claude-vtpm-05a6b5e52` against
`main@ccd0f47`, from the maintainer's decisions A-1 to A-4 (A-5/A-6
deferred). Record only until the maintainer's go.

## 1. Record

- [x] 1.1 `proposal.md`, `design.md`, this file, deltas for `spire-reconciler`
      (ADDED) and `devid-enrollment` (MODIFIED issuance line)
      **Verify:** `openspec validate --all --strict`: Totals 7 passed, 0 failed
      (2026-10-29, `0000d99`)
- [x] 1.2 Review of the record by a stronger model (Fable), findings taken or
      deferred by the maintainer
      **Fable, 2026-10-29, of `0000d99`:** all taken. Must-fix: pools that
      collide after lowercasing grant nothing; nested pools not served; a
      fingerprint may identify only one cluster and no root may be configured
      (D-f), since `ca:fingerprint` covers the root; only entries parented by
      the server are owned (D-a). Should-fix: privilege check before every
      pass with the propagate flag; an empty listing is always doubtful; what
      makes an entry wanted (D-g); one alias per tag value; `.`/`..` and
      config names as segments; `user-tag-access`/`registered-tags` in the
      derivation; cluster rename; `-once` and restarts never ban. Nits: the
      validate output recorded; renewals leave old agents (Risks); facts read
      from source.
- [x] 1.3 Maintainer's answers to Q-1 and Q-2
      **Verify:** Q-1 (ask `s67xd1uz…`, 22:38): reused VM IDs are a change
      of their own, by way of forwarded `leaf=` lines. Q-2 (ask `jpwfsysg…`,
      22:49): a third `.deb` and a static binary in a follow-up change.
- [x] 1.4 Maintainer's go for implementation
      **Verify:** ask `srpayine…`, 👍 2026-10-29 22:51

## 2. Before code

- [x] 2.1 Against a real cluster, read-only: `/cluster/resources?type=vm`
      returns `pool` and `tags` per qemu guest; a `PVEAuditor` token on `/`
      sees all guests and pools; a token without it gets a short list, not
      `403`; `/access/permissions` shows the privileges D-d checks
      **Verify:** the API answers, recorded here without site names
      **2026-10-29, two tokens of one `@pve` user with privilege separation,
      made by the maintainer** (`probe`: `PVEAuditor` on `/` for user and
      token; `noacl`: no token ACL). `probe`: `200`, every guest listed, keys
      include `vmid` (a JSON **number**), `node`, `pool` (absent when none),
      `tags` (separated by `;` and by spaces), `template` (`0`/`1`: templates
      are listed as qemu, hence D-b's template rule); `/access/permissions`
      gives `VM.Audit: 1` and `Pool.Audit: 1` on `/`, `/vms` and `/pool`.
      `noacl`: `/cluster/resources` `200` with an **empty list**, not `403`;
      `/access/permissions` an empty map; `/pools` `200`, empty. Not seen: a
      nested pool (none exists there).
- [ ] 2.2 Against SPIRE 1.14.x: `ListAgents` output for a `tpm_devid` agent
      (selectors, anything that ties it to an EK), entry create/delete under one
      SPIFFE ID with two selector sets, `BanAgent` then re-attestation refused
      **Verify:** lab output recorded here
      **Partly done, 2026-10-29, local scratch SPIRE 1.14.0** (sqlite, own
      socket, `join_token` agents, run as the agent user and stopped after):
      two node aliases under `…/pve/cluster-1/vm/350` with
      `subject:cn:350` + `ca:fingerprint:aaaa` and `…:bbbb` were both created
      and `entry show -parentID spiffe://<td>/spire/server` lists both; the
      same ID with the same selector set again is refused
      `AlreadyExists, "similar entry already exists"`. `agent show` holds
      SPIFFE ID, attestation type, SVID expiry and serial, `Can re-attest`,
      and no selector for a `join_token` agent. A running agent banned with
      `agent ban` failed its next sync 5 s later with `PermissionDenied` on
      `SyncAuthorizedEntries` and shut down (`Agent is banned: removing SVID
      and shutting down`). **Still open, needs the cloud lab:** `agent show`
      for a `tpm_devid` agent (which selectors, anything tying it to an EK),
      and that a banned `tpm_devid` agent ID cannot attest again with the
      same LDevID.

## 3. Implementation

- [x] 3.1 `logIssuance` writes `leaf=<sha1>`; test that it equals the SHA-1 of
      the returned leaf DER
      **Verify:** `go test ./internal/devid/ -run 'LoggedOnce|Issuance'`:
      both PASS (2026-10-29); `go vet ./...` and `go test ./...` green.
      `debian/vtpm-mds.8` names only the `ek=` value of the line, so it is
      unchanged.
- [x] 3.2 `internal/reconcile`: desired-alias computation (D-b, D-c, D-g) as a pure
      function, table tests for every scenario of the delta
      **Verify:** `go test ./internal/reconcile/`: PASS (2026-10-29),
      `Wanted` in `wanted.go`, subtests for every scenario of the first two
      requirements plus dot segments, separators, a tag without value, two
      intermediates, no fingerprints, duplicates and sorting. D-g's
      field-default check belongs to the writer (3.4).
- [x] 3.3 Proxmox reader with the permission check and the doubtful-read rule
      (D-d), tests on a fake API
      **Verify:** `go test ./internal/reconcile/ -run TestRead`: PASS
      (2026-10-29): a good read, an empty listing, only templates, no rights
      (resources not requested), privilege without propagation, server error
      (no token in the message), unreachable, vmid as a number. Response
      shapes from task 2.1.
- [ ] 3.4 SPIRE writer: diff under the prefix, create/delete, bans after two
      reads (D-e), tests on a fake entry and agent API
      **Decision half done (2026-10-29):** `Diff` and `BanTracker` in
      `diff.go`, pure; `go test ./internal/reconcile/ -run 'TestDiff|TestBanTracker'`
      PASS (a removed tag, entries outside the prefix, a workload entry under
      it, a hand-edited alias, duplicates, `cluster-10` vs `cluster-1`; a
      destroyed VM, a VM missing once, another cluster's agent, counts dropped
      for vanished agents). **Open:** the gRPC adapter to the SPIRE server API,
      tested against a scratch SPIRE 1.14.0. 3.5 must refuse a cluster without
      an intermediate, or `Diff` would delete all its aliases.
- [ ] 3.5 `cmd/vtpm-mds-reconcile`: strict config (D-i, D-f), `-version`,
      `-once` for a single pass that never bans (D-e), signal handling
- [ ] 3.6 `vtpm-mds-reconcile(8)` (D-j), `README.md`, `ARCHITECTURE.md`
      **Verify (3.x):** `go vet ./...`, `go test ./...`,
      `openspec validate --all --strict`

## 4. Verification

- [ ] 4.1 Lab: two-tier PKI, SPIRE 1.14.x, a fake or real Proxmox API: a VM
      with pool and tags gets its aliases and a workload under the tag alias
      gets an SVID; removing the tag removes it within one interval plus sync;
      a VM without a pool gets only `vm/`; a destroyed VM's agent is banned
      after two reads
- [ ] 4.2 Real cluster, read-only against Proxmox, against a scratch SPIRE
      server: one `-once` pass, aliases listed, nothing banned that is alive

## 5. Ship

- [ ] 5.1 Review of the implementation by a stronger model before ready
- [ ] 5.2 After merge, the coordinator archives the change
