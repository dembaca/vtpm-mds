# Design

## Context

See proposal.md — Why.

`VerifyEKCertificate(roots *x509.CertPool, pub *tpm2.Public, cert *x509.Certificate)`
already takes the endorsement key. Its body contains `_ = pub` and a comment
calling the comparison an optional future step. Two call sites pass different
things:

- `auth.go`, on both legs, passes `nil` — at that point there is no signing
  request on the finish leg, so no endorsement key is available.
- `enroll.go`, inside `Enroller.Start`, passes `sr.EndorsementKey` — which is
  the one place the key *is* available, and the one place the parameter is
  ignored.

So the signature suggests a check that no call site performs, and the `nil`
call site is what makes the omission invisible. That shape is the defect as
much as the missing comparison is.

`HandleStart` already decodes the signing request before authenticating, in
order to pass `sr.EndorsementCertificate` for the header/request DER equality
check. The endorsement key is in the same struct.

## Goals / Non-Goals

**Goals:**

- The EK factor proves possession of the certified TPM.
- A trusted EK certificate for one TPM cannot be used to enrol another.
- The check cannot be forgotten again by passing `nil`.
- Legitimate enrollment against a real vTPM is unchanged.

**Non-Goals:**

- Revocation checking, OCSP or CRL for EK certificates. Absent today, and
  adding it changes what an operator must run alongside the service.
- Requiring an `ek_sha256` pin, or changing how the pin is configured. The pin
  becomes meaningful as a side effect; that is not a behaviour change.
- Supporting non-RSA endorsement keys. Challenge creation already requires RSA
  and says so.
- Changing the guest client, the wire format, or the issued certificate.
- Binding the EK on `enroll/finish` directly. It has no signing request, and
  the session fingerprint already carries the binding proven at start.

## Decisions

### Split the function rather than make the parameter optional

`VerifyEKCertificate` becomes two functions with names that state what they
check: a chain-only verification, and one that additionally binds the
certificate to an endorsement key public area. The finish leg calls the
chain-only form deliberately, and the start leg cannot reach the chain-only
form by accident.

Keeping one function and treating `nil` as "skip the binding" was rejected. It
is precisely the shape that produced this defect: the omission is invisible at
the call site and looks like an ordinary optional argument.

### Bind during authentication, not during signing-request verification

The check moves into `AuthenticateEnrollCaller`, which already receives the
signing request's endorsement certificate on the start leg and gains its
endorsement key alongside. Consequences:

- The failure is a `401`, logged as `devid enroll auth:`, consistent with every
  other EK factor. A `400` would file a failed proof of possession alongside
  malformed base64.
- It runs before challenge creation, so a request that cannot be bound never
  causes a nonce to be generated or a session to be stored.
- All three EK factors — trusted chain, equals the certificate in the request,
  certifies the key in the request — are decided in one place.

The alternative, honouring `pub` where `Enroller.Start` already passes it, was
rejected because `Start` runs after authentication and its errors are `400`,
which would answer an authentication failure with the wrong status.

### Compare public keys through `crypto.PublicKey.Equal`

The certificate's `PublicKey` is compared with the key returned by the
endorsement key's `Key()` using the `Equal(crypto.PublicKey) bool` method that
`*rsa.PublicKey` implements. Comparing RSA modulus and exponent by hand would
work today and silently do the wrong thing if an EC endorsement key is ever
supported. A key type that does not implement `Equal`, or two keys of different
types, is a mismatch.

### A signing request with no endorsement key is an authentication failure

Both legs already require an endorsement certificate. A request that carries
one but omits the endorsement key public area cannot be bound, so it is
rejected with the same `401` rather than being allowed through to fail later at
challenge creation with `create challenge: missing EK public`. The caller gets
the reason that actually applies.

### One error message for both mismatch cases

A wrong key and a missing key produce the same body. Distinguishing them tells
an unauthorised caller which half of its forgery was detected, and a legitimate
guest hits neither.

## Risks / Trade-offs

- **[Risk] A deployment whose EK certificates were minted over a key other than
  the TPM's endorsement key stops enrolling** → That deployment has no working
  EK factor today, so the change reveals a broken provisioning step rather than
  causing one. Mitigation: task 1 reproduces the attack *and* confirms a
  correctly provisioned swtpm guest still enrolls, so the two are told apart
  before release.
- **[Risk] The endorsement key public area in the signing request and the
  certificate encode the same key differently** → Mitigation: the comparison is
  on parsed public keys, not on encodings; `tpm2.Public.Key()` yields a
  `*rsa.PublicKey` and `x509` yields the same type.
- **[Risk] The finish leg still verifies the chain only, so it reads as the
  weaker path** → Mitigation: the spec states why it is sufficient, and the
  session's EK fingerprint plus the VM identity check bound at start are what
  carry the proof forward. Whoever changes session handling should read that
  paragraph.
- **[Trade-off] The binding is proven only at start, so a session is only as
  good as the fingerprint it stores** → Accepted; sessions are single use and
  expire in five minutes.

## Migration Plan

No configuration, state or wire-format change. Deploy the package and re-run
the guest enrollment end-to-end script; a correctly provisioned guest enrolls
exactly as before.

Rollback is reinstalling the previous package. Already-issued LDevIDs are
unaffected either way — they do not expire and there is no revocation list,
which is itself a known gap and not addressed here.

## Open Questions

None.
