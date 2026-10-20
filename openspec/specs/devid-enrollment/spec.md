# DevID Enrollment

## Purpose

Issue IEEE 802.1AR / TCG LDevID certificates to QEMU guests over the metadata
service, so a guest can obtain SPIRE `tpm_devid` materials anchored in its
vTPM. Enrollment is a two-leg HTTP protocol — a signed TPM signing request
answered with a credential-activation challenge, then a challenge response
answered with a certificate — authenticated by the caller's inventory identity
together with an endorsement key certificate that chains to a configured EK CA
and certifies the endorsement key being enrolled.
This capability covers the wire protocol, its authentication and the guest
client; how a caller's MAC maps to a VM belongs to the `vm-inventory`
capability, and the IMDSv2 token flow belongs to `instance-metadata`.

## Requirements

### Requirement: Gate Enroll Routes On Attestation And DevID CA Material

The service SHALL mount `POST /latest/devid/enroll/start` and
`POST /latest/devid/enroll/finish` only when `enable_tpm_attestation` is true
and a DevID CA can be loaded. The CA key SHALL come from a systemd credential
when the environment variable `CREDENTIALS_DIRECTORY` names a directory holding a
non-empty file `devid_ca.key`; the service SHALL then not read or stat the
configured `devid_ca_key` at all, SHALL accept an empty `devid_ca_key`, and SHALL
log once `DevID CA key from systemd credential`. An empty credential file SHALL
count as absent. Without a usable credential, loading SHALL require both
`devid_ca_cert` and `devid_ca_key` to be non-empty and both paths to exist. In
every case the PEM material SHALL parse into a certificate plus a PKCS#8 or
PKCS#1 private key.

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

#### Scenario: A credential replaces the configured key path

- **GIVEN** `CREDENTIALS_DIRECTORY` holding a valid `devid_ca.key`, and a
  `devid_ca_key` that names a path the service cannot reach
- **WHEN** the daemon starts
- **THEN** it logs `DevID CA key from systemd credential` and
  `DevID enrollment routes registered`, and never touches the configured path

#### Scenario: No credential keeps the configured path

- **GIVEN** no `CREDENTIALS_DIRECTORY`, or one without `devid_ca.key`, or one
  whose `devid_ca.key` is empty
- **WHEN** the daemon starts
- **THEN** it loads the key from `devid_ca_key`, as before

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

### Requirement: Prove TPM Residency With A Credential Activation Challenge

`enroll/start` SHALL create the challenge by wrapping a fresh random nonce,
whose length is the digest size of the endorsement key's name algorithm, to the
endorsement key carried in the signing request, naming the request's
attestation key. The endorsement key SHALL be RSA and SHALL carry symmetric
parameters; otherwise start SHALL fail with `400` and body
`create challenge: only RSA EK is supported` or
`create challenge: EK missing symmetric parameters`.

Because authentication has already bound the trusted EK certificate to that
endorsement key, the challenge SHALL be wrapped to the key the configured EK CA
certified. Only the TPM holding that endorsement key can recover the nonce, so
a successful finish proves the attestation key — and through the residency
check the DevID key — lives in the certified TPM.

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

### Requirement: Write DevID Materials On The Guest

The guest client SHALL build its signing request from the TPM: read the EK
certificate from NV index `0x01c00002`, derive the endorsement key, create
an attestation key and a DevID key under the storage root key that SPIRE's
`tpm_devid` attestor derives, certify the DevID key with the attestation key,
and sign the marshalled request with the DevID key. When
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

That storage root key SHALL be derived as SPIRE's attestor derives it, because
the DevID key can only be loaded under the primary key it was created under.
The client SHALL create the parent as a transient primary key in the owner
hierarchy from the TCG "H-1" template: the go-tpm-tools RSA storage-key
template with its `unique` field (`ModulusRaw`) **empty**, not 256 zero bytes.
It SHALL flush that parent when it is done, SHALL NOT make it persistent, and
SHALL NOT read, reuse, replace or evict any persistent object at `0x81000001`
or any other persistent handle on behalf of the storage root key. The DevID
key and the attestation key SHALL both be created under that parent.

