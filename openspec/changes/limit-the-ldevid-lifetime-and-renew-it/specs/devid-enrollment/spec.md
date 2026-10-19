## REMOVED Requirements

### Requirement: Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID

**Reason**: This is the post-#70 (`issue-the-ldevid-subject-from-the-vm-id-only`)
issuance requirement. E2.3 replaces far-future `notAfter` with a configurable
lifetime, changes serial derivation so renewals are unique, and adds guest
renewal. The scenario `Certificate shape` that required
`9999-12-31T23:59:59Z` and the serial sentence that promised the same serial
for the same key cannot be carried forward. The scenario `Platform identity
does not change the serial` is also dropped here: with validity in the serial
hash, two enrollments at different times get different serials even with the
same key and subject; same-instant enrollments with differing platform
identity still match (covered by unit tests). Replaced by the scenario
`Renewal with the same key yields a new serial`.

**Migration**: Apply only after #70 has been archived into the living spec.
Already-issued non-expiring certificates remain valid until re-issued (no
CRL). Replaced by `Issue A Time-Limited LDevID Whose Subject Is Only The VM
ID`. SPIRE entries keyed on `…/tpm_devid/<fingerprint>` must move to
`tpm_devid:subject:cn:<vmid>` (or the old agent must be evicted) because each
new certificate gets a new fingerprint. Operators clearing a renewed guest's
SPIRE agent SVID cache is required for the new LDevID to be attested.

## ADDED Requirements

### Requirement: Issue A Time-Limited LDevID Whose Subject Is Only The VM ID

On a successful finish the service SHALL sign a certificate with the DevID CA
containing:

- the DevID public key from the signing request as the certificate public key;
- a subject consisting of exactly one attribute, the common name, set to the
  authenticated VM ID bound to the enroll session. Nothing from the signing
  request's platform identity SHALL be carried into the subject; the guest
  SHALL NOT be able to influence any attribute of the subject of the
  certificate it receives, because every attribute is signed by the same CA
  and a relying party cannot tell which ones the service checked;
- `notBefore` set to the time of the finish call and `notAfter` set to
  `notBefore` plus the configured LDevID lifetime (`mds.devid_lifetime`;
  default ninety days when the key is omitted);
- key usage digitalSignature, basic constraints present with CA false, and the
  extended key usage OID `2.23.133.11.1.2` (tcg-cap-verifiedTPMFixed);
- a Subject Alternative Name extension carrying a TCG HardwareModuleName with
  hardware type `2.23.133.1.2` and a PermanentIdentifier with assigner
  `2.23.133.12.1`, built from the SHA-256 digest of the request's endorsement
  public key when one is present and from the endorsement certificate DER
  otherwise. Because authentication requires a non-empty VM ID and that VM ID
  is always the common name, the subject is never empty, so this extension
  SHALL NOT be marked critical;
- a serial number derived deterministically from the CA certificate, the
  subject, the public key, and the certificate's `notBefore` and `notAfter`,
  so two issuances that differ in validity receive different serials even when
  the DevID key and subject are unchanged.

The certificate SHALL be returned PEM-encoded in `devid_cert_pem`. The service
SHALL NOT publish a revocation list or offer any other way to withdraw an
issued LDevID.

#### Scenario: Guest-supplied common name is ignored

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries a platform identity with common name
  `other-name`
- **THEN** the issued certificate's subject common name is `100` and the string
  `other-name` does not appear in the subject

#### Scenario: VM ID is the common name

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries no platform identity
- **THEN** the issued certificate's subject is `CN=100`

#### Scenario: Other platform identity attributes are dropped

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries a platform identity with organization
  `Example GmbH`, organizational unit `lab` and common name `other-name`
- **THEN** the enrolment succeeds and the issued certificate's subject is
  exactly `CN=100`, with no organization and no organizational unit

#### Scenario: Certificate shape under the default lifetime

- **GIVEN** the default LDevID lifetime (ninety days)
- **WHEN** a certificate is issued at time `T`
- **THEN** it has `notAfter` equal to `T` plus ninety days, key usage
  digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, and a TCG
  SAN with a HardwareModuleName and a PermanentIdentifier

#### Scenario: Configured lifetime is applied

- **GIVEN** `mds.devid_lifetime` is `48h`
- **WHEN** a certificate is issued at time `T`
- **THEN** its `notAfter` is `T` plus forty-eight hours

