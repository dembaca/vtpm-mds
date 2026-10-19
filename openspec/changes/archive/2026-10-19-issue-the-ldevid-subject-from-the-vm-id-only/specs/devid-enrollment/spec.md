## MODIFIED Requirements

### Requirement: Verify The Signing Request Before Issuing A Challenge

`enroll/start` SHALL decode the signing request from the exact bytes carried
in `request_b64` and, after authentication, verify in order:

1. `signature_b64` is a valid RSA PKCS#1 v1.5 signature by the request's DevID
   key over those exact bytes, hashed with the algorithm named in that key's
   signature scheme; only RSA keys are supported;
2. the request's endorsement certificate chains to `ek_ca_chain`;
3. the certify data and certify signature prove DevID residency — the
   signature verifies under the request's attestation key, the blob decodes as
   TPM attestation data containing certify info, and the certified name matches
   the DevID public area;
4. the DevID key attributes are sign, not decrypt, not restricted, fixedTPM;
5. the attestation key attributes are sign, restricted, not decrypt, fixedTPM.

Any failure SHALL be answered `400` with the failure text as the body and
logged as `devid enroll/start: <reason>`. Step 2 is reached with an already
trusted certificate whenever the request carries one, because authentication
has bound it to the header; a request that carries no endorsement certificate
SHALL be rejected there with body `missing EK certificate`.

The request's platform identity is parsed as an RDN sequence and is not
verified against the caller's identity. It SHALL contribute nothing to the
issued certificate: the subject is built from the authenticated VM ID alone. A
platform identity that names another VM or carries any other attribute SHALL
therefore be accepted without error and SHALL have no effect on the
certificate.

#### Scenario: Request signature does not verify

- **GIVEN** an authenticated caller whose signing request is well formed
- **WHEN** `signature_b64` is not a valid signature by the DevID key over the
  request bytes
- **THEN** the response is `400` and the body starts with
  `invalid request signature:`

#### Scenario: Signing request carries no endorsement certificate

- **GIVEN** an authenticated caller with a trusted EK certificate header and a
  correctly signed signing request
- **WHEN** the signing request omits the endorsement certificate
- **THEN** the response is `400` with body `missing EK certificate`

#### Scenario: DevID is not resident with the attestation key

- **GIVEN** a signing request whose own signature verifies
- **WHEN** the certify data does not name the DevID public area in the request
- **THEN** the response is `400` and the body starts with `DevID residency:`

#### Scenario: Key attributes violate TCG DevID guidance

- **GIVEN** a signing request whose signature and residency proof verify
- **WHEN** the DevID key is restricted, or the attestation key is not
  restricted, or either key cannot sign or is not fixedTPM
- **THEN** the response is `400` and the body starts with
  `key attribute error:`

## REMOVED Requirements

### Requirement: Issue A Non-Expiring LDevID Bound To The Authenticated VM

**Reason:** it carried the guest's platform identity attributes other than the
common name into a certificate signed by the DevID CA, and its scenario "Other
platform identity attributes survive" asserted that. A `MODIFIED` block cannot
drop a scenario, so the requirement is replaced as a whole.

**Migration:** replaced by "Issue A Non-Expiring LDevID Whose Subject Is Only
The VM ID" below, identical except for the subject, the serial sentence and the
dropped-attribute scenarios. Certificates issued before the upgrade keep their
attributes until they are re-issued.

## ADDED Requirements

### Requirement: Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID

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
  `9999-12-31T23:59:59Z`, so LDevIDs do not expire;
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
  subject and the public key, so re-enrolling the same key for the same VM
  yields the same serial whatever platform identity the request carries.

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

#### Scenario: Platform identity does not change the serial

- **GIVEN** an authenticated caller identified as VM `100` enrolling the same
  DevID key twice
- **WHEN** one signing request carries a platform identity with organization
  `Example GmbH` and the other carries none
- **THEN** both issued certificates have the same serial number

#### Scenario: Certificate shape

- **WHEN** a certificate is issued
- **THEN** it has `notAfter` `9999-12-31T23:59:59Z`, key usage
  digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, and a TCG
  SAN with a HardwareModuleName and a PermanentIdentifier

#### Scenario: The TCG SAN is never critical

- **WHEN** a certificate is issued to any authenticated caller
- **THEN** its Subject Alternative Name extension is not marked critical,
  because the subject always carries the VM ID as common name
