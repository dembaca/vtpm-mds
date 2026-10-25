# Proposal

## Why

On 2026-10-25 bgl-infra TASK 2.3c cloned VM 399 into a new VM, gave the
clone an empty vTPM and booted it with 399's DevID files and SPIRE agent data
still in place, as a template built without cleanup would. `devid-enroll`
0.3.1 left all of it alone: it decides only from the certificate's lifetime,
and 399's certificate was fresh. The clone thus kept a certificate with
`CN=399` and DevID blobs that its own TPM cannot load, so the SPIRE agent
could not attest with them, and nothing would have renewed them until the
last third of their lifetime.

The same gap exists without a clone: a vTPM replaced on purpose (`qm
fresh-tpm`, a restored backup without `tpmstate`), or DevID files copied
between guests by hand, leave a fresh certificate over a key the TPM does not
hold. A certificate whose public key is not the key in `devid.pub.blob`
(files from two different runs) is broken in the same way, and so is a fresh
certificate with a missing blob.

## What Changes

- Besides the lifetime, the renewal check asks whether the materials still
  fit this TPM. It renews also when either blob is missing or empty, when
  the certificate's public key is not the public key in `devid.pub.blob`,
  or when the blobs cannot be loaded under the storage root key derived from
  this TPM.
- When the blobs cannot be loaded, the renewal creates a new DevID key, as a
  first enrollment does. When they load but the certificate does not match,
  it reuses the loaded key, as a due renewal does.
- A TPM error other than a failed load fails the run and leaves the
  materials unchanged, so a transient fault never discards a working key.
- Because a renewal changes the certificate PEM, the existing behaviour
  clears the SPIRE agent's cached SVID and queues its restart; an agent
  that inherited another guest's SVID therefore attests afresh.

## Impact

- `internal/devid/client.go`, `internal/devid/renew.go` (or a new helper
  beside it), their tests, `debian/devid-enroll.1`. Guest package only;
  nothing on the host side changes.
- Spec: `devid-enrollment` modifies "Guest Renews The LDevID Before SPIRE
  And When Due".
- Every run, including the daily timer's, now derives the storage root key
  and loads the DevID key once even when nothing is due: a few seconds of TPM
  work per day on a vTPM.
- **Apply order:** `make-guest-renewal-work-with-a-non-root-spire-agent`
  modifies the same requirement and is not archived yet. Its delta must be
  archived first; this delta is written against the text that one leaves.
- Not a replacement for template cleanup: bgl-infra's template still empties
  `/var/lib/spire/agent`, the DevID files and `/etc/machine-id`.
- Released as 0.3.2 of the guest package, after the maintainer's go.
