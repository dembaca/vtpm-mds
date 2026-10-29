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
- [ ] 1.4 Maintainer's go for implementation

## 2. Before code

- [ ] 2.1 Against a real cluster, read-only: `/cluster/resources?type=vm`
      returns `pool` and `tags` per qemu guest; a `PVEAuditor` token on `/`
      sees all guests and pools; a token without it gets a short list, not
      `403`; `/access/permissions` shows the privileges D-d checks
      **Verify:** the API answers, recorded here without site names
- [ ] 2.2 Against SPIRE 1.14.x: `ListAgents` output for a `tpm_devid` agent
      (selectors, anything that ties it to an EK), entry create/delete under one
      SPIFFE ID with two selector sets, `BanAgent` then re-attestation refused
      **Verify:** lab output recorded here

## 3. Implementation

- [ ] 3.1 `logIssuance` writes `leaf=<sha1>`; test that it equals the SHA-1 of
      the returned leaf DER
      **Verify:** `go test ./internal/devid/ -run Issuance`
- [ ] 3.2 `internal/reconcile`: desired-alias computation (D-b, D-c, D-g) as a pure
      function, table tests for every scenario of the delta
- [ ] 3.3 Proxmox reader with the permission check and the doubtful-read rule
      (D-d), tests on a fake API
- [ ] 3.4 SPIRE writer: diff under the prefix, create/delete, bans after two
      reads (D-e), tests on a fake entry and agent API
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
