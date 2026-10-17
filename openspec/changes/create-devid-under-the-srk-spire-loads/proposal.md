# Proposal

## Why

**`devid-enroll` creates DevID material that SPIRE `1.14.0` cannot load on a
TPM that has no storage root key yet.** The living `devid-enrollment` spec says
the client creates the DevID key "under the SRK" and writes
`devid.priv.blob` and `devid.pub.blob` "as SPIRE's `tpm_devid` attestor
consumes them". It does not say *which* SRK, and the two programs pick
different ones.

- **SPIRE** derives its parent **every time it loads**: it calls
  `tpm2.CreatePrimaryEx` in the owner hierarchy with the TCG "H-1" template
  (`SRKTemplateHighRSA`, or `SRKTemplateHighECC` for an ECC DevID) and loads the
  DevID blobs under that fresh primary. That template is go-tpm-tools'
  `SRKTemplateRSA()` with its `unique` field (`ModulusRaw`) set to `[]byte{}`.
- **`devid-enroll`** creates its parent with `client.SRKTemplateRSA()` itself,
  whose `ModulusRaw` is 256 zero bytes, and makes it persistent at `0x81000001`.

A TPM derives a primary key from its seed and the **whole template, including
`unique`**. Two templates that differ only there give two different keys, and a
child blob only loads under the parent it was created under. On a TPM with no
persistent SRK, `devid-enroll` therefore enrolls a DevID that SPIRE then fails
to load with `tpm2.Load … integrity check failed (0x1f)`.

This was found by `agent-cursor-vtpm-22e0` on 2026-10-17, while showing the
node-alias route end to end in the cloud lab (`declare-per-vm-metadata`, #33).
On a fresh vTPM the stock client's blobs loaded under `SRKTemplateRSA` and
failed under `SRKTemplateHighRSA`, and blobs made with the high template did
the reverse. The lab needed a lab-only binary with an empty `ModulusRaw` to
finish. **VM 399 on hogan works**: its agent attests with material from
2026-09-14.

**Why VM 399 works is a hypothesis, not a finding.** `cachedPrimary` in
`internal/devid/keys.go` reuses a persistent object at `0x81000001` when
`MatchesTemplate` says it fits, and go-tpm's `MatchesTemplate` for RSA compares
`Symmetric`, `Sign`, `KeyBits` and `ExponentRaw`, **not** `ModulusRaw`. VM 399
has persistent objects at `0x81000001`, `0x81010001` and `0x81010016`
(coordinator, `tpm2_getcap handles-persistent`, 2026-10-17). If the one at
`0x81000001` was created with an empty `unique`, for example by `tpm2-tools`,
it would match, be reused, and give material that SPIRE loads. That is
consistent with every observation and has not been checked: nobody has read the
public area of that object.

## What Changes

- **The DevID key and the attestation key are created under the primary SPIRE
  derives**: a transient owner-hierarchy primary from the H-1 template, flushed
  when the client is done.
- **The client no longer reads, reuses, replaces or evicts a persistent object
  at `0x81000001`.** Today a persistent object there that does not match the
  template is **evicted and replaced**: `cachedPrimary` calls `EvictControl` on
  it. That deletes a key that may belong to other software on the TPM.
- **Blobs made by earlier clients on a TPM without a matching SRK stay
  unloadable by SPIRE.** They have to be re-created by enrolling again, which
  also changes the certificate fingerprint and with it the agent's SPIFFE ID.

**Not decided here:** what to do with a persistent object that is present;
`design.md` recommends leaving it alone and lists the alternatives. The decision
is the maintainer's. The delta is written for the recommendation.

## Capabilities

### Modified Capabilities

- `devid-enrollment` — `Write DevID Materials On The Guest`, in its first
  paragraph and with a new paragraph on the parent key and two new scenarios.
  The three existing scenarios are carried verbatim.

## Impact

- `internal/devid/keys.go` and `internal/devid/templates.go` — the parent key.
  `CreateSRK` and `cachedPrimary` are used for the SRK today; the EK uses
  `cachedPrimary` too and is **not** changed here.
- `internal/devid/*_test.go` — a test that loads the blobs the way SPIRE does,
  and a test that a persistent object at `0x81000001` survives.
- `scripts/qemu-lab/` is not touched; the lab verifies, it does not change.
- Operators: any DevID made by an earlier client on a TPM without a matching
  SRK is unusable for SPIRE `1.14.0` and has to be enrolled again.
- **Ordering:** independent of the other open work. It touches no file that
  `declare-per-vm-metadata` (#33) touches.
