# DevID Enrollment

## Purpose

Issue IEEE 802.1AR / TCG LDevID certificates to QEMU guests over the metadata
service, so a guest can obtain SPIRE `tpm_devid` materials anchored in its
vTPM. Enrollment is a two-leg HTTP protocol — a signed TPM signing request
answered with a credential-activation challenge, then a challenge response
answered with a certificate — authenticated by the caller's inventory identity
together with an endorsement key certificate that chains to a configured EK CA.
This capability covers the wire protocol, its authentication and the guest
client; how a caller's MAC maps to a VM belongs to the `vm-inventory`
capability, and the IMDSv2 token flow belongs to `instance-metadata`.

## Requirements

### Requirement: Gate Enroll Routes On Attestation And DevID CA Material

The service SHALL mount `POST /latest/devid/enroll/start` and
`POST /latest/devid/enroll/finish` only when `enable_tpm_attestation` is true
and a DevID CA can be loaded. Loading SHALL require both `devid_ca_cert` and
`devid_ca_key` to be non-empty, both paths to exist, and the PEM material to
parse into a certificate plus a PKCS#8 or PKCS#1 private key.

When either path is empty the service SHALL log
`Warning: devid_ca_cert/devid_ca_key not set; skipping DevID routes` and
continue without the routes. When a path is set but unusable the service SHALL
log `Warning: DevID enrollment disabled: <reason>` and continue without the
routes. In both cases the enroll paths SHALL fall through to the catch-all
handler and return `404`.

`ek_ca_chain` SHALL supply the EK trust roots, and a path that cannot be read
or that holds no parseable certificate SHALL disable the routes the same way.
An empty `ek_ca_chain` SHALL NOT prevent route registration: the service logs
`Warning: ek_ca_chain empty; DevID EK verification will fail` and starts with
an empty root pool, so every enroll request is rejected with `401` at
authentication time.

#### Scenario: Enroll routes registered

- **GIVEN** `enable_tpm_attestation: true` and readable `devid_ca_cert` and
  `devid_ca_key` files
- **WHEN** the daemon starts
- **THEN** it logs `DevID enrollment routes registered` and both enroll paths
  accept `POST`

#### Scenario: No DevID CA configured

- **GIVEN** `devid_ca_cert` or `devid_ca_key` is empty
- **WHEN** a guest posts to `/latest/devid/enroll/start`
- **THEN** the response is `404` and the daemon logged that it skipped the
  DevID routes

#### Scenario: DevID CA file missing

- **GIVEN** `devid_ca_cert` names a path that does not exist
- **WHEN** the daemon starts
- **THEN** it logs `Warning: DevID enrollment disabled:` with the failing path
  and the enroll paths return `404`

#### Scenario: TPM attestation disabled

- **GIVEN** `enable_tpm_attestation: false` and valid DevID CA material
- **WHEN** a guest posts to `/latest/devid/enroll/finish`
- **THEN** the response is `404`

#### Scenario: EK trust roots missing

- **GIVEN** `ek_ca_chain` is empty and DevID CA material is valid
- **WHEN** a guest posts a well-formed enroll request
- **THEN** the routes exist and the response is `401` because the EK
  certificate cannot be chained to any root

### Requirement: Serve Enrollment As Two POST JSON Legs

Both legs SHALL accept `POST` with a JSON body, read at most 1 MiB of that
body, and answer with `Content-Type: application/json` on success.

`enroll/start` SHALL read `request_b64` and `signature_b64`, each standard
base64, and on success return `200` with `session_id`,
`credential_blob_b64` and `secret_b64`.

`enroll/finish` SHALL read `session_id` and `challenge_response_b64` and on
success return `200` with `devid_cert_pem` holding a PEM `CERTIFICATE` block.