The endorsement key SHALL be derived afresh, as a transient primary key in the
endorsement hierarchy from the go-tpm-tools default RSA EK template, because
the EK certificate in NV certifies the key that template yields on that TPM. The
client SHALL read the EK's public area for the signing request and flush the EK
at once, and SHALL derive it again for credential activation and flush it after
use, so that the endorsement key is never loaded together with the attestation
key, the DevID key and the storage root key. It SHALL NOT make the EK
persistent and SHALL NOT read, reuse, replace or evict any persistent object at
`0x81010001` or any other persistent handle on behalf of the endorsement key.

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

#### Scenario: DevID key loads under SPIRE's storage root key

- **GIVEN** a TPM with no persistent object at `0x81000001`
- **WHEN** `devid-enroll` runs and the resulting `devid.pub.blob` and
  `devid.priv.blob` are loaded under a primary key created in the owner
  hierarchy from the H-1 template, as SPIRE's `tpm_devid` attestor does
- **THEN** the load succeeds

#### Scenario: A persistent object at the reserved SRK handle is left alone

- **GIVEN** a TPM that holds a persistent object at `0x81000001`
- **WHEN** `devid-enroll` runs
- **THEN** the object is still there afterwards with the same public area, and
  the enrolled DevID key loads under the H-1 primary as in the previous
  scenario

#### Scenario: A persistent object at the reserved EK handle is left alone

- **GIVEN** a TPM that holds a persistent object at `0x81010001`, whether a
  key from another template or a key from the EK template with another
  `unique`
- **WHEN** `devid-enroll` runs
- **THEN** the object is still there afterwards with the same public area, and
  the EK in the signing request is the key derived from the default EK template

#### Scenario: Four keys are never loaded at once

- **GIVEN** a TPM with three transient object slots
- **WHEN** `devid-enroll` creates its keys, builds the signing request and
  solves the credential challenge
- **THEN** no command fails for lack of object memory, and no transient handle
  is left loaded afterwards

#### Scenario: Guest TPM has no EK certificate

- **GIVEN** a TPM with nothing at NV index `0x01c00002`
- **WHEN** `devid-enroll` runs
- **THEN** it fails with `read EK certificate NV` and sends no HTTP request

#### Scenario: Service rejects the caller

- **GIVEN** a guest whose MAC is not in the inventory
- **WHEN** `devid-enroll` runs
- **THEN** it reports `HTTP 401:` with the service's reason, exits non-zero,
  and writes no DevID files

### Requirement: Trust The EK Certificate And Bind It To The Endorsement Key

On both legs the service SHALL verify the header EK certificate against the
root pool built from `ek_ca_chain`, accepting any extended key usage. A
certificate that cannot be chained, or whose validity period does not cover
the current time, SHALL be rejected with `401` and body
`EK certificate not trusted: EK certificate verification failed: <reason>`.

Because swtpm-issued EK certificates carry a critical Subject Alternative Name
extension that the Go verifier does not handle, verification SHALL drop the SAN
OID from the certificate's unhandled critical extensions before chain building,
and SHALL keep every other unhandled critical extension.

On `enroll/start` the service SHALL additionally require that the certificate
certifies the endorsement key the caller is enrolling with: the public key of
the header EK certificate SHALL equal the endorsement key public area carried
in the signing request. A request that fails this check SHALL be rejected with
`401` and body
`EK certificate does not match the endorsement key in the signing request`.
A signing request that carries an endorsement certificate but no endorsement
key public area SHALL be rejected the same way, because the binding cannot be
established; the service SHALL NOT proceed to challenge creation in that case.

Because the challenge is wrapped to the endorsement key from the signing
request, this binding is what makes the EK factor prove possession of the
certified TPM rather than possession of the certificate. An EK certificate is
not secret — it is readable from the TPM's NV storage and is sent in the clear
on both legs — so chain building alone establishes nothing about the caller.

