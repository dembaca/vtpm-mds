# Tasks

Record only — no product code until dembaca answers Q1–Q5 and the
coordinator says code-go. Prefer applying after #70 and #71 are archived
(see `proposal.md` Apply order).

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

## 2. Validate and hand off

- [x] 2.1 `openspec validate --all --strict` clean for this change
      **Verify:** CLI: Totals 6 passed, 0 failed (2026-10-19)
- [x] 2.2 Draft PR from `cursor/issue-ldevids-from-an-intermediate-per-cluster-1e7e`
      (base `main` @ `da0083a`); edits only under this change directory
      **Verify:** PR #72; diff only under that directory (`d65bb08`)
- [x] 2.3 Bus RESULT on the ASSIGN thread with PR link and that Q1–Q5 are
      ready for dembaca (coordinator asks singly)
      **Verify:** RESULT on ASSIGN root `pw96c7k1tpb8mpxjc56gboiqoc`

## 3. After dembaca answers

- [x] 3.1 Fold dembaca's answers into `design.md` (replace recommendations
      with decisions where they differ); retick only the deltas that change
      **Verify (2026-10-20):** Q0=B, Q1=A, Q3=1y/`pathLen=0`/`keyCertSign`/
      no EKU/NC recorded as Decisions; Q2/Q4/Q5 marked confirmed
      recommendations; Risks for B/D8 only; `openspec validate --all --strict`
- [ ] 3.2 Code-go on a later ASSIGN — implement task-by-task then; out of
      scope for this record until coordinator ACK
