## ADDED Requirements

### Requirement: Issue LDevIDs From A Per-Cluster Intermediate

The DevID CA material the service loads for enrollment SHALL be a **cluster
intermediate** certificate and its private key, not the offline DevID root.
On a successful finish the service SHALL sign the LDevID with that
intermediate.

The PEM returned in `devid_cert_pem` SHALL contain one or more PEM
`CERTIFICATE` blocks: after cut-over, the leaf followed by the intermediate
(leaf first) and SHALL NOT include the root. Before cut-over (signing CA still
the root) the second block MAY be that root. A relying party that already
trusts the offline root (SPIRE `tpm_devid` `devid_ca_path`) SHALL be able to
verify a post-cut-over leaf using the intermediate from that PEM.

The offline root private key SHALL NOT be present on the hypervisor. How the
intermediate is minted and how the root certificate reaches SPIRE are
deployment concerns settled in the design (Q1–Q2); the service only requires
readable intermediate cert+key at enroll time.

#### Scenario: Finish returns leaf then intermediate

- **GIVEN** a configured cluster intermediate and an authenticated enroll
- **WHEN** finish succeeds
- **THEN** `devid_cert_pem` decodes to at least two certificates, the first
  is the issued LDevID (subject CN = VM ID), and the second is the
  intermediate that issued it

#### Scenario: Leaf chains to the offline root through the intermediate

- **GIVEN** the offline DevID root as a trust anchor
- **WHEN** a verifier builds a chain from `devid_cert_pem`
- **THEN** the leaf verifies against that root with the intermediate from the
  PEM (and fails if the intermediate PEM is omitted)

#### Scenario: Root key is not on the node

- **GIVEN** a correctly deployed node
- **WHEN** enrollment is configured
- **THEN** the service signs with the intermediate key and does not require
  the root private key to be readable

## MODIFIED Requirements

### Requirement: Gate Enroll Routes On Attestation And DevID CA Material

The service SHALL mount `POST /latest/devid/enroll/start` and
`POST /latest/devid/enroll/finish` only when `enable_tpm_attestation` is true
and a DevID CA can be loaded. That CA SHALL be the **cluster intermediate**
certificate and key (`devid_ca_cert` / `devid_ca_key` or the systemd credential
`devid_ca.key` pointing at the intermediate key). The CA key SHALL come from a
systemd credential when the environment variable `CREDENTIALS_DIRECTORY` names
a directory holding a non-empty file `devid_ca.key`; the service SHALL then not
read or stat the configured `devid_ca_key` at all, SHALL accept an empty
`devid_ca_key`, and SHALL log once `DevID CA key from systemd credential`. An
empty credential file SHALL count as absent. Without a usable credential,
loading SHALL require both `devid_ca_cert` and `devid_ca_key` to be non-empty
and both paths to exist. In every case the PEM material SHALL parse into a
certificate plus a PKCS#8 or PKCS#1 private key. Loading SHALL use the **first**
`CERTIFICATE` block as the signing CA (a trailing root in the same file MUST
NOT become the issuer), SHALL require that certificate to be a CA
(`basicConstraints` CA:TRUE), and SHALL require the private key to match that
certificate's public key. A mismatch or non-CA certificate SHALL fail loading
the same way as an unusable path (`Warning: DevID enrollment disabled:`).

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

- **GIVEN** `enable_tpm_attestation: true` and readable intermediate
  `devid_ca_cert` and `devid_ca_key` files
- **WHEN** the daemon starts
- **THEN** it logs `DevID enrollment routes registered` and both enroll paths
  accept `POST`

#### Scenario: A credential replaces the configured key path

- **GIVEN** `CREDENTIALS_DIRECTORY` holding a valid intermediate `devid_ca.key`,
  and a `devid_ca_key` that names a path the service cannot reach
- **WHEN** the daemon starts
- **THEN** it logs `DevID CA key from systemd credential` and
  `DevID enrollment routes registered`, and never touches the configured path

#### Scenario: No credential keeps the configured path

- **GIVEN** no `CREDENTIALS_DIRECTORY`, or one without `devid_ca.key`, or one
  whose `devid_ca.key` is empty
- **WHEN** the daemon starts
- **THEN** it loads the intermediate key from `devid_ca_key`, as before

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

#### Scenario: Key and certificate must match at load

- **GIVEN** readable `devid_ca_cert` and `devid_ca_key` whose key does not
  match the certificate, or whose certificate is not a CA
- **WHEN** the daemon starts
- **THEN** it logs `Warning: DevID enrollment disabled:` and the enroll paths
  return `404` (no routes registered)

#### Scenario: First certificate in a PEM bundle is the signing CA

- **GIVEN** `devid_ca_cert` holds the intermediate PEM followed by the root PEM
- **WHEN** the CA is loaded
- **THEN** the service signs with the intermediate (first block), not the root

#### Scenario: TPM attestation disabled

- **GIVEN** `enable_tpm_attestation: false` and valid DevID CA material
- **WHEN** a guest posts to `/latest/devid/enroll/finish`
- **THEN** the response is `404`

#### Scenario: EK trust roots missing

- **GIVEN** `ek_ca_chain` is empty and DevID CA material is valid
- **WHEN** a guest posts a well-formed enroll request
- **THEN** the routes exist and the response is `401` because the EK
  certificate cannot be chained to any root

#### Scenario: Missing intermediate credential source fails as today

- **GIVEN** the packaged unit’s `LoadCredential=` source for the intermediate
  key is absent
- **WHEN** systemd starts the unit
- **THEN** the start fails (credential failure), as with today’s root key

### Requirement: Serve Enrollment As Two POST JSON Legs

Both legs SHALL accept `POST` with a JSON body, read at most 1 MiB of that
body, and answer with `Content-Type: application/json` on success.

`enroll/start` SHALL read `request_b64` and `signature_b64`, each standard
base64, and on success return `200` with `session_id`,
`credential_blob_b64` and `secret_b64`.

`enroll/finish` SHALL read `session_id` and `challenge_response_b64` and on
success return `200` with `devid_cert_pem` holding one or more PEM
`CERTIFICATE` blocks (leaf then intermediate after cut-over; see
"Issue LDevIDs From A Per-Cluster Intermediate").

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
- **THEN** the response is `200` and `devid_cert_pem` contains one or more PEM
  `CERTIFICATE` blocks

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
