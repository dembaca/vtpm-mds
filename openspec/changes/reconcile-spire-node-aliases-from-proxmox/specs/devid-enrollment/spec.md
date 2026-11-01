## MODIFIED Requirements

### Requirement: Log Each Issued LDevID

On every successful finish the service SHALL write exactly one log line stating
that it issued a certificate, holding the certificate's serial number in
hexadecimal, the authenticated VM ID, the SHA-256 fingerprint, in hexadecimal,
of the EK certificate the enrolment was bound to, and the SHA-1 fingerprint, in
lowercase hexadecimal, of the issued certificate's DER, which is the last path
segment of the agent ID SPIRE's `tpm_devid` attestor gives that certificate. The
line SHALL NOT hold the certificate, the DevID or endorsement public key, the
challenge or its response, the session identifier or the signing request. A
finish that fails SHALL NOT write this line. Writing it SHALL NOT change the
response to the guest.

#### Scenario: A successful finish is logged once

- **GIVEN** an authenticated caller identified as VM `100` completing enrolment
- **WHEN** the finish succeeds and the certificate is returned
- **THEN** exactly one issuance line is written, with the certificate's serial, `100`
  and the EK fingerprint that the start leg bound

#### Scenario: The line names the agent ID SPIRE will use

- **GIVEN** a successful finish that returns a leaf certificate
- **WHEN** the issuance line is written
- **THEN** its `leaf=` value equals the lowercase hex SHA-1 of that leaf's DER

#### Scenario: A failed finish is not logged as an issuance

- **GIVEN** an enrolment whose challenge response is wrong
- **WHEN** the finish is refused
- **THEN** no issuance line is written, and the refusal is answered as before

#### Scenario: The line holds nothing secret

- **WHEN** an issuance line is written
- **THEN** it holds neither the PEM of the certificate, nor the nonce, nor the
  session identifier
