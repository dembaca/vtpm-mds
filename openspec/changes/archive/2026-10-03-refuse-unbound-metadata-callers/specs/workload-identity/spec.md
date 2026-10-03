# Spec Delta

## Purpose

Issue a short-lived JSON Web Token naming the guest the service is talking to,
so a workload inside that guest can present a bearer identity document to a
verifier, and publish the key set a verifier would use. This capability covers
`GET /latest/identity` and `GET /.well-known/jwks.json`: who may obtain a
document, whom it names, and what its signature is and is not worth. TPM quote
verification and real signing keys are not implemented and are not specified
here.

## ADDED Requirements

### Requirement: Register The Identity Endpoints With TPM Attestation

The service SHALL register `GET /latest/identity` and
`GET /.well-known/jwks.json` only when `mds.enable_tpm_attestation` is true.
When it is false neither route SHALL be registered, and requests for them SHALL
fall through to the catch-all handler, which SHALL respond `404` with the body
`404 page not found`.

Neither route SHALL depend on `mds.enable_ec2_compat`, on a DevID CA, or on a
TPM device being present.

#### Scenario: Endpoints registered with attestation enabled

- **GIVEN** `mds.enable_tpm_attestation` is true
- **WHEN** the daemon starts
- **THEN** `GET /latest/identity` and `GET /.well-known/jwks.json` are routed

#### Scenario: Endpoints absent with attestation disabled

- **GIVEN** `mds.enable_tpm_attestation` is false
- **WHEN** a caller sends `GET /latest/identity`
- **THEN** the response is `404` with the body `404 page not found`

### Requirement: Require A Session Token For The Identity Document

`GET /latest/identity` SHALL require the request header
`X-Aws-Ec2-Metadata-Token` to carry a token the token store holds and that has
not expired, on the same terms as a metadata read.

A request with no token, an empty token, an unknown token, an expired token, or
made while the token store has not been initialized SHALL be rejected
identically: status `401`, response header `X-Aws-Ec2-Metadata-Token:
required`, and the body `IMDSv2 token required`.

`GET /.well-known/jwks.json` SHALL NOT require a token, because it publishes
only public material.

#### Scenario: Identity document without a token

- **WHEN** a caller sends `GET /latest/identity` with no
  `X-Aws-Ec2-Metadata-Token` header
- **THEN** the response is `401` with header
  `X-Aws-Ec2-Metadata-Token: required` and the body `IMDSv2 token required`

#### Scenario: Key set needs no token

- **WHEN** a caller sends `GET /.well-known/jwks.json` with no token
- **THEN** the response is `200` with `Content-Type: application/json`

### Requirement: Name The Bound VM In The Identity Document

The document SHALL name the VM record that the `vm-inventory` capability bound
to the connection. The `sub` claim and the `instance_id` claim SHALL both be
`i-` followed by that record's VM id, and SHALL be derived exactly as
`GET /latest/meta-data/instance-id` derives its value.

No claim SHALL be derived from a request header that names the caller. In
particular `X-Forwarded-For` SHALL NOT be consulted: the `ip` claim SHALL be
the IPv4 address of the connection's peer, and SHALL be the empty string when
the peer has no IPv4 address.

When no VM record is bound and `mds.require_vm_identity` is true, the request
SHALL be refused after the token check and before any claim is derived, with
`404` and the body `404 page not found`, exactly as the metadata handlers
refuse an unbound caller. When that setting is false, the `sub` and
`instance_id` claims SHALL be synthesized from the peer address in the same way
the metadata instance id is.

The `hostname` claim SHALL be the request's `Host` header truncated at the
first colon, or `localhost.localdomain` when `Host` is empty; it is the address
the caller used, not the guest's own hostname, which the service does not know.

#### Scenario: Bound caller is named by its VM id

- **GIVEN** `mds.enable_tpm_attestation` is true, a valid session token, and a
  caller bound to VM `100`
- **WHEN** it sends `GET /latest/identity`
- **THEN** the response is `200` and the claims report `sub` and `instance_id`
  as `i-100`

#### Scenario: Forwarded-for header cannot name the caller

- **GIVEN** a valid session token and a caller bound to VM `100`
- **WHEN** it sends `GET /latest/identity` with `X-Forwarded-For: 10.9.9.9`
- **THEN** the claims still report `i-100` and the `ip` claim is the peer
  address, not `10.9.9.9`

#### Scenario: Unbound caller is refused

- **GIVEN** `mds.require_vm_identity` at its built-in default of true, a valid
  session token, and a caller bound to no VM record
- **WHEN** it sends `GET /latest/identity`
- **THEN** the response is `404` with the body `404 page not found` and no
  token is issued

### Requirement: Serve The Identity Document As An Unverifiable Placeholder

The response SHALL be `200` with `Content-Type: application/json` and a JSON
object carrying `token`, `claims` and `jwks_uri`. `jwks_uri` SHALL be
`/.well-known/jwks.json`.

`token` SHALL be a JWT with issuer `vtpm-mds`, an `iat` of the time of the
request, an `exp` five minutes later, a `jti`, and the claims `instance_id`,
`hostname`, `ip`, `level` and `attest`. `level` SHALL be `unattested` and
`attest` SHALL be false for every caller, because no attestation result is
consulted.

The token SHALL be signed with `HS256` under a fixed secret compiled into the
binary, and `GET /.well-known/jwks.json` SHALL return a fixed placeholder key
set whose only entry carries the literal modulus `placeholder-modulus`. The key
set therefore cannot verify the token, and the secret is neither private nor
per-deployment. The document SHALL NOT be treated as evidence of the caller's
identity by anything outside this service; it reports what the service believes
about the connection, with no way for a third party to check it.

`mds.jwt_ttl` SHALL be read by nothing: the five minute lifetime is fixed in
the code, so changing that setting has no effect.

#### Scenario: Document shape

- **GIVEN** a valid session token and a bound caller
- **WHEN** it sends `GET /latest/identity`
- **THEN** the response carries `token`, `claims` and
  `jwks_uri` `/.well-known/jwks.json`, and the claims report `level`
  `unattested` and `attest` false

#### Scenario: The published key set cannot verify the token

- **GIVEN** a token obtained from `GET /latest/identity`
- **WHEN** a verifier fetches `GET /.well-known/jwks.json` and tries to verify
  it
- **THEN** verification is impossible, because the token is an `HS256` JWT and
  the key set holds one placeholder RSA entry

#### Scenario: Configured JWT lifetime is ignored

- **GIVEN** a configuration whose `mds.jwt_ttl` is `1h`
- **WHEN** a bound caller obtains an identity document
- **THEN** the token's `exp` is five minutes after its `iat`
