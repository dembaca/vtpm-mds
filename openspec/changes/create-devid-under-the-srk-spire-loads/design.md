# Design

## Context

`proposal.md` states the defect. This document records the evidence with
sources, so nobody has to derive it again, and sets out the decision about an
existing persistent object. Everything below was read on 2026-10-17.

### Evidence: what SPIRE `1.14.0` does

Source: `spiffe/spire` at tag `v1.14.0` (commit `05ce428f0f98`), the agent
plugin under `pkg/agent/plugin/nodeattestor/tpmdevid/tpmutil/`.

- **`session.go`, `loadKey` (around lines 258 to 318).** It decodes the public
  blob and picks the parent template from the key's *type*:
  `SRKTemplateHighRSA()` for an RSA key, `SRKTemplateHighECC()` for an ECC key
  (around lines 271 to 282). It then calls `tpm2.CreatePrimaryEx(rwc,
  tpm2.HandleOwner, tpm2.PCRSelection{}, ownerHierarchyPassword,
  parentKeyPassword, srkTemplate)` (around lines 296 to 300), and `tpm2.Load`
  with that handle as the parent (around line 306). The parent is flushed
  afterwards. **It never reads a persistent handle.**
- **The same parent is used for the attestation key.** `createAttestationKey`
  (around lines 320 to 345) calls `CreatePrimaryEx` with
  `SRKTemplateHighRSA()` too, and the agent creates a new attestation key on
  every attestation.
- **`signingkey.go`, `SRKTemplateHighRSA` (lines 86 to 94).** It takes
  `client.SRKTemplateRSA()` and sets `template.RSAParameters.ModulusRaw =
  []byte{}`. The comment points at the TCG EK Credential Profile and calls it
  the "high range" template, "H-1". `SRKTemplateHighECC` does the same with the
  two ECC point coordinates.
- **`devid.go`.** The agent passes a random password as the parent key
  password and the configured `devid_password` as the key's own. A random
  parent password is harmless: the primary is created with it and used at once.
- **Documentation.** `doc/plugin_agent_nodeattestor_tpm_devid.md` says the
  DevID key pair is generated inside the TPM and loaded from `devid_pub_path`
  and `devid_priv_path`. It does not name the parent template.

### Evidence: what `devid-enroll` does

Source: this repository at `main` `b77b259`, `internal/devid/keys.go`.

- `DefaultKeyFactory` sets `SRKTemplate: client.SRKTemplateRSA()`.
  `go-tpm-tools` v0.4.4, `client/template.go`, `defaultRSAParams`: `ModulusRaw:
  make([]byte, 256) // public.unique must be all zeros`.
- `CreateSRK` calls `cachedPrimary(rw, f.SRKTemplate, tpm2.HandleOwner,
  tpm2.HandleOwner, client.SRKReservedHandle)`, and `SRKReservedHandle` is
  `0x81000001` (`client/handles.go:16`).
- `cachedPrimary` reads the public area at that handle. If it **matches the
  template**, the object is reused. If it does **not**, it is **evicted** with
  `tpm2.EvictControl(rw, "", owner, cachedHandle, cachedHandle)` and a new one
  is created and made persistent there. Nothing is asked and nothing is logged.
- `createUnderSRK` then creates the attestation key and the DevID key under
  that handle (`CreateAK`, `CreateDevID`).
- **`MatchesTemplate`** (`go-tpm` v0.9.8, `legacy/tpm2/structures.go:151`)
  compares `Type`, `NameAlg`, `Attributes` and `AuthPolicy`, and for RSA
  `Symmetric`, `Sign`, `KeyBits` and `ExponentRaw`. **`ModulusRaw` is not
  compared.** An existing SRK whose `unique` differs from ours therefore counts
  as a match and is reused.
- Nothing on the server side depends on the parent key: no code or spec outside
  `keys.go` mentions the SRK, except the one phrase "under the SRK" in
  `devid-enrollment`.

### Evidence: observed

From the cloud lab, `agent-cursor-vtpm-22e0`, SPIRE `1.14.0`, 2026-10-17, as
reported on the agent channel: with a fresh vTPM and the stock client, loading
the blobs under `SRKTemplateRSA` works and under `SRKTemplateHighRSA` fails
with `tpm2.Load` → `integrity check failed (0x1f)`; blobs made under the high
template do the reverse. From hogan, `agent-vtpm-hogan-ead994124`: `keys.go`
is unchanged since `0c2623c` (2026-09-09), and VM 399 has persistent objects
at `0x81000001`, `0x81010001` and `0x81010016`.

