# Spec Delta

## ADDED Requirements

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

## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Trust The EK Certificate Through The Configured EK CA Chain

**Reason**: Its paragraph beginning `Verification is chain building only` and
its scenario `EK certificate public key is not bound to the TPM` state the
defect as the contract — that a caller presenting any certificate which chains
to `ek_ca_chain` is accepted even when the challenge is wrapped to an unrelated
endorsement key, and that enrollment then succeeds. The replacement inverts
that outcome, so the scenario cannot be carried forward. Replaced by
`Trust The EK Certificate And Bind It To The Endorsement Key`, which keeps the
chain-building rules, the SAN workaround and their three scenarios verbatim.

**Migration**: None for a guest that enrolls with the TPM its EK certificate
was issued over, which is every guest in a correctly provisioned deployment.
A caller that relied on presenting a borrowed EK certificate was never
authorised to enrol and is now rejected at `enroll/start`.