Malformed input SHALL be rejected with `400` and a plain-text reason:
`invalid JSON` when the body does not decode, `invalid request_b64` or
`invalid signature_b64` when a start field is not base64,
`invalid signing request` when `request_b64` does not decode into a signing
request, and `invalid challenge_response_b64` when the finish field is not
base64. Unknown JSON members SHALL be ignored.

Neither leg SHALL require an IMDSv2 token: the handlers never consult the
token store, and the guest client sends no `X-Aws-Ec2-Metadata-Token` header.

#### Scenario: Start returns a challenge

- **GIVEN** an authenticated caller and a signing request that passes
  verification
- **WHEN** it posts `request_b64` and `signature_b64` to
  `/latest/devid/enroll/start`
- **THEN** the response is `200` with `session_id`, `credential_blob_b64` and
  `secret_b64`

#### Scenario: Finish returns the certificate

- **GIVEN** a session created by `enroll/start` and the matching challenge
  response
- **WHEN** the caller posts `session_id` and `challenge_response_b64` to
  `/latest/devid/enroll/finish`
- **THEN** the response is `200` and `devid_cert_pem` contains a PEM
  `CERTIFICATE` block

#### Scenario: Body is not JSON

- **WHEN** a caller posts `{` to either enroll path
- **THEN** the response is `400` with body `invalid JSON`

#### Scenario: Field is not base64

- **WHEN** a caller posts `{"request_b64":"!!!"}` to the start path
- **THEN** the response is `400` with body `invalid request_b64`

#### Scenario: Enrollment without an IMDSv2 token

- **GIVEN** an authenticated caller that never called `PUT /latest/api/token`
- **WHEN** it runs both enroll legs
- **THEN** enrollment succeeds and a certificate is issued

### Requirement: Authenticate Each Leg By Caller Identity And EK Certificate

Each leg SHALL authenticate independently, before doing any enrollment work,
by requiring two factors:

1. a VM identity attached to the connection by the inventory lookup, with a
   non-empty VM ID — the lookup itself is specified by the `vm-inventory`
   capability;
2. an EK certificate in the request header `X-vtpm-mds-ek-cert`.

A missing or unmatched caller identity SHALL be rejected with `401` and body
`VM identity required (MAC not in inventory)`.

The header name SHALL be matched case-insensitively, as HTTP header names are.
Its value SHALL be standard base64, after trimming surrounding whitespace, of
either the EK certificate DER or a whole PEM `CERTIFICATE` block. An empty or
absent value SHALL be rejected with `401` and body
`missing EK certificate header`; a value that is not base64 with `401` and
`ek cert header: base64: <reason>`; a value that does not parse as a
certificate with `401` and `ek cert header: <reason>`.

Every authentication failure SHALL be answered `401` and logged as
`devid enroll auth: <reason>`; the reason text SHALL be the response body.

#### Scenario: Caller MAC is not in inventory

- **GIVEN** a request whose connection yielded no VM identity
- **WHEN** it posts to either enroll path with a valid EK certificate header
- **THEN** the response is `401` with body
  `VM identity required (MAC not in inventory)`

#### Scenario: EK certificate header missing

- **GIVEN** a caller with a VM identity
- **WHEN** it posts to either enroll path without `X-vtpm-mds-ek-cert`
- **THEN** the response is `401` with body `missing EK certificate header`

#### Scenario: EK certificate header is not base64

- **WHEN** a caller sends `X-vtpm-mds-ek-cert: !!!not-base64`
- **THEN** the response is `401` and the body starts with
  `ek cert header: base64:`

#### Scenario: EK certificate header is not a certificate

- **WHEN** a caller sends base64 of bytes that are not a certificate
- **THEN** the response is `401` and the body starts with `ek cert header:`

#### Scenario: Header name case does not matter

- **GIVEN** a caller with a VM identity and a trusted EK certificate
- **WHEN** it sends the header as `x-vtpm-mds-ek-cert` or
  `X-VTPM-MDS-EK-CERT`
- **THEN** the header is accepted and the request proceeds past authentication

