# Spec Delta

## ADDED Requirements

### Requirement: Serve The Instance Identity Document Without A Signature

With `mds.enable_ec2_compat` enabled,
`GET /latest/dynamic/instance-identity/document` SHALL respond `200` with
`Content-Type: application/json` and a JSON object carrying exactly the keys
`instanceId`, `imageId`, `instanceType`, `region`, `availabilityZone`,
`privateIp`, `devpayProductCodes`, `version`, `billingProducts` and `accountId`.

`imageId` SHALL be `proxmox-unknown`, `instanceType` SHALL be `vm`, `region`
SHALL be `local`, `availabilityZone` SHALL be `proxmox`, `version` SHALL be
`2017-09-30`, `accountId` SHALL be `012345678901`, and `devpayProductCodes` and
`billingProducts` SHALL both be `null`. Only `instanceId` and `privateIp` vary
by caller.

`privateIp` SHALL be the IPv4 address of the connection's peer, derived exactly
as `local-ipv4` is, and SHALL NOT honour `X-Forwarded-For`, even though
`instanceId` does. When the peer has no IPv4 address `privateIp` SHALL be the
empty string; the key SHALL remain present, so the document keeps its fixed key
set.

When no instance id can be derived, as specified by
`Derive The Instance Id From The Bound VM Record`, the service SHALL NOT serve a
document with an empty or invented `instanceId`; it SHALL respond `422` with an
empty body and `Content-Type: text/plain`.

The document is not signed, and the service SHALL NOT serve a signature over
it. `GET /latest/dynamic/instance-identity/signature`, `.../pkcs7` and
`.../rsa2048` SHALL NOT be registered routes: each SHALL be answered by the
catch-all handler with `404` and the body `404 page not found`, byte for byte
what `GET /latest/no-such-thing` returns, whether or not the request carries a
valid session token and whether or not the caller is bound to a VM record. A
signature path SHALL be served only by a change that names a consumer which
verifies it against a key this service holds.

#### Scenario: Guest reads its identity document

- **GIVEN** a valid session token and a caller from `10.0.0.5`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the service responds `200` with `Content-Type: application/json`, and
  the document reports `imageId` `proxmox-unknown`, `instanceType` `vm`,
  `region` `local`, `availabilityZone` `proxmox` and `privateIp` `10.0.0.5`

#### Scenario: Identity document for a peer with no IPv4 address

- **GIVEN** a valid session token and a caller bound to VM `100` whose peer
  address is `[fe80::1]:5000`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the response is `200`, the `privateIp` key is present with the empty
  string as its value, and it is in particular not `[fe80`

#### Scenario: No document for a caller with no derivable instance id

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller whose `RemoteAddr` is `not-an-address`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the service responds `422` with an empty body

#### Scenario: Signature path does not exist

- **GIVEN** a caller bound to VM `100` with a valid session token
- **WHEN** it sends `GET /latest/dynamic/instance-identity/signature` and
  `GET /latest/no-such-thing`
- **THEN** both responses are `404` with the body `404 page not found`, and the
  first is in particular not `dGVzdC1zaWduYXR1cmU=`

#### Scenario: Signature path does not exist without a token either

- **GIVEN** a caller with no session token
- **WHEN** it sends `GET /latest/dynamic/instance-identity/signature`
- **THEN** the response is `404` with the body `404 page not found`, not `401`,
  because there is no handler to ask for a token

## MODIFIED Requirements

### Requirement: Refuse An Instance Identity To Callers With No VM Record

The service SHALL read `mds.require_vm_identity`, whose built-in default is
true.

When it is true, every handler that reports the caller's *instance identity*
SHALL require the request's connection to be bound to a VM record by the
`vm-inventory` capability, and SHALL refuse the request when it is not. Exactly
these paths SHALL be refused:

- `GET /latest/meta-data/instance-id`
- `GET /latest/dynamic/instance-identity/document`

The remaining metadata paths SHALL NOT be refused, because none of them asserts
an identity the service vouches for. `local-hostname` echoes the caller's own
`Host` header, `local-ipv4` reports the caller's own peer address,
`placement/availability-zone` and `services/domain` are fixed strings,
`public-ipv4` is always unavailable, and the index is a static listing. An
unbound caller SHALL continue to be served all of them, and the index SHALL
continue to list `instance-id` even though reading it will be refused.

