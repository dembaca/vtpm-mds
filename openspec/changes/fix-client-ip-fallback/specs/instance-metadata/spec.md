# Spec Delta

## MODIFIED Requirements

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

When no VM record is bound, `mds.require_vm_identity` is false and the peer of
the connection has no IPv4 address — its address cannot be parsed, or it is an
IPv6 address that is not IPv4-mapped — the service SHALL NOT synthesize an id
and SHALL NOT substitute any address for the one it does not have. It SHALL
respond `422` with an empty body and `Content-Type: text/plain`.

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

#### Scenario: Unparseable peer is not given an invented address

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller whose `RemoteAddr` is `not-an-address`
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the service responds `422` with an empty body, and in particular
  never responds `200` with the body `i-127-0-0-1`

#### Scenario: IPv6 peer is not given an id with colons in it

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller whose peer address is `[fe80::1]:5000`
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the service responds `422` with an empty body, and in particular
  never responds `200` with the body `i-fe80::1`

#### Scenario: IPv4-mapped peer still gets the fallback id

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller whose peer address is `[::ffff:10.0.0.5]:5000`
- **WHEN** it sends `GET /latest/meta-data/instance-id`
- **THEN** the service responds `200` with the body `i-10-0-0-5`

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

`privateIp` SHALL be the IPv4 address of the connection's peer, derived exactly
as `local-ipv4` is, and SHALL NOT honour `X-Forwarded-For`, even though
`instanceId` does. When the peer has no IPv4 address `privateIp` SHALL be the
empty string; the key SHALL remain present, so the document keeps its fixed key
set.

When no instance id can be derived, as specified by
`Derive The Instance Id From The Bound VM Record`, the service SHALL NOT serve a
document with an empty or invented `instanceId`; it SHALL respond `422` with an
empty body and `Content-Type: text/plain`.

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

#### Scenario: No document for a caller with no derivable instance id

- **GIVEN** `mds.require_vm_identity` is false, a valid session token, and an
  unbound caller whose `RemoteAddr` is `not-an-address`
- **WHEN** it sends `GET /latest/dynamic/instance-identity/document`
- **THEN** the service responds `422` with an empty body

#### Scenario: Identity signature is a constant placeholder

- **GIVEN** a valid session token
- **WHEN** any guest sends `GET /latest/dynamic/instance-identity/signature`
- **THEN** the service responds `200` with the body `dGVzdC1zaWduYXR1cmU=`