`enroll/finish` SHALL verify the chain only. It carries no signing request, and
the session binds the fingerprint of the certificate whose binding was proven
at start, so the caller cannot substitute another certificate there.

The service SHALL perform no revocation checking on either leg.

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

#### Scenario: EK certificate certifies another key than the one being enrolled

- **GIVEN** an EK certificate that chains to `ek_ca_chain` but whose public key
  is not the caller's endorsement key
- **WHEN** the caller posts to `/latest/devid/enroll/start` with that
  certificate in the header and in the signing request, while the signing
  request carries its own TPM's endorsement key
- **THEN** the response is `401` with body
  `EK certificate does not match the endorsement key in the signing request`,
  no session is created, and no certificate is issued

#### Scenario: Signing request omits the endorsement key

- **GIVEN** an authenticated caller whose header EK certificate is trusted and
  matches the certificate in its signing request
- **WHEN** that signing request carries no endorsement key public area
- **THEN** the response is `401` with body
  `EK certificate does not match the endorsement key in the signing request`

#### Scenario: A guest enrolling with its own TPM is unaffected

- **GIVEN** a guest whose EK certificate was issued over its own TPM's
  endorsement key, as an swtpm-issued certificate is
- **WHEN** it runs both enroll legs
- **THEN** the binding check passes and a certificate is issued

### Requirement: Log Each Issued LDevID

On every successful finish the service SHALL write exactly one log line stating
that it issued a certificate, holding the certificate's serial number in
hexadecimal, the authenticated VM ID, and the SHA-256 fingerprint, in hexadecimal,
of the EK certificate the enrolment was bound to. The line SHALL NOT hold the
certificate, the DevID or endorsement public key, the challenge or its response, the
session identifier or the signing request. A finish that fails SHALL NOT write this
line. Writing it SHALL NOT change the response to the guest.

#### Scenario: A successful finish is logged once

- **GIVEN** an authenticated caller identified as VM `100` completing enrolment
- **WHEN** the finish succeeds and the certificate is returned
- **THEN** exactly one issuance line is written, with the certificate's serial, `100`
  and the EK fingerprint that the start leg bound

#### Scenario: A failed finish is not logged as an issuance

- **GIVEN** an enrolment whose challenge response is wrong
- **WHEN** the finish is refused
- **THEN** no issuance line is written, and the refusal is answered as before

#### Scenario: The line holds nothing secret

- **WHEN** an issuance line is written
- **THEN** it holds neither the PEM of the certificate, nor the nonce, nor the
  session identifier

### Requirement: Limit Enroll Attempts Per Peer

`POST /latest/devid/enroll/start` and `POST /latest/devid/enroll/finish` SHALL
count toward a per-peer enroll budget of at most 6 requests within each 60
second window, using the same peer key as the instance-metadata guest
throttle. A request that exceeds that budget SHALL be refused with status
`429`, `Content-Type: text/plain`, body `Too many requests` followed by a
newline, and a `Retry-After` header, and SHALL NOT create a session, consume a
session, or issue a certificate.

Those requests SHALL also count toward the general per-peer HTTP request
budget of the instance-metadata guest throttle. The enroll budget SHALL NOT
replace authentication, EK binding, or any other enroll check: a peer under
budget is still subject to every existing enroll requirement.

#### Scenario: Enroll flood from one peer is refused

- **WHEN** a single peer has already sent 6 enroll start or finish requests
  within 60 seconds and sends another
- **THEN** the service responds `429` with body `Too many requests` and does
  not create or consume an enroll session

#### Scenario: A normal enroll stays under budget

- **WHEN** a peer performs one successful `enroll/start` and one successful
  `enroll/finish` within 60 seconds and is under the general request budget
- **THEN** both legs are handled by the existing enroll requirements and are
  not refused as `429` for the enroll limit