#### Scenario: Renewal with the same key yields a new serial

- **GIVEN** an authenticated caller identified as VM `100` that already holds
  an LDevID for a DevID key
- **WHEN** it successfully enrols again with that same DevID key at a later
  `notBefore`
- **THEN** the new certificate's serial number differs from the previous
  certificate's serial

#### Scenario: The TCG SAN is never critical

- **WHEN** a certificate is issued to any authenticated caller
- **THEN** its Subject Alternative Name extension is not marked critical,
  because the subject always carries the VM ID as common name

### Requirement: Configure The LDevID Lifetime

The service SHALL take the LDevID certificate lifetime from
`mds.devid_lifetime`, parsed as a Go duration string. A configuration file that
omits `devid_lifetime` SHALL take the built-in default of ninety days
(`2160h`). A `mds.devid_lifetime` that is present but does not parse as a Go
duration, that parses to a duration that is not strictly positive, or that is
greater than one year (`8760h`), SHALL cause configuration load to fail with
an error naming `devid_lifetime` and the offending value; the process SHALL
exit non-zero and SHALL NOT start serving.

#### Scenario: Default when omitted

- **GIVEN** a configuration file that does not mention `mds.devid_lifetime`
- **WHEN** the configuration is loaded
- **THEN** the LDevID lifetime is ninety days

#### Scenario: Unparseable or oversized lifetime is refused

- **GIVEN** a configuration file whose `mds.devid_lifetime` is a value such as
  `90` (no unit), `0s`, or `9000h`
- **WHEN** the configuration is loaded
- **THEN** it exits non-zero with an error naming `devid_lifetime` and the
  offending value

### Requirement: Guest Renews The LDevID Before SPIRE And When Due

The guest enrollment client SHALL renew the LDevID materials at its configured
output path when any of the following holds:

- no certificate file is present there;
- the certificate's `notAfter` is not after the current time;
- the time remaining until `notAfter` is strictly less than one third of the
  interval from that certificate's `notBefore` to its `notAfter`.

Otherwise the client SHALL leave the materials unchanged and exit successfully
without calling enroll.

When it renews and usable DevID private and public TPM blobs already exist at
the output path, the client SHALL reuse that DevID key in the signing request
and SHALL NOT create a new DevID key under the storage root. When those blobs
are missing or cannot be loaded, the client SHALL create a new DevID key as in
a first enrollment.

The guest package SHALL arrange that this check runs on every boot before the
SPIRE agent starts when that agent is installed, and at least once per day
while the guest is up. It SHALL NOT require the SPIRE agent unit to be
installed. When a renew replaces the certificate PEM, the guest SHALL clear
the SPIRE agent's cached SVID under the agent data directory and then
`try-restart` the SPIRE agent unit so the next start performs node attestation
with the new LDevID; if the unit is absent or inactive, clearing and
`try-restart` SHALL NOT fail the renew.

#### Scenario: Fresh certificate is left alone

- **GIVEN** a certificate at the output path whose remaining lifetime is at
  least one third of its total validity
- **WHEN** the guest renewal check runs
- **THEN** no enroll request is made and the files are unchanged

#### Scenario: Last third triggers renew with the same key

- **GIVEN** a certificate and matching DevID blobs at the output path whose
  remaining lifetime is less than one third of its total validity
- **WHEN** the guest renewal check runs
- **THEN** enroll runs using the existing DevID public key and writes a new
  certificate PEM while keeping the same private and public blobs

#### Scenario: Renew clears the SPIRE agent SVID cache

- **GIVEN** a successful renew that changed the certificate PEM and a SPIRE
  agent data directory with a cached agent SVID
- **WHEN** the renew finishes
- **THEN** that cached agent SVID is removed and `try-restart` is attempted on
  the SPIRE agent unit

#### Scenario: Missing certificate enrolls

- **GIVEN** no certificate at the output path
- **WHEN** the guest renewal check runs on boot
- **THEN** a full enrollment runs and materials are written before the SPIRE
  agent starts if that unit is enabled

#### Scenario: Guest without SPIRE still renews

- **GIVEN** a guest with no `spire-agent` unit installed
- **WHEN** the renewal oneshot runs and a renew is due
- **THEN** enrollment succeeds and the oneshot exits successfully
