# Spec Delta

## ADDED Requirements

### Requirement: Refuse An Instance Identity To Callers With No VM Record

The service SHALL read `mds.require_vm_identity`, whose built-in default is
true.

When it is true, every handler that reports the caller's *instance identity*
SHALL require the request's connection to be bound to a VM record by the
`vm-inventory` capability, and SHALL refuse the request when it is not. Exactly
these paths SHALL be refused:

- `GET /latest/meta-data/instance-id`
- `GET /latest/dynamic/instance-identity/document`
- `GET /latest/dynamic/instance-identity/signature`
- `GET /latest/identity`, as specified by `workload-identity`

The signature endpoint is refused with the document although it serves a
constant: the two form one feature, and a detached signature over a document
the caller cannot obtain reports nothing.

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
- **THEN** both responses are `404`

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

### Requirement: Derive The Instance Id From The Bound VM Record

`GET /latest/meta-data/instance-id` SHALL return `i-` followed by the VM id of
the record the `vm-inventory` capability bound to the connection.

The instance id SHALL NOT be derived from any request header. In particular
`X-Forwarded-For` SHALL NOT be consulted by this or any other handler: the
service listens directly on the link-local metadata address with nothing in
front of it, so the header is caller-supplied data and never evidence of the
caller's address.

When no VM record is bound and `mds.require_vm_identity` is true, the request
SHALL be refused as specified by
`Refuse An Instance Identity To Callers With No VM Record`.

When no VM record is bound and `mds.require_vm_identity` is false, the handler
SHALL synthesize an id as `i-` followed by the peer address of the connection
with every `.` replaced by `-`, and respond `200`. A caller from `127.0.0.1`
therefore receives `i-127-0-0-1`. That synthesized id names the connection, not
an instance, and SHALL NOT be treated as an identity; the setting exists so an
operator can keep a deployment running while its inventory is completed.

#### Scenario: Bound caller gets its inventory id

- **GIVEN** a valid session token and a caller that `vm-inventory` resolved to
  VM id `100`
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the service responds `200` with the body `i-100`

#### Scenario: Forwarded-for header cannot steer the instance id

- **GIVEN** a valid session token and a caller bound to VM `100`
- **WHEN** it sends `GET /latest/meta-data/instance-id` with
  `X-Forwarded-For: 10.9.9.9`
- **THEN** the service responds `200` with the body `i-100`, and the header has
  no effect

#### Scenario: Forwarded-for header cannot steer the fallback either

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller from `127.0.0.1`
- **WHEN** it sends `GET /latest/meta-data/instance-id` with
  `X-Forwarded-For: 10.9.9.9`
- **THEN** the service responds `200` with the body `i-127-0-0-1`, derived from
  the peer address alone

#### Scenario: Unbound caller under the default configuration

- **GIVEN** a configuration that does not mention `mds.require_vm_identity`, so
  it takes its built-in default of true
- **WHEN** an unbound caller sends `GET /latest/meta-data/instance-id` with a
  valid session token
- **THEN** the response is `404` and no synthesized id is served

## MODIFIED Requirements

### Requirement: Serve An Unsigned Instance Identity Document

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

`instanceId` SHALL be derived exactly as `GET /latest/meta-data/instance-id` is,
so a caller with no bound VM record is refused rather than served a document.
The signature endpoint SHALL be refused on the same terms, so the pair is
available or refused together.

`privateIp` SHALL be the IPv4 address of the connection's peer, derived exactly
as `local-ipv4` is. When the peer has no IPv4 address `privateIp` SHALL be the
empty string; the key SHALL remain present, so the document keeps its fixed key
set. Neither value SHALL be taken from a request header.

`GET /latest/dynamic/instance-identity/signature` SHALL respond `200` with
`Content-Type: text/plain` and the fixed placeholder string
`dGVzdC1zaWduYXR1cmU=`. No signing key is involved and the value is identical
for every caller, so the endpoint SHALL NOT be treated as evidence of the
document's authenticity.

#### Scenario: Guest reads its identity document

- **GIVEN** a valid session token and a caller from `10.0.0.5`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the service responds `200` with `Content-Type: application/json`, and
  the document reports `imageId` `proxmox-unknown`, `instanceType` `vm`,
  `region` `local`, `availabilityZone` `proxmox` and `privateIp` `10.0.0.5`

#### Scenario: Identity document for a peer with no IPv4 address

- **GIVEN** a valid session token and a caller whose peer address is
  `[fe80::1]:5000`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the response is `200`, the `privateIp` key is present with the empty
  string as its value, and it is in particular not `[fe80`

#### Scenario: Identity signature is a constant placeholder

- **GIVEN** a valid session token
- **WHEN** any guest sends `GET /latest/dynamic/instance-identity/signature`
- **THEN** the service responds `200` with the body `dGVzdC1zaWduYXR1cmU=`

## REMOVED Requirements

### Requirement: Derive The Instance Id From Inventory Or The Caller Address

**Reason**: Its second and third paragraphs, and its scenarios
`Unidentified caller is served a synthesized id` and
`Forwarded-for header steers the synthesized id`, state the defect as the
contract: that an unidentified caller is served an id rather than refused, and
that it selects that id through `X-Forwarded-For`. The replacement inverts both
outcomes, so those scenarios cannot be carried forward. Replaced by
`Derive The Instance Id From The Bound VM Record`, which keeps the `i-<vm id>`
rule verbatim.

**Migration**: A deployment whose guests are all in the inventory needs no
change. One that relies on the synthesized id must either complete its
inventory or set `mds.require_vm_identity: false`, and in the latter case the
synthesized id is derived from the peer address, so anything that supplied
`X-Forwarded-For` to select an id stops working either way.