The refusal SHALL be checked after the session token is validated and before
any instance value is derived, and SHALL be answered `404` with the body
`404 page not found` — byte for byte what the catch-all handler returns for an
unregistered route. An unbound caller SHALL therefore not be able to tell a
refused endpoint from one that does not exist. The service SHALL log each
refusal with the caller's peer address, so an operator can.

`PUT /latest/api/token` and `GET /health` SHALL NOT be affected: both remain
unauthenticated and are served to any caller, bound or not.

When `mds.require_vm_identity` is false none of these paths SHALL be refused,
and the instance identity SHALL be derived as specified by
`Derive The Instance Id From The Bound VM Record`.

#### Scenario: Unbound caller is refused the instance id

- **GIVEN** `mds.require_vm_identity` is true, a valid session token, and a
  caller that `vm-inventory` could not bind to any VM record
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the response is `404` with the body `404 page not found`

#### Scenario: Refusal is indistinguishable from an unknown route

- **GIVEN** an unbound caller with a valid session token
- **WHEN** it sends `GET /latest/meta-data/instance-id` and
  `GET /latest/no-such-thing`
- **THEN** both responses are `404` with the body `404 page not found`

#### Scenario: Unbound caller is refused both instance identity endpoints

- **GIVEN** `mds.require_vm_identity` is true and an unbound caller with a
  valid session token
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document` and
  `GET /latest/dynamic/instance-identity/signature`
- **THEN** both responses are `404` with the body `404 page not found`: the
  document because it is refused, the signature because the path does not
  exist for any caller, as `Serve The Instance Identity Document Without A
  Signature` specifies

#### Scenario: Unbound caller keeps the caller-derived metadata

- **GIVEN** `mds.require_vm_identity` is true and an unbound caller from
  `10.0.0.5` with a valid session token
- **WHEN** it sends `GET /latest/meta-data/`,
  `GET /latest/meta-data/local-hostname`,
  `GET /latest/meta-data/local-ipv4`,
  `GET /latest/meta-data/placement/availability-zone` and
  `GET /latest/meta-data/services/domain`
- **THEN** every response is `200`, `local-ipv4` is `10.0.0.5`, and the index
  still lists `instance-id`

#### Scenario: Bound caller is unaffected

- **GIVEN** `mds.require_vm_identity` is true and a caller bound to VM `100`
- **WHEN** it reads the metadata tree with a valid session token
- **THEN** every value is served exactly as it is without the setting

#### Scenario: Token and health endpoints stay open

- **GIVEN** `mds.require_vm_identity` is true and an unbound caller
- **WHEN** it sends `PUT /latest/api/token` and `GET /health`
- **THEN** both respond `200`

#### Scenario: A missing token is still answered as a missing token

- **GIVEN** `mds.require_vm_identity` is true and an unbound caller with no
  session token
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the response is `401` with the body `IMDSv2 token required`,
  because the token check runs first

## REMOVED Requirements

### Requirement: Serve An Unsigned Instance Identity Document

**Reason**: Its signature clause requires
`GET /latest/dynamic/instance-identity/signature` to answer every caller `200`
with the constant `dGVzdC1zaWduYXR1cmU=`, and its scenario
`Identity signature is a constant placeholder` asserts exactly that. The
requirement already said the value "SHALL NOT be treated as evidence of the
document's authenticity"; it nonetheless made serving a value with the form of
a signature part of the contract. No consumer that verifies such a signature
can be pointed at a key this service holds, so the path is removed rather than
implemented.

The scenario's outcome inverts, so it cannot be carried into a modified block.
Replaced by `Serve The Instance Identity Document Without A Signature`, which
keeps every clause about the document and every document scenario verbatim —
the fixed key set and values, `privateIp` and its IPv6 handling, and the `422`
for a caller with no derivable instance id are all unchanged.

**Migration**: A caller that reads only `document` is unaffected. A caller that
reads `signature` now receives `404` with the body `404 page not found`, as it
already did for `pkcs7` and `rsa2048`. It SHALL stop requesting the path; the
constant it received never verified anything. No such caller is known:
cloud-init reaches instance-identity leaves only through the
`dynamic/instance-identity/` listing, which this service has never served.
