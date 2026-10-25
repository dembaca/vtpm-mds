## MODIFIED Requirements

### Requirement: Guest Renews The LDevID Before SPIRE And When Due

The guest enrollment client SHALL renew the LDevID materials at its configured
output path when any of the following holds:

- no certificate file is present there;
- the certificate's `notAfter` is not after the current time;
- the time remaining until `notAfter` is strictly less than one third of the
  interval from that certificate's `notBefore` to its `notAfter`;
- the DevID private or public TPM blob is missing or empty;
- the certificate's public key is not the public key in the DevID public
  blob;
- the DevID blobs cannot be loaded under the storage root key derived from
  the guest's TPM.

Otherwise the client SHALL leave the materials unchanged and exit successfully
without calling enroll. When checking whether the blobs load fails for any
other reason than the blobs themselves, the client SHALL exit non-zero and
leave the materials unchanged.

When it renews and the DevID blobs at the output path load under that storage
root key, the client SHALL reuse that DevID key in the signing request and
SHALL NOT create a new DevID key under the storage root. When those blobs are
missing or cannot be loaded, the client SHALL create a new DevID key as in a
first enrollment. Each renewal SHALL log which of the conditions above caused
it.

The guest package SHALL arrange that this check runs on every boot before the
SPIRE agent starts when that agent is installed, and at least once per day
while the guest is up. It SHALL NOT require the SPIRE agent unit to be
installed. When a renew replaces the certificate PEM, the guest SHALL clear
the SPIRE agent's cached SVID under the agent data directory and then queue a
`try-restart` of the SPIRE agent unit without waiting for it, so that the next
start performs node attestation with the new LDevID and a renewal oneshot
ordered before that unit can finish first; if the unit is absent or inactive, clearing and
`try-restart` SHALL NOT fail the renew.

#### Scenario: Fresh certificate is left alone

- **GIVEN** a certificate at the output path whose remaining lifetime is at
  least one third of its total validity, and DevID blobs that load in the
  guest's TPM and hold the certificate's public key
- **WHEN** the guest renewal check runs
- **THEN** no enroll request is made and the files are unchanged

#### Scenario: Last third triggers renew with the same key

- **GIVEN** a certificate and matching DevID blobs at the output path whose
  remaining lifetime is less than one third of its total validity
- **WHEN** the guest renewal check runs
- **THEN** enroll runs using the existing DevID public key and writes a new
  certificate PEM while keeping the same private and public blobs

#### Scenario: Materials from another TPM renew with a new key

- **GIVEN** a fresh certificate and DevID blobs created in another TPM, as in
  a clone of a guest that received a new vTPM
- **WHEN** the guest renewal check runs
- **THEN** it logs that the key does not load in this TPM, creates a new
  DevID key, enrolls, and writes a new certificate and new blobs

#### Scenario: Certificate for another key renews with the loaded key

- **GIVEN** a fresh certificate whose public key differs from the key in
  DevID blobs that load in the guest's TPM
- **WHEN** the guest renewal check runs
- **THEN** it logs that the certificate does not match the key and enrolls
  with the loaded DevID key, keeping its blobs

#### Scenario: Missing blob renews

- **GIVEN** a fresh certificate and no DevID public blob
- **WHEN** the guest renewal check runs
- **THEN** it logs the missing blob, creates a new DevID key and enrolls

#### Scenario: A TPM fault does not discard the key

- **GIVEN** a fresh certificate and matching blobs, and a TPM that fails to
  derive the storage root key
- **WHEN** the guest renewal check runs
- **THEN** it exits non-zero, makes no enroll request and leaves the files
  unchanged

#### Scenario: Renew clears the SPIRE agent SVID cache

- **GIVEN** a successful renew that changed the certificate PEM and a SPIRE
  agent data directory with a cached agent SVID
- **WHEN** the renew finishes
- **THEN** that cached agent SVID is removed and `try-restart` is attempted on
  the SPIRE agent unit

#### Scenario: Renewal oneshot does not wait for the agent restart

- **GIVEN** an active SPIRE agent unit and the packaged renewal oneshot, which
  is ordered before it
- **WHEN** the oneshot renews and the certificate PEM changes
- **THEN** the oneshot finishes successfully without waiting for the restart,
  and the SPIRE agent restarts after it

#### Scenario: Missing certificate enrolls

- **GIVEN** no certificate at the output path
- **WHEN** the guest renewal check runs on boot
- **THEN** a full enrollment runs and materials are written before the SPIRE
  agent starts if that unit is enabled

#### Scenario: Guest without SPIRE still renews

- **GIVEN** a guest with no `spire-agent` unit installed
- **WHEN** the renewal oneshot runs and a renew is due
- **THEN** enrollment succeeds and the oneshot exits successfully
