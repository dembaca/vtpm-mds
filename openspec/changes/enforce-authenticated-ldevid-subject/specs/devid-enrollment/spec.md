# Spec Delta

## ADDED Requirements

### Requirement: Issue A Non-Expiring LDevID Bound To The Authenticated VM

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

#### Scenario: Certificate shape

- **WHEN** a certificate is issued
- **THEN** it has `notAfter` `9999-12-31T23:59:59Z`, key usage
  digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, and a TCG
  SAN with a HardwareModuleName and a PermanentIdentifier

#### Scenario: The TCG SAN is never critical

- **WHEN** a certificate is issued to any authenticated caller
- **THEN** its Subject Alternative Name extension is not marked critical,
  because the subject always carries the VM ID as common name

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
verified against the caller's identity. It contributes only the subject
attributes other than the common name, which is taken from the authenticated
VM ID. A platform identity that names another VM SHALL therefore be accepted
without error and SHALL have no effect on the identity the certificate asserts.

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

### Requirement: Issue A Non-Expiring LDevID With TCG DevID Encoding

**Reason**: Its subject rule — `a platform identity common name supplied by the
guest therefore wins over the authenticated VM ID` — and its scenario
`Guest-supplied common name is used` state the defect as the contract. The
replacement inverts that outcome, so the scenario cannot be carried forward.
Replaced by `Issue A Non-Expiring LDevID Bound To The Authenticated VM`, which
keeps the public key, validity, key usage, extended key usage, SAN contents,
serial derivation and PEM encoding rules verbatim.

**Migration**: A guest that never set a platform identity common name is
unaffected; its certificate already read `CN=<vm id>`. A guest that did set one
receives its VM ID on the next enrollment, so any authorisation keyed on the
old name — a SPIRE `tpm_devid` registration entry, for example — must be
re-keyed to the VM ID before that guest re-enrols. Certificates already issued
under a guest-chosen name stay valid and cannot be withdrawn, because the
service publishes no revocation list; withdrawing them means rotating the DevID
CA.
