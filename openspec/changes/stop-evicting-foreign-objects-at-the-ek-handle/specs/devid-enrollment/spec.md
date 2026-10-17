# Spec Delta

## MODIFIED Requirements

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