### Requirement: Persist An EK Pin Per VM After First Successful Enrollment

On `enroll/finish`, when the caller's VMID has **no** inventory `ek_sha256`
and **no** pin file yet, the service SHALL create
`/var/lib/<package>/ek-pins/<vmid>` (one line: lowercase hex SHA-256 of that
enrollment's EK certificate DER) with an exclusive create (`O_EXCL` or
equivalent) **before** signing the LDevID. The `ek-pins` directory SHALL be
created if missing. Two concurrent first finishes SHALL leave a single pin.

If that create fails for any reason other than the file already existing, the
finish SHALL answer `500` and SHALL NOT return an LDevID PEM. When a pin
already exists (including the concurrent loser), finish SHALL NOT rewrite it
and SHALL enforce it before signing. The store applies for both YAML and
Proxmox inventory whenever `ek_sha256` is empty.

The service SHALL NOT delete or replace a pin automatically on TPM reset,
failed enroll, inventory reload, or absence of the VM from inventory.

#### Scenario: First successful finish creates a pin file

- **GIVEN** VM `100` with empty inventory `ek_sha256` and no file
  `/var/lib/<package>/ek-pins/100`
- **WHEN** `enroll/finish` succeeds for that VM with EK fingerprint `abc…`
- **THEN** the file `/var/lib/<package>/ek-pins/100` contains `abc…` and a
  later enroll with a different trusted EK for VM `100` is rejected

#### Scenario: Second finish does not overwrite an existing pin

- **GIVEN** `/var/lib/<package>/ek-pins/100` already holding fingerprint `abc…`
- **WHEN** `enroll/finish` succeeds again with the same EK
- **THEN** the file content remains `abc…`

#### Scenario: Unusable pin directory fails closed before issuance

- **GIVEN** VM `100` with no pin yet and `/var/lib/<package>/ek-pins` not
  writable by the service user
- **WHEN** `enroll/finish` would otherwise issue an LDevID
- **THEN** the response is `5xx`, no LDevID PEM is returned, and no pin file
  is created

### Requirement: Enforce The Stored EK Pin When Inventory Has None

On both `enroll/start` and `enroll/finish`, after resolving the caller VMID,
the effective EK pin SHALL be:

1. the inventory `ek_sha256` if non-empty after trim; otherwise
2. the contents of `/var/lib/<package>/ek-pins/<vmid>` if that file exists and
   contains a non-empty fingerprint; otherwise
3. no pin.

When the effective pin is non-empty, both legs SHALL require the header EK
certificate fingerprint to equal it (case-insensitive). A mismatch SHALL be
rejected with `401` and body `EK certificate does not match inventory pin`
when the pin came from inventory, or `EK certificate does not match stored pin`
when the pin came from the store file.

This extends, and does not replace, the existing inventory-pin scenarios in
"Bind The EK Certificate To Request, Pin And Session".

#### Scenario: Stored pin rejects a different EK

- **GIVEN** `/var/lib/<package>/ek-pins/100` holding fingerprint `abc…` and
  inventory `ek_sha256` empty
- **WHEN** either enroll leg is called for VM `100` with a trusted EK whose
  fingerprint is not `abc…`
- **THEN** the response is `401` with body
  `EK certificate does not match stored pin`

#### Scenario: Inventory pin wins over a conflicting store file

- **GIVEN** inventory `ek_sha256` set to fingerprint `aaa…` and store file
  for the same VMID holding `bbb…`
- **WHEN** either enroll leg is called with EK fingerprint `aaa…`
- **THEN** the request is not rejected for pin mismatch (other auth checks
  still apply)

#### Scenario: No pin allows first enroll

- **GIVEN** empty inventory `ek_sha256` and no store file for the VMID
- **WHEN** either enroll leg is called with a trusted EK bound to the
  endorsement key
- **THEN** the pin check does not reject the request

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
  the DevID key and subject are unchanged. Whatever platform identity the
  request carries does not enter the serial (it is not part of the subject).

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