**Measured on 2026-10-17, tables in `proposal.md`.** Through **go-tpm**, on both
the lab's TPM (`swtpm 0.7.3`, `libtpms0 0.9.3`) and hogan's (`swtpm 0.8.0+pve3`,
`libtpms0 0.9.7+pve2`), the client's `SRKTemplateRSA` gives a different primary
from SPIRE's H-1, so the defect is general. Through tpm2-tools an empty and a
256-zero `unique` give one key on both TPMs, which makes tpm2-tools unsuited to
compare these two templates. On hogan the persistent object at `0x81000001` has
the same modulus as the H-1 primary. VM 399's pilot material most likely came
from a tpm2-tools script of 2026-09-14, not from this client, so VM 399 neither
shows nor refutes the defect. **Not understood:** why go-tpm's 256-byte zero
`unique` differs from tpm2-tools'. Nobody has read go-tpm's or the TPM's code for
it, and the TPM specification was not re-read.

## Goals

- A DevID made by `devid-enroll` loads in SPIRE's `tpm_devid` attestor on any
  TPM, with or without a persistent object at `0x81000001`.
- The client stops deleting persistent objects it did not create.

## Non-Goals

- **ECC DevID keys.** `DefaultDevIDTemplateRSA` is the only DevID template.
  SPIRE would use `SRKTemplateHighECC` for an ECC key; that is a separate
  question if ECC is ever wanted.
- **The EK.** `CreateEK` uses the same `cachedPrimary`, including the eviction
  of a non-matching persistent object at `0x81010001`. That behaviour is the
  same kind of problem and is left alone here; it is noted for a separate
  change if the maintainer wants it.
- **Changing SPIRE.** The template is SPIRE's and is fixed in `1.14.0` and
  `1.15.4`.
- **Re-enrolling VM 399.** It must not be re-enrolled until this is understood:
  its agent attests with material that works, and a new certificate would
  change its SPIFFE ID.
- Any change to what the certificate says; that is `declare-per-vm-metadata`.

## The decision: what to do with a persistent object that is there

SPIRE does not use a persistent parent, so a persistent SRK is irrelevant to
loading *unless* it is the one the client enrolled under. The question is what
the client does about an object at `0x81000001`.

1. **Leave it alone; derive a transient H-1 primary and flush it
   (recommended).** The client never reads or touches persistent handles for
   the SRK. The H-1 primary is created from the owner seed, so it is the same
   key as a persistent H-1 SRK would be, if one exists. This is exactly what
   SPIRE does, costs one `CreatePrimary` per run, and cannot delete anything.
2. **Reuse it if it is the H-1 primary, otherwise stop.** Compare the object's
   public area, `unique` included, with the H-1 template and use the persistent
   handle when it is equal; fail with a clear message when it is not. It saves
   a `CreatePrimary` and keeps the persistent handle in use, but it needs a
   comparison the library does not provide, and a TPM with an unrelated SRK
   there cannot enroll at all.
3. **Today's behaviour: reuse when it matches, evict and replace otherwise.**
   **Rejected:** it deletes a key that may belong to other software, and its
   match test ignores `unique`, so it accepts exactly the objects that make the
   load fail.

Option 1 also removes a state the code has to reason about: whether the
reserved handle holds something, and what.

## What it costs

- **A DevID made by an earlier client, on a TPM where the client created the
  SRK itself, is dead for SPIRE.** It has to be enrolled again. The certificate
  fingerprint then changes, so the agent's SPIFFE ID changes with it and any
  registration that selects on the old one stops matching (see the SPIRE
  section of `declare-per-vm-metadata`).
- **On VM 399,** material that SPIRE already loads is not touched. A
  re-enrollment with the fixed client gives a key under SPIRE's primary, which
  is the persistent SRK there.
- **One more `CreatePrimary` per enrollment.** Enrollment is rare.

## Risks

- **The cause of the library-level difference is not understood.** The
  defect is shown on two TPM implementations. A physical TPM has not been
  tried. The fix does not depend on the answer, because it moves the client to
  the template SPIRE uses.
- **How VM 399's pilot material was made is not established.** The indications
  point at a tpm2-tools script, not at `devid-enroll`.
- **Another SPIRE version may use another template.** This record is written
  against `1.14.0`, the deployed version. `SRKTemplateHighRSA`,
  `SRKTemplateHighECC` and their use in `loadKey` and `createAttestationKey`
  are the same at tags `v1.14.0` and `v1.15.4` and on `main` (compared on
  2026-10-17), so the template has not moved, but a later release could.
