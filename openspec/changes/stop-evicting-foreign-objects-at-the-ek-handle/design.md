# Design

## Context

`proposal.md` states the defect. This records the measurements and the options.
Written 2026-10-17 against `main` at `4b5ed44`.

### Evidence: the code

- `keys.go`, `CreateEK` calls `cachedPrimary(rw, f.EKTemplate, tpm2.HandleOwner,
  tpm2.HandleEndorsement, client.EKReservedHandle)`; `EKReservedHandle` is
  `0x81010001`. The template is `client.DefaultEKTemplateRSA()`.
- `cachedPrimary` reuses an object whose public area `MatchesTemplate`s, evicts
  one that does not, then creates the primary in the endorsement hierarchy and
  persists it with `EvictControl`, authorised by the owner hierarchy.
- `MatchesTemplate` (go-tpm v0.9.8) does not compare `ModulusRaw`, as recorded in
  `create-devid-under-the-srk-spire-loads`.
- `csr.go`: `CreateSigningRequest` reads the EK certificate from NV
  `0x01c00002`, calls `CreateEK`, then the AK and DevID, and returns
  `RequestResources`. `Activate` later calls `ActivateCredential` with the AK and
  EK handles, which opens a `PolicySecret` session on the endorsement hierarchy.
  `Flush` releases the AK and the DevID key and deliberately leaves the EK, "typically
  persistent".
- `auth.go` and `verify.go`: the service verifies that the request's EK public
  key equals the key in the EK certificate (`VerifyEKCertificateBound`). That is
  the check a wrong EK fails.

### Evidence: measured on 2026-10-17

go-tpm-tools' in-process simulator (Microsoft reference implementation), the
`tpmsim` build tag of `srk_sim_test.go`, a throwaway probe not committed. It is a
third TPM implementation beside swtpm/libtpms on hogan and in the lab, so it shows
the logic, not those.

| Case | Result |
|---|---|
| Persistent **storage key** at `0x81010001`, then `CreateEK` | No error. The object at `0x81010001` was **replaced**: the foreign key is gone. |
| Persistent key from the EK template with `unique` 256 x `0x01` at `0x81010001`, then `CreateEK` | No error. The client **used the foreign key**, which is not the EK. |
| Transient EK from the default template vs a persistent one made from it | **Same key.** |
| Transient objects held at once | **3**; the fourth `CreatePrimary` fails with `out of memory for object contexts`. |

**On hogan** (VM 399, `swtpm 0.8.0+pve3`, `libtpms 0.9.7`, task 1.2, coordinator,
read-only): the EK derived from the default template, the persistent object at
`0x81010001` and the public key in the EK certificate at NV `0x01c00002` are the
same key (`952f9c28...f23a23fb`). Unlike the SRK, the EK has a fixed reference,
the TCG EK template and the certificate, so a derived EK is right by construction
and a foreign persistent object cannot change it; only an option that *uses* the
persistent object could break the binding. That is a further argument for option 1.

**Not measured:** the same on the cloud lab's `swtpm-localca` with another
`libtpms` (task 3.1), and the three-slot flow on a real TPM.

## Goals

- The client never deletes or replaces a persistent object it did not create.
- The EK in the signing request is the key the EK certificate certifies, by
  construction, not by what happens to sit at a handle.

## Non-Goals

- **Checking on the client that the derived EK matches the NV certificate.**
  That would fail early with a clear message on a TPM whose EK was not made from
  the default template, instead of the service's `401`. It is a separate
  behaviour; noted for a follow-up if the maintainer wants it.
- **ECC EKs** and any EK template other than the RSA default.
- **The service side.** `VerifyEKCertificateBound` stays as it is.
- **The SRK.** Done in `create-devid-under-the-srk-spire-loads`.

## The decision: what the client does with a persistent object at `0x81010001`

1. **Leave it alone; derive a transient EK each time (recommended).** The client
   never reads or touches the handle. Correct by construction: the transient
   EK is the key a persistent one made from the same template would be, and the
   certificate certifies that key. Costs two `CreatePrimary` calls per enrollment
   (RSA-2048 key generation, a few seconds on some TPMs); enrollment is rare.
2. **Use the persistent object if its public key equals the EK certificate's key,
   otherwise derive.** Saves the derivations on a platform with a provisioned EK,
   and needs a key comparison the library does not provide. The saving is small
   and the comparison is code to get wrong.
3. **Today's behaviour (reuse on template match, evict otherwise).** **Rejected:**
   it deletes keys it does not own, and its match test accepts keys that are not
   the EK.

## Decided

*Pending the maintainer's decision. The delta is written for option 1.*

## Implementation notes

- **Why two derivations.** With a transient EK held for the whole run, creating
  the DevID key needs the EK, the AK, the SRK parent and the new key: four. The
  simulator and libtpms have three object slots; the SRK change measured that
  three fit on both real vTPMs and the simulator. The EK is needed only as a
  public area (signing request) and, much later, for `ActivateCredential`, so it
  is derived and flushed around each use.
- `RequestResources.Endorsement` keeps the EK's public area and template and no
  loaded handle between the two uses; `Activate` derives it, activates, flushes.
- `RequestResources.Flush`'s comment "EK is typically persistent; leave it
  loaded" becomes untrue and goes.

## What it costs

- **Two more `CreatePrimary` calls per enrollment**, RSA-2048 each.
- **Nothing to re-enroll.** The signing request carries the same EK public key as
  before on any TPM where the old client used the right key.
- **A TPM whose EK was not made from the default template** fails exactly as it
  failed before, with the service's binding error. Not fixed here; see Non-Goals.

## Risks

- **The derived EK may not be the EK the platform certifies** on a TPM whose
  vendor used another template. Same as today; unverified on hogan.
- **A real TPM with two object slots** would fail where the simulator and
  libtpms do not. The flow holds at most three, as after the SRK change.