### Requirement: Trust The EK Certificate Through The Configured EK CA Chain

On both legs the service SHALL verify the header EK certificate against the
root pool built from `ek_ca_chain`, accepting any extended key usage. A
certificate that cannot be chained, or whose validity period does not cover
the current time, SHALL be rejected with `401` and body
`EK certificate not trusted: EK certificate verification failed: <reason>`.

Because swtpm-issued EK certificates carry a critical Subject Alternative Name
extension that the Go verifier does not handle, verification SHALL drop the SAN
OID from the certificate's unhandled critical extensions before chain building,
and SHALL keep every other unhandled critical extension.

Verification is chain building only. The service does not compare the
certificate's public key with the endorsement key carried in the signing
request, and performs no revocation checking. A caller presenting any EK
certificate that chains to `ek_ca_chain` is therefore accepted even when the
credential-activation challenge is wrapped to an unrelated endorsement key, so
possession of a trusted EK certificate — which is not secret — rather than
possession of the certified TPM is what this factor establishes.

#### Scenario: EK certificate not issued by a configured CA

- **GIVEN** an EK certificate that does not chain to `ek_ca_chain`
- **WHEN** a caller sends it in `X-vtpm-mds-ek-cert`
- **THEN** the response is `401` and the body starts with
  `EK certificate not trusted:`

#### Scenario: EK certificate has expired

- **GIVEN** an EK certificate issued by a configured CA whose `notAfter` is in
  the past
- **WHEN** a caller sends it in `X-vtpm-mds-ek-cert`
- **THEN** the response is `401` and the body reports
  `x509: certificate has expired or is not yet valid`

#### Scenario: EK certificate with a critical SAN extension

- **GIVEN** an swtpm EK certificate whose SAN extension is marked critical
- **WHEN** it chains to `ek_ca_chain`
- **THEN** verification succeeds and the request proceeds

#### Scenario: EK certificate public key is not bound to the TPM

- **GIVEN** an EK certificate that chains to `ek_ca_chain` but whose public key
  belongs to no TPM the caller controls
- **WHEN** the caller enrolls with that certificate in the header and in the
  signing request, while the signing request carries its own TPM's endorsement
  key
- **THEN** enrollment succeeds and a certificate is issued

### Requirement: Bind The EK Certificate To Request, Pin And Session

On `enroll/start` the header EK certificate SHALL be byte-for-byte identical,
in DER, to the endorsement certificate inside the signing request; otherwise
the request SHALL be rejected with `401` and body
`EK certificate header does not match signing request`. Start SHALL also
re-check that the signing request's endorsement certificate has the
fingerprint derived from the authenticated header.

The EK fingerprint used for binding SHALL be the lowercase hex SHA-256 digest
of the certificate DER.

When the caller's inventory entry carries a non-empty `ek_sha256` pin, both
legs SHALL require the header EK certificate fingerprint to equal that pin,
compared case-insensitively, and SHALL otherwise reject the request with `401`
and body `EK certificate does not match inventory pin`. This is the only place
the pin is enforced. The pin's source is specified by the `vm-inventory`
capability.

On `enroll/finish` the header EK certificate fingerprint SHALL equal the
fingerprint bound to the session at start, and the caller's VM ID SHALL equal
the VM ID bound at start. A mismatch SHALL be rejected with `401` and body
`EK certificate does not match enroll session` or
`VM identity does not match enroll session` respectively.

#### Scenario: Header certificate differs from the signing request

- **GIVEN** an authenticated caller whose header EK certificate is trusted
- **WHEN** its signing request carries a different endorsement certificate
- **THEN** the response is `401` with body
  `EK certificate header does not match signing request`

#### Scenario: Inventory pin does not match

- **GIVEN** an inventory entry with `ek_sha256` set to another certificate's
  digest
- **WHEN** the caller posts to either enroll leg with its own trusted EK
  certificate
- **THEN** the response is `401` with body
  `EK certificate does not match inventory pin`

