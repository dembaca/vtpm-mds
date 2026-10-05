# Design

## Context

This change implements the direction recorded in `AGENTS.md` under known-gaps
item 6, which the maintainer deferred on 2026-10-03 with the conclusion that
the answer is "remove", not "implement". The analysis is reproduced in
`proposal.md` so this change record stands on its own.

Three facts were established by reading the tree rather than the note, and two
of them are not in the note:

1. The routes are registered in `internal/server/server.go:87–96`, **not** in
   `imds/handlers.go`. `attest` is a top-level package of its own.
2. The `if cfg.MDS.EnableTPMAttestation` block that registers the four
   endpoints **also** registers DevID enrollment (`loadDevIDEnroller`,
   `devid.Register`). Removing the block wholesale would delete the one path
   that SPIRE and Teleport actually use.
3. `/latest/attest/nonce` and `POST /latest/attest` are **not specified
   anywhere** in `openspec/specs/`. Only `/latest/identity` and the JWKS
   endpoint are, by `workload-identity`. This is a pre-existing spec gap, and
   it shapes the delta — see "Why the attest endpoints have no REMOVED
   requirement" below.

## Goals

- Remove four endpoints that report a result they did not compute.
- Leave DevID enrollment byte-for-byte unchanged, including its gating.
- Make the spec tree stop describing placeholder behaviour as a contract.

## Non-Goals

- Implementing TPM quote verification or real signing keys. That is the
  framing this change rejects.
- Deciding a PCR policy. There is no consumer for one, and quote verification
  without a policy consumer is its own purpose, which is no purpose.
- Removing `mds.jwt_ttl`, `mds.jwks_path` or `mds.attestation_ca`.
- Touching `/latest/dynamic/instance-identity/*`.
- Renaming `mds.enable_tpm_attestation`.

The last three are deliberate and each has a decision recorded below.

## Decisions

### Keep `mds.enable_tpm_attestation` under its present name

The flag gates DevID enrollment as well as the removed endpoints, and
`devid-enrollment`'s first requirement specifies that gating by name. After
this change the name is misleading: it will gate enrollment only, and
enrollment is not attestation.

Renaming it is nonetheless **out of scope**. A rename is a breaking
configuration change that needs a deprecation alias, a migration note for
deployed `config.yaml` files, a `debian/vtpm-mds.8` entry and a delta against
`devid-enrollment` — a change of its own, which `AGENTS.md` scope discipline
requires rather than permits. Folding it in here would make this change's
non-goals untrue in the same way the `-version` flags once did.

**Recommendation for a follow-up change:** rename to
`mds.enable_devid_enrollment`, accept `enable_tpm_attestation` as a deprecated
alias for one release, and log a warning when the old key is used.

### Why the attest endpoints have no REMOVED requirement

`/latest/attest/nonce` and `POST /latest/attest` were never written into
`openspec/specs/`. A `REMOVED` requirement names a requirement that exists in
the living spec; there is none to name, so inventing one in order to remove it
in the same change would put text into the spec tree that was never the
contract.

The removal is therefore recorded in `proposal.md` under **What Changes**, as a
`BREAKING` entry, and in `tasks.md` as a verified behavioural step. This is
stated here so a later reader does not mistake the absence of a delta for an
oversight.

### Deletion, not deprecation

No deprecation period, no `410 Gone`, no warning header. The endpoints have no
consumers to warn, and a caller that *is* relying on
`{"valid": true}` from `POST /latest/attest` is relying on an answer that was
never computed — the sooner that breaks, the better. A removed route falls
through to the catch-all and is answered `404 page not found`, which is what an
unregistered route has always returned and what `workload-identity` already
specifies for the case where `enable_tpm_attestation` is false.

### The `workload-identity` spec is removed, not emptied

All three requirements are `REMOVED` and the capability's `spec.md` is deleted
at archive time rather than left as a Purpose with no requirements. A
capability with no requirements asserts nothing and would read as "unfinished"
rather than "gone". The migration note in the delta carries what a reader needs.

## Open questions for the maintainer

Neither is settled here, and the first one changes this change's scope.

### 1. Do `/latest/dynamic/instance-identity/{document,signature}` stay?

**This decides the scope.** `signature` returns the constant
`dGVzdC1zaWduYXR1cmU=` (base64 `test-signature`) to every caller — the same
defect as `POST /latest/attest`: a value that looks like evidence and is not.

The difference is that these two are **genuine AWS paths**. cloud-init and the
AWS SDKs look for them, and `instance-metadata` specifies the document as
*unsigned* and names it as such, so the spec does not currently claim the
signature means anything.

**Recommendation: keep them, and write the signature defect up separately.**
Removing an AWS-compatible path is a compatibility decision about who the
service pretends to be; removing an invented path that nothing consumes is
not. Folding the first into the second would hide a real question inside an
obvious one. If the maintainer decides the constant signature must go, the
cleanest shape is a separate change that either removes `signature` alone and
keeps `document`, or serves `404` for `signature` — `document` is useful
without it, and `instance-metadata` already explains why they are currently
refused together.

**If the maintainer instead wants them removed here**, this change grows a
fourth spec delta against `instance-metadata` and the refused-path list loses
three of its four entries rather than one.

### 2. Does anything outside this repository call the removed endpoints?

Searched: this repository, including `scripts/qemu-lab/` and all tests. Nothing
calls them. Not searched, because it is not reachable from here: any guest
image, Ansible role or operator runbook outside the repo. The maintainer is the
only one who can answer for the deployed estate.

If the answer is "something does", the finding is not that the endpoints should
stay — it is that the caller is trusting an answer that was never computed, and
it needs telling.
