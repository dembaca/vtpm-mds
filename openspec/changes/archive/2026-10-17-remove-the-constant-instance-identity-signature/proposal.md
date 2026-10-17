# Proposal

## Why

`GET /latest/dynamic/instance-identity/signature` answers every caller with the
same constant, `dGVzdC1zaWduYXR1cmU=` — base64 for `test-signature`. No key is
involved. `instance-metadata` already records this honestly as a placeholder
that "SHALL NOT be treated as evidence of the document's authenticity", and the
coordinator confirmed the constant against the installed package
`0.2.0+git20261005.5a60c877cd9c` on 2026-10-16. The handler still carries a
doc comment calling it "a PKCS#7 signature" and a `TODO: Implement actual
PKCS#7 signing with CA`, and `README.md` lists it as a "Detached signature over
the document".

This is the defect `remove-the-unverifiable-identity-endpoints` removed from
`/latest/attest` and `/latest/identity`: an endpoint that has the shape of a
check and passes everyone. That change deliberately left
`/latest/dynamic/instance-identity/*` alone and recorded the constant as a
separate defect, because the two paths are genuine AWS paths and there might be
a compatibility reason to keep a signature. The maintainer decided on
2026-10-05 that the **document stays**, and commissioned research into whether
a *real* signature would be worth implementing before deciding the signature's
fate.

The research is in; it was done by `agent-cursor-vtpm-0c1a` against current
documentation and posted to the `vtpm-mds` agent channel on 2026-10-05
(`RESULT Recherche IID-Signatur`). It is cited here, not re-derived. A real
signature would be worth something only if a consumer that verifies one can be
pointed at a trust anchor this service holds. None can:

| Consumer | Verifies the signature? | Can its trust anchor be replaced? |
|---|---|---|
| Vault `auth/aws`, `ec2` type | yes | **yes**, `POST auth/aws/config/certificate/<name>` — but every `ec2` login also calls `ec2:DescribeInstances` and requires `State=running` (`validateInstance` in `builtin/credential/aws/path_login.go`), which no Proxmox VM can pass, and which cannot be switched off |
| SPIRE `aws_iid` | yes | **no** — the AWS certificates are compiled in; an operator override was proposed in spiffe/spire#4124 and deliberately not accepted |
| Teleport `ec2` join | yes | **no** — region-specific AWS certificates, and a `DescribeInstances` check as well |
| cloud-init | **no**, reads the data only | n/a |
| AWS SDKs | **no**, return the document; verifying is the relying party's job | n/a |
| Consul, Nomad | do not use the instance identity document | n/a |

So a signature from this service has, today, no consumer that would both
verify it and accept the key it was made with. The two integrations this
project exists for need none of it: SPIRE uses `tpm_devid` against the DevID
files `devid-enrollment` produces, and Teleport's TPM join talks to the node's
own TPM and to the Teleport Auth Service, not to a metadata service.

The research therefore concludes, and the maintainer agreed on 2026-10-16, that
the honest move is to **remove** the signature rather than implement it.
Keeping the constant is not neutral: a listed `signature` beside a `document`
invites the reading that the document is signed.

Removing it breaks nothing that can be found. No client, lab script or guest
component in this repository reads it. cloud-init reaches leaves under
`dynamic/instance-identity` only through the directory listing
`GET /latest/dynamic/instance-identity/`, which this service has never served,
so it has never fetched `signature` from here either — see `design.md`.

## What Changes

- `GET /latest/dynamic/instance-identity/signature` is **no longer registered**.
  It is answered by the catch-all `404` with the body `404 page not found`,
  exactly as `/pkcs7` and `/rsa2048` — the other two AWS signature paths, which
  this service never served — already are.
- `GET /latest/dynamic/instance-identity/document` is **unchanged**, byte for
  byte, including its refusal to unbound callers and its `422` for a caller with
  no derivable instance id.
- The path leaves `imds.IdentityBearingPaths`, so the set refused to unbound
  callers shrinks from three paths to two. Nothing becomes reachable that was
  refused: the path stops existing for every caller.
- The documentation that describes the signature as a signature stops doing so:
  `README.md`, `ARCHITECTURE.md`, `debian/vtpm-mds.8`, and the comment above
  `require_vm_identity` in `config.yaml`.

**Not changing:** the document, `mds.signing_key_path` (read by nothing before
and after this change; see `design.md`), and any other metadata path.

## Capabilities

### Modified Capabilities

- `instance-metadata` — two requirements:
  - `Serve An Unsigned Instance Identity Document` is **replaced** by
    `Serve The Instance Identity Document Without A Signature` (REMOVED +
    ADDED). Every clause and scenario about the document is carried verbatim;
    the signature clause and its `Identity signature is a constant placeholder`
    scenario give way to a clause and two scenarios stating that no signature
    path is served.
  - `Refuse An Instance Identity To Callers With No VM Record` is **modified**:
    it enumerates two paths instead of three, drops the paragraph explaining
    why a constant was refused alongside the document, and its scenario
    `Unbound caller is refused both instance identity endpoints` keeps its name
    but now says why each of the two answers `404`.

## Impact

- `imds/handlers.go` — `HandleInstanceIdentitySignature` is deleted, and the
  signature path leaves `IdentityBearingPaths`; the doc comment above it is
  updated to say why.
- `internal/server/server.go` — one route registration.
- `imds/handlers_test.go` — the signature row in the refused-path table,
  `TestIdentityBearingPathsIsExactlyTheSpecifiedSet`, the signature route in
  the test mux, and `TestSignatureServedToPeerWithoutIPv4` are replaced by a
  test that the path answers the catch-all `404`.
- `README.md` — one row of the API endpoint table. **This file belongs to the
  coordinator** (`AGENTS.md`, "One hand on `AGENTS.md` and the root
  `README.md`"); the implementer hands the edit over rather than making it.
- `ARCHITECTURE.md` — one row of the key-endpoints table.
- `debian/vtpm-mds.8` — the refused-path list in the `require_vm_identity`
  entry.
- `config.yaml` — the comment above `require_vm_identity`, which says "both
  /latest/dynamic/instance-identity/ endpoints". `debian/config.yaml` does not
  carry that comment and is unaffected.
- `AGENTS.md` — the known-gaps entry for the constant signature, at archive
  time, by the coordinator.
- **Overlap with `serve-a-public-address-or-omit-the-key`**, in flight at the
  same time: both change `imds/handlers.go`, and both carry an
  `instance-metadata` delta. The deltas touch different requirements, but this
  one carries the text of `Refuse An Instance Identity To Callers With No VM
  Record` in full, including a sentence that change makes untrue. See
  `design.md` and the ordering note in `tasks.md`.
- Operators and guests: a caller that reads `/signature` now gets `404` instead
  of a constant. None is known; `design.md` records how that was checked.