#### Scenario: Finishing with a different EK certificate

- **GIVEN** a session started with one trusted EK certificate
- **WHEN** `enroll/finish` is called with a different trusted EK certificate
- **THEN** the response is `401` with body
  `EK certificate does not match enroll session`

#### Scenario: Finishing from a different VM

- **GIVEN** a session started by the VM with ID `100`
- **WHEN** a caller identified as VM `999` posts that `session_id`
- **THEN** the response is `401` with body
  `VM identity does not match enroll session`

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

The request's platform identity is parsed as an RDN sequence but not verified
against the caller's identity; it is only a source for the certificate subject.

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

### Requirement: Prove TPM Residency With A Credential Activation Challenge

`enroll/start` SHALL create the challenge by wrapping a fresh random nonce,
whose length is the digest size of the endorsement key's name algorithm, to the
endorsement key carried in the signing request, naming the request's
attestation key. The endorsement key SHALL be RSA and SHALL carry symmetric
parameters; otherwise start SHALL fail with `400` and body
`create challenge: only RSA EK is supported` or
`create challenge: EK missing symmetric parameters`.

The returned `credential_blob_b64` and `secret_b64` SHALL have the two-byte
TPM2B size prefixes stripped, so the guest can pass them straight to
`TPM2_ActivateCredential`.

The guest SHALL recover the nonce by activating the credential with its
attestation key and endorsement key under an endorsement policy session.

`enroll/finish` SHALL require `challenge_response_b64` to decode to bytes equal
to the stored nonce, and SHALL otherwise reject the request with `400` and body
`challenge verification failed`.

#### Scenario: Guest solves the challenge

- **GIVEN** a start response for a signing request from a real TPM
- **WHEN** the guest activates the credential and posts the recovered nonce
- **THEN** the response is `200` with an issued certificate

#### Scenario: Wrong challenge response

- **GIVEN** a valid `session_id`
- **WHEN** `challenge_response_b64` decodes to anything other than the stored
  nonce
- **THEN** the response is `400` with body `challenge verification failed`

#### Scenario: Endorsement key is not RSA

- **WHEN** a signing request carries a non-RSA endorsement key
- **THEN** `enroll/start` responds `400` with body
  `create challenge: only RSA EK is supported`

### Requirement: Consume Enroll Sessions Once Within Their Lifetime

`enroll/start` SHALL create a session identified by 16 bytes from a
cryptographic random source, rendered as 32 lowercase hex characters, holding
the nonce, the verified signing request, the subject common name, the caller's
VM ID and the EK certificate fingerprint.

Sessions SHALL live at most 5 minutes from creation. Expired sessions SHALL be
purged whenever a session is created or looked up. Looking a session up SHALL
remove it, so a session is usable exactly once.

An unknown, already-used or expired `session_id` SHALL be rejected with `400`
and body `unknown or expired session`. Sessions are held in memory only and
SHALL NOT survive a daemon restart.

#### Scenario: Session is single use

- **GIVEN** a session that has completed `enroll/finish` successfully
- **WHEN** the same `session_id` is posted again with the same challenge
  response
- **THEN** the response is `400` with body `unknown or expired session`

#### Scenario: Session expires

- **GIVEN** a session older than the session lifetime
- **WHEN** its `session_id` is posted to `enroll/finish`
- **THEN** the response is `400` with body `unknown or expired session`

#### Scenario: Unknown session id

- **WHEN** `enroll/finish` is posted with a `session_id` the service never
  issued
- **THEN** the response is `400` with body `unknown or expired session`

### Requirement: Issue A Non-Expiring LDevID With TCG DevID Encoding

On a successful finish the service SHALL sign a certificate with the DevID CA
containing:

- the DevID public key from the signing request as the certificate public key;
- a subject taken from the signing request's platform identity, with the
  authenticated VM ID used as common name only when the platform identity
  carries none — a platform identity common name supplied by the guest
  therefore wins over the authenticated VM ID;
