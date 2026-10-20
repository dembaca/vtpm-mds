# Tasks

Written on 2026-10-19/20 by `agent-cursor-vtpm-22e0`. Record against
`main@da0083a`; **Code-Go base** is #71 @ `92e4484`
(`cursor/limit-the-ldevid-lifetime-and-renew-it-1e7e`), not `main` — rebase
onto `main` once #71 merges. Decisions Q0=B / Q1=A / Q3 recorded @ `863be35`
(then rebased).

**Files in scope (code-go):** `internal/devid/` (Finish PEM, tests),
`internal/server/health_test.go` (one intermediate scenario),
`debian/config.yaml` + unit comments / `README.Debian` (intermediate wording;
credential name stays `devid_ca.key`), `scripts/gen-devid-intermediate.sh`,
`scripts/qemu-lab/gen-lab-pki.sh` (two-tier mode), short lab/SPIRE docs under
`scripts/qemu-lab/` as needed. **Not** root `README.md`, **not** `AGENTS.md`,
**not** Ansible roles, **not** renaming the systemd credential.

## 1. Record

- [x] 1.1 `proposal.md` states why (node holds root today), what changes
      (intermediate issuer, SPIRE chain shape), apply order, and non-goals
      **Verify:** file present; Impact names `internal/devid`, config, packaging,
      SPIRE 1.14 — not E2.4 / #70 subject / new JSON fields by default
- [x] 1.2 `design.md` frames Q0–Q5 as questions + recommendations for dembaca,
      with SPIRE 1.14.0 source facts for `devid_ca_path`, agent chain PEM, and
      `ca:fingerprint` (hash over DER; AND multi-selector entries)
      **Verify:** Q0 has A/B + intra-cluster limit; Q1–Q5 Ask+Recommendation;
      table cites `tpmdevid` on tag `v1.14.0`; lab single-tier fingerprint
      match recorded
- [x] 1.6 Review follow-up: add Q0 (threat model A vs B), expand SPIRE
      selector facts, rewrite Risks for (A)/(B)
      **Verify:** coordinator Review on ASSIGN addressed; RESULT posted
- [x] 1.3 Delta `specs/devid-enrollment/spec.md`: ADDED intermediate issuance;
      MODIFIED gate to load intermediate as signing CA
      **Verify:** scenarios cover leaf+intermediate PEM, chain to offline root,
      and enroll gated on intermediate material
- [x] 1.4 Delta `specs/instance-metadata/spec.md`: `devid_ca_readable` still
      means the **signing** CA material is readable (intermediate after cut-over)
      **Verify:** scenario keeps field semantics; does not require root readable
- [x] 1.5 Delta `specs/debian-packaging/spec.md`: packaged comments/unit
      credential docs name intermediate key + root-for-SPIRE (not root key on node)
      **Verify:** ADDED requirement; no change to postinst start/reload behaviour

## 2. Validate and hand off (record)

- [x] 2.1 `openspec validate --all --strict` clean for this change
      **Verify:** CLI: Totals 6 passed, 0 failed (2026-10-19)
- [x] 2.2 Draft PR from `cursor/issue-ldevids-from-an-intermediate-per-cluster-1e7e`
      (base `main` @ `da0083a`); edits only under this change directory
      **Verify:** PR #72; diff only under that directory (`d65bb08`)
- [x] 2.3 Bus RESULT on the ASSIGN thread with PR link and that Q1–Q5 are
      ready for dembaca (coordinator asks singly)
      **Verify:** RESULT on ASSIGN root `pw96c7k1tpb8mpxjc56gboiqoc`

## 3. After dembaca answers (record)

- [x] 3.1 Fold dembaca's answers into `design.md` (replace recommendations
      with decisions where they differ); retick only the deltas that change
      **Verify (2026-10-20):** Q0=B, Q1=A, Q3=1y/`pathLen=0`/`keyCertSign`/
      no EKU/NC recorded as Decisions; Q2/Q4/Q5 marked confirmed
      recommendations; Risks for B/D8 only; `openspec validate --all --strict`
- [x] 3.2 Code-go ACK from interim coordinator (conditions: base #71@`92e4484`,
      keep `devid_ca.key` name, lab Q0(B) proof, health test, Fable before ready)
      **Verify:** ACK on `pw96c7k1…` / Cursor ACK `h667n8qh…`

