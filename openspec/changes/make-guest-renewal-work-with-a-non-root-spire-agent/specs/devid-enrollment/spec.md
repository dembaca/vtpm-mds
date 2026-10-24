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

When the client runs as root, each of those files SHALL have the owner and
group of the file it replaces at that path, or, when there is none, of the
output directory, so that a SPIRE agent running as another user can still
read materials renewed by a root process. A client that cannot set the owner
because it does not run as root SHALL leave the owner as the file was created
with; a client running as root that fails to set it SHALL fail without
replacing the file.

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

Defaults SHALL be TPM `/dev/tpmrm0` (the kernel resource manager, which
flushes a process's transient objects when it closes the device), service base URL `http://169.254.169.254`,
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

#### Scenario: Renewal by root keeps the SPIRE agent's ownership

- **GIVEN** an output directory and DevID files owned by the SPIRE agent's
  user, and a client running as root
- **WHEN** a renewal replaces the files
- **THEN** the new `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob`
  are owned by that user and group with mode `0600`

#### Scenario: A killed run leaves nothing loaded

- **GIVEN** the default TPM device
- **WHEN** a run is killed while it holds keys loaded in the TPM
- **THEN** no transient object of that run is left loaded afterwards

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
the SPIRE agent's cached SVID under the agent data directory and then queue a
`try-restart` of the SPIRE agent unit without waiting for it, so that the next
start performs node attestation with the new LDevID and a renewal oneshot
ordered before that unit can finish first; if the unit is absent or inactive, clearing and
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