- `notBefore` set to the time of the finish call and `notAfter` set to
  `9999-12-31T23:59:59Z`, so LDevIDs do not expire;
- key usage digitalSignature, basic constraints present with CA false, and the
  extended key usage OID `2.23.133.11.1.2` (tcg-cap-verifiedTPMFixed);
- a Subject Alternative Name extension carrying a TCG HardwareModuleName with
  hardware type `2.23.133.1.2` and a PermanentIdentifier with assigner
  `2.23.133.12.1`, built from the SHA-256 digest of the request's endorsement
  public key when one is present and from the endorsement certificate DER
  otherwise, and marked critical exactly when the subject is empty;
- a serial number derived deterministically from the CA certificate, the
  subject and the public key, so re-enrolling the same key with the same
  subject yields the same serial.

The certificate SHALL be returned PEM-encoded in `devid_cert_pem`. The service
SHALL NOT publish a revocation list or offer any other way to withdraw an
issued LDevID.

#### Scenario: Guest-supplied common name is used

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries a platform identity with common name
  `other-name`
- **THEN** the issued certificate's subject is `CN=other-name`

#### Scenario: VM ID is the fallback common name

- **GIVEN** an authenticated caller identified as VM `100`
- **WHEN** its signing request carries no platform identity
- **THEN** the issued certificate's subject is `CN=100`

#### Scenario: Certificate shape

- **WHEN** a certificate is issued
- **THEN** it has `notAfter` `9999-12-31T23:59:59Z`, key usage
  digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, and a TCG
  SAN with a HardwareModuleName and a PermanentIdentifier

### Requirement: Write DevID Materials On The Guest

The guest client SHALL build its signing request from the TPM: read the EK
certificate from NV index `0x01c00002`, create or reuse the cached EK, create
an attestation key and a DevID key under the SRK, certify the DevID key with
the attestation key, and sign the marshalled request with the DevID key. When
that certificate cannot be read or parsed the client SHALL fail before
contacting the service, reporting `read EK certificate NV: <reason>` or
`parse EK certificate: <reason>`, and it SHALL refuse to enroll at all without
an endorsement certificate.

The client SHALL send the base64 EK certificate in `X-vtpm-mds-ek-cert` on both
legs, treat any response status of 300 or above as a failure reporting
`HTTP <status>: <body>`, and exit non-zero on any failure.

On success the client SHALL create the output directory with mode `0700` and
write exactly three files with mode `0600`: `devid.crt.pem` with the issued
certificate, `devid.priv.blob` with the DevID TPM2B_PRIVATE and
`devid.pub.blob` with the DevID TPM2B_PUBLIC, as SPIRE's `tpm_devid` attestor
consumes them. Nothing SHALL be written when a leg fails.

Defaults SHALL be TPM `/dev/tpm0`, service base URL `http://169.254.169.254`,
output directory `/var/lib/vtpm-mds/devid` and an empty platform common name.
The client SHALL check only that `devid_cert_pem` is non-empty: it does not
verify the issued certificate against the DevID CA, nor that the certificate
matches its DevID key.

#### Scenario: Successful enrollment writes SPIRE materials

- **GIVEN** a guest whose TPM holds an EK certificate trusted by the service
  and whose MAC is in the inventory
- **WHEN** `devid-enroll` runs
- **THEN** `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob` exist in the
  output directory with mode `0600`

#### Scenario: Guest TPM has no EK certificate

- **GIVEN** a TPM with nothing at NV index `0x01c00002`
- **WHEN** `devid-enroll` runs
- **THEN** it fails with `read EK certificate NV` and sends no HTTP request

#### Scenario: Service rejects the caller

- **GIVEN** a guest whose MAC is not in the inventory
- **WHEN** `devid-enroll` runs
- **THEN** it reports `HTTP 401:` with the service's reason, exits non-zero,
  and writes no DevID files
