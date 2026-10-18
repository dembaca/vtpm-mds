# Spec Delta

## ADDED Requirements

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