## 4. Implementation (Code-Go)

- [x] 4.1 `Finish` returns leaf then intermediate PEM (no root); unit test
      decodes two certs, verifies leaf CN and chain to offline root with/without
      intermediate block
      **Verify:** `go test ./internal/devid/ -run 'TestFinishLeafThenIntermediate|TestIssueDevIDAndSession'`
- [x] 4.2 `/health` `devid_ca_readable` true with intermediate cert+key and no
      root private key on the node (one dedicated test)
      **Verify:** `go test ./internal/server/ -run TestHealth.*Intermediate`
- [x] 4.3 Packaged comments: `debian/config.yaml`, unit `LoadCredential=` note,
      `README.Debian` describe cluster intermediate + SPIRE needs root cert;
      credential **name** stays `devid_ca.key`
      **Verify:** grep/comments; `go test ./internal/config/ -run TestLoadShippedDebianConfig` if present
- [x] 4.4 `scripts/gen-devid-intermediate.sh` mints a 1y `pathLen=0`
      `keyCertSign`-only intermediate from an offline root; docs note Ansible
      owns distribution
      **Verify:** script `-h` / dry run against a temp root; openssl inspect
- [x] 4.5 `scripts/qemu-lab/gen-lab-pki.sh` two-tier mode: root for SPIRE
      `devid_ca_path`, intermediate as host `devid_ca_*`
      **Verify:** mode produces distinct root vs intermediate; fingerprints differ

## 5. Lab / hogan verification (Code-Go)

- [x] 5.1 Cloud lab Q0(B): two-tier PKI; SPIRE 1.14 `devid_ca_path` = root;
      agent PEM = leaf+intermediate; attest; selectors include both
      `ca:fingerprint` values; node alias with `subject:cn` **and** intermediate
      fingerprint matches; a leaf from a *second* intermediate does **not**
      **Verify:** `spire-server agent show` / entry create evidence in RESULT
- [ ] 5.2 Hogan: VM 399 enroll/renew into a **scratch** directory only; do not
      overwrite live `/var/lib/spire/agent/` materials. SPIRE live materials
      unchanged. (Coordinator / hogan agent after merge readiness.)
      **Verify:** openssl + sha1sum; journal as appropriate

## 6. OpenSpec / ship

- [x] 6.1 `openspec validate --all --strict` green; `go vet ./...` and
      `go test ./...` green; push draft PR; bus RESULT on #72 thread
- [ ] 6.2 Fable pre-review (read-only) before marking ready-for-review

## 7. Not verified in this change (do not tick as done)

- [ ] 7.1 Ansible role that mints/distributes intermediates (Q1) — out of repo
- [ ] 7.2 Hogan live SPIRE agent paths on 399 (only scratch in 5.2)
- [ ] 7.3 Name Constraints / CRL / HSM (non-goals)
- [ ] 7.4 Rebase onto `main` after #71 merges

## Verification notes (Code-Go 2026-10-20)

- **4.1** `go test ./internal/devid/ -run TestFinishLeafThenIntermediate` PASS; full `./internal/devid/` PASS.
- **4.2** `go test ./internal/server/ -run TestHealthAttestationOnIntermediateWithoutRootKey` PASS.
- **4.3** `debian/config.yaml`, unit `LoadCredential=` comment, `README.Debian` name intermediate; credential name still `devid_ca.key`.
- **4.4** `scripts/gen-devid-intermediate.sh`: 365d, pathLen=0, keyCertSign only (openssl inspect).
- **4.5** `MDS_LAB_DEVID_TIER=two`: host=intermediate fp `7ff474c2…`, SPIRE=root fp `a78be7ec…`.
- **5.1** Lab Q0(B): e2e guest enroll PASS (TCG); PEM 2 blocks; agent `…/tpm_devid/b90bb8f2…` selectors =
  `subject:cn:100`, `ca:fingerprint:7ff474c2…` (inter), `ca:fingerprint:a78be7ec…` (root).
  Node alias CN+inter → SVID `workload/ok`; alias CN+cluster2 inter → no `workload/nope`.
- **6.1** `openspec validate --all --strict` 9/9; `go vet ./...` + `go test ./...` green.
- **6.2** Fable pre-review still open before ready-for-review.
- **5.2** Hogan scratch renew deferred (coordinator).
