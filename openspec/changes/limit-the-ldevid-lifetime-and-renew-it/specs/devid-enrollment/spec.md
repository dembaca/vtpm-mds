## REMOVED Requirements

### Requirement: Issue A Non-Expiring LDevID Bound To The Authenticated VM

**Reason**: E2.3 (2026-10-19) replaces far-future `notAfter`
(`9999-12-31T23:59:59Z`) with a configurable lifetime and guest renewal. The
scenario `Certificate shape` that required that far-future date cannot be
carried forward.

**Migration**: Already-issued non-expiring certificates remain valid until an
operator rotates them (the service still publishes no CRL). After this change
ships, the next successful enroll or guest renew writes a time-limited
certificate. SPIRE registration entries keyed on
`…/tpm_devid/<fingerprint>` must be re-keyed to `tpm_devid:subject:cn:<vmid>`
(or the previous agent evicted) because each new certificate gets a new
fingerprint. Replaced by `Issue A Time-Limited LDevID Bound To The
Authenticated VM`.

## ADDED Requirements

### Requirement: Issue A Time-Limited LDevID Bound To The Authenticated VM

On a successful finish the service SHALL sign a certificate with the DevID CA
containing:

- the DevID public key from the signing request as the certificate public key;
- a subject whose common name is the authenticated VM ID bound to the enroll
  session. A common name carried in the signing request's platform identity
  SHALL be ignored; the guest SHALL NOT be able to influence the common name of
  the certificate it receives. The remaining relative distinguished names of
  the platform identity SHALL be carried into the subject unchanged, because
  they assert no identity the service has established;
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
  subject and the public key, so re-enrolling the same key with the same
  subject yields the same serial.

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

#### Scenario: Other platform identity attributes survive

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries a platform identity with organization
  `Example GmbH` and organizational unit `lab`
- **THEN** the issued certificate's subject carries that organization and
  organizational unit, and its common name is `100`

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

#### Scenario: The TCG SAN is never critical

- **WHEN** a certificate is issued to any authenticated caller
- **THEN** its Subject Alternative Name extension is not marked critical,
  because the subject always carries the VM ID as common name

### Requirement: Configure The LDevID Lifetime

The service SHALL take the LDevID certificate lifetime from
`mds.devid_lifetime`, parsed as a Go duration string. A configuration file that
omits `devid_lifetime` SHALL take the built-in default of ninety days
(`2160h`). A `mds.devid_lifetime` that is present but does not parse as a Go
duration, or that parses to a duration that is not strictly positive, SHALL
cause configuration load to fail with an error naming `devid_lifetime` and the
offending value; the process SHALL exit non-zero and SHALL NOT start serving.

#### Scenario: Default when omitted

- **GIVEN** a configuration file that does not mention `mds.devid_lifetime`
- **WHEN** the configuration is loaded
- **THEN** the LDevID lifetime is ninety days

#### Scenario: Unparseable lifetime is refused

- **GIVEN** a configuration file whose `mds.devid_lifetime` is a value such as
  `90` (no unit) or `0s`
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
SPIRE agent starts, and at least once per day while the guest is up. A
successful renew that replaced the certificate SHALL cause the SPIRE agent to
re-attest with the new materials (for example by restarting the agent unit).

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

#### Scenario: Missing certificate enrolls

- **GIVEN** no certificate at the output path
- **WHEN** the guest renewal check runs on boot
- **THEN** a full enrollment runs and materials are written before the SPIRE
  agent starts

#### Scenario: Boot order

- **GIVEN** the guest package's renewal oneshot and a SPIRE agent unit
- **WHEN** the guest boots
- **THEN** the renewal oneshot finishes before the SPIRE agent starts
