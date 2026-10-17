# Proposal

## Why

**`devid-enroll` deletes a persistent object at `0x81010001` that it did not
create, and can enroll with a key that is not the one the EK certificate
certifies.** The endorsement key goes through `cachedPrimary`, the helper the
storage root key used until `create-devid-under-the-srk-spire-loads`. It reads
the public area at `0x81010001`. If the object **matches the EK template** it is
reused; if it does **not**, the client calls `EvictControl` on it and persists
its own key there. Nothing is asked and nothing is logged.

Two failures follow, both measured on the TPM simulator (`design.md`):

- **A foreign object is deleted.** A persistent object at `0x81010001` whose
  attributes differ from the EK template, a storage key made by other software,
  is evicted and replaced. `0x81010001` is the TCG reserved handle for the RSA
  EK, so on a real TPM the object there is most likely the platform's own EK.
- **A wrong key is reused.** `MatchesTemplate` ignores the template's `unique`
  field. An object with the EK template's attributes and another `unique` counts
  as a match, and the client uses it. Its public key is not the one in the EK
  certificate at NV `0x01c00002`, so the service refuses the enrollment with an
  EK-binding error that points at the certificate and not at the cause.

The EK is a deterministic primary: the same endorsement seed and template give
the same key every time, persistent or transient. Deriving it each time costs a
`CreatePrimary` and makes the persistent handle irrelevant, as it was for the
SRK.

## What Changes

- **The EK is a transient primary key**, derived from the default RSA EK
  template in the endorsement hierarchy, never made persistent.
- **The client no longer reads, reuses, replaces or evicts a persistent object
  at `0x81010001`**, or anywhere else, on behalf of the EK.
- **The EK is loaded twice, briefly:** once to read its public area for the
  signing request, once to solve the credential challenge, and flushed each time.
  Holding it for the whole run together with the attestation key, the DevID key
  and the storage root key would need four object slots; libtpms and the
  simulator have three.
- `cachedPrimary` has no caller left and is removed.

## Capabilities

### Modified Capabilities

- `devid-enrollment` — `Write DevID Materials On The Guest`: the EK is derived,
  not cached, with a new paragraph and two new scenarios. The existing
  scenarios are carried verbatim.

## Impact

- `internal/devid/keys.go`, `csr.go` (`RequestResources`, `Activate`,
  `Flush`) and the tests in `internal/devid/`.
- `scripts/qemu-lab/` is not touched; the lab verifies, it does not change.
- Operators: nothing to re-enroll. The EK is not part of the issued material;
  the certificate's EK fingerprint is that of the NV certificate, unchanged.
- **Ordering:** independent of the other open work. `declare-per-vm-metadata`
  (#33) touches no file this change touches.
