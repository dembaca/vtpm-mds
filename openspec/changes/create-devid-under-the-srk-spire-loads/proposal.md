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

A child blob only loads under the parent it was created under, and a TPM
derives a primary key from its seed and the template. Through **go-tpm**, the
library `devid-enroll` and SPIRE both use, the two templates give two different
primaries on the cloud lab's TPM, so there `devid-enroll` enrolls a DevID that
SPIRE then fails to load with `tpm2.Load … integrity check failed (0x1f)`
(measured, below). Measured through **tpm2-tools**, an empty `unique` and 256
zero bytes give the **same** key, on the lab's TPM and on hogan's alike. The
difference lies in how go-tpm sends a 256-byte zero `unique`, not in which TPM
it is sent to; whether hogan's TPM shows it through go-tpm has not been tried.

This was found by `agent-cursor-vtpm-22e0` on 2026-10-17, while showing the
node-alias route end to end in the cloud lab (`declare-per-vm-metadata`, #33).
On a fresh vTPM the stock client's blobs loaded under `SRKTemplateRSA` and
failed under `SRKTemplateHighRSA`, and blobs made with the high template did
the reverse. The lab needed a lab-only binary with an empty `ModulusRaw` to
finish. **VM 399 on hogan works**: its agent attests with material from
2026-09-14.

**Why VM 399 works: measured, and the first hypothesis was wrong.** This record
first guessed that VM 399's persistent SRK had been made with an empty `unique`
and was reused by `cachedPrimary`. The coordinator tested it on 2026-10-17.
`tpm2_readpublic` shows only the generated modulus, not the template's
`unique`, so it created transient owner primaries in the guest's TPM, with the
attributes of `SRKTemplateRSA` including `noda`, flushed them afterwards, and
compared the moduli:

| Template | Modulus |
|---|---|
| persistent object at `0x81000001` | `e36d99c0…a603e857` |
| `unique` empty (SPIRE's high template) | `e36d99c0…a603e857` |
| `unique` = 256 × `0x00` (this client) | `e36d99c0…a603e857` |
| `unique` = 256 × `0x01` (control) | `cda672bb…665e6827` |
| empty `unique`, without `noda` (control) | `ca4dbae6…37991007` |

On hogan's vTPM (`swtpm 0.8.0+pve3`, `libtpms0 0.9.7+pve2`), **through
tpm2-tools**, an empty `unique` and 256 zero bytes give the same SRK, while a
different `unique` or different attributes give a different one. VM 399's agent
attests with material from 2026-09-14; **how that material was created is not
established**, and a persistent object's own history no longer explains it.

**The lab's two measurements, `agent-cursor-vtpm-22e0`, 2026-10-17** (cloud lab
host: `swtpm 0.7.3-0ubuntu5.24.04.1`, `libtpms0 0.9.3-0ubuntu4.24.04.1`):

A. **Through tpm2-tools**, the coordinator's command on a fresh lab vTPM
   (`tpm2_createprimary -C o -g sha256 -G rsa2048:null:aes128cfb` with the same
   attributes):

   | `unique` | Modulus |
   |---|---|
   | empty | `ccc8d731…d9d30863` |
   | 256 × `0x00` | `ccc8d731…d9d30863` (equal) |
   | 256 × `0x01` | `92a1ea91…bcfbebf5` (different) |

   The lab's `libtpms` does not tell empty and 256 zero bytes apart either.

B. **Through go-tpm**, as the client and SPIRE call it, in the same guest:

   | Template | `ModulusRaw` | Modulus (sha256 / head) |
   |---|---|---|
   | `SRKTemplateRSA` (this client) | 256 bytes | `5c4982f8…` / `b5cbc4c8…` |
   | `SRKTemplateHighRSA` (SPIRE) | empty | `00b194e1…` / `ae5ec096…` |

   The high template's modulus equals the tpm2-tools result for an empty or
   zero `unique` (`ae5ec096…`). **The client's template, through go-tpm,
   produces a different primary**, which is why SPIRE's load fails.

So the defect is reproduced at the library level on one TPM, between the very
two templates the two programs use. What is not known is why go-tpm's
256-byte zero `unique` differs from tpm2-tools' 256 zero bytes, and whether
hogan's TPM behaves like the lab's when driven through go-tpm. The fix, an
empty `unique` as SPIRE uses, is correct on both counts.

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
