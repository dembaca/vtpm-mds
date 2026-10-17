# Spec Delta

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
`placement/availability-zone` and `placement/region` carry the host's own
placement, which an unbound caller cannot steer, `services/domain` is a fixed
string, and
`public-ipv4` is unavailable to an unbound caller for the same reason it is
unavailable to any caller with no declared address — a public address is read
from the bound VM record, and an unbound caller has none. The index asserts no
identity either: it is derived per request, and for an unbound caller it lists
the five keys that are always listed and omits `public-ipv4`.

An unbound caller SHALL continue to be served all of them, and the index SHALL
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

### Requirement: Serve The EC2 Metadata Tree With A Caller-Dependent Index

With `mds.enable_ec2_compat` enabled, the service SHALL serve the following
paths, each to an authenticated caller and each with `Content-Type: text/plain`
and no trailing newline unless stated otherwise:
- `GET /latest/meta-data/` SHALL return an index listing one key per line, each
  terminated by a newline. It SHALL always list `instance-id`,
  `local-hostname`, `local-ipv4`, `placement/` and `services/`. It SHALL list
  `public-ipv4` if and only if the service has a public address to serve the
  asking caller, as defined below. The index is therefore the first metadata
  value that depends on which VM is asking: a caller with a declared public
  address receives six lines, one without receives five. The index SHALL NOT be
  cached across callers.
- `GET /latest/meta-data/local-hostname` SHALL return the request's `Host`
  header truncated at the first colon, or `localhost.localdomain` when `Host` is
  empty. It SHALL NOT return the guest's own hostname, which the service does
  not know.
- `GET /latest/meta-data/local-ipv4` SHALL return the IPv4 address of the
  connection's peer. The peer address SHALL be parsed as a host and port pair
  rather than truncated at the first colon, so a caller connecting from
  `[fe80::1]:5000` is not served a fragment of its own address. A peer whose
  address is an IPv4-mapped IPv6 address SHALL be served the IPv4 form. When
  the peer has no IPv4 address, the service SHALL respond `404` with an empty
  body and `Content-Type: text/plain`, as it does for `public-ipv4`.
- `GET /latest/meta-data/placement/availability-zone` SHALL return the zone of
  the asking caller, and `GET /latest/meta-data/placement/region` its region,
  as defined below. Neither is ever empty.
- `GET /latest/meta-data/services/domain` SHALL return the fixed string
  `localdomain`.
The zone of a caller SHALL be the zone declared for its bound VM record when one
is declared, and otherwise the host's zone. Its region SHALL be the region
declared for its bound VM record when one is declared, and otherwise the host's
region. The two are resolved independently, so a VM may declare one and take the
host's value for the other. The host's zone and region are those of the
inventory backend in use, as `vm-inventory` defines them. A caller with no bound
VM record has nothing declared and SHALL be served the host's zone and region.
The values SHALL NOT be derived from the connection or from any request header.
`placement/region` was not a registered key before this requirement; it was
answered by the index handler like any unknown key under `/latest/meta-data/`.

`GET /latest/meta-data/public-ipv4` SHALL return the public IPv4 address
declared for the caller's bound VM record, when one is declared. The service
SHALL NOT derive the value from the connection, from any request header, or
from any address it observes: a public address is not something this service
can observe, only something an operator can declare, and the peer address of
the connection is by construction the guest's address on the metadata bridge.

When no public address is declared for the caller, or when no VM record is
bound to the connection, the service SHALL respond `404` with an empty body and
`Content-Type: text/plain`. That response SHALL stay distinct from the
catch-all handler's `404 page not found`, which carries a body.

The metadata index SHALL be registered as a subtree, so any path under
`/latest/meta-data/` that is not one of the registered keys SHALL be answered by
the index handler with `200` and the index listing rather than `404`. This
includes the `placement/` and `services/` prefixes that the index advertises.

`GET /latest/meta-data` without the trailing slash SHALL be answered with a
`301` redirect to `/latest/meta-data/`.

#### Scenario: Index omits the public address when none is declared

- **GIVEN** a valid session token and a caller bound to a VM with no declared
  public address
- **WHEN** it sends `GET /latest/meta-data/`
- **THEN** the service responds `200` with the five lines `instance-id`,
  `local-hostname`, `local-ipv4`, `placement/` and `services/`, and
  `public-ipv4` is absent

#### Scenario: Index lists the public address when one is declared

- **GIVEN** a valid session token and a caller bound to a VM whose declared
  public address is `203.0.113.7`
- **WHEN** it sends `GET /latest/meta-data/`
- **THEN** the service responds `200` and the listing contains `public-ipv4`

#### Scenario: Local address reflects the connection

- **GIVEN** a valid session token and a guest connecting from `10.0.0.5`
- **WHEN** it sends `GET /latest/meta-data/local-ipv4`
- **THEN** the service responds `200` with the body `10.0.0.5`

#### Scenario: IPv6 peer is not served a fragment of its address

- **GIVEN** a valid session token and a guest whose peer address is
  `[fe80::1]:5000`
- **WHEN** it sends `GET /latest/meta-data/local-ipv4`
- **THEN** the service responds `404` with an empty body, and in particular
  never responds `200` with the body `[fe80`

#### Scenario: IPv4-mapped peer is served its IPv4 address

- **GIVEN** a valid session token and a guest whose peer address is
  `[::ffff:10.0.0.5]:5000`
- **WHEN** it sends `GET /latest/meta-data/local-ipv4`
- **THEN** the service responds `200` with the body `10.0.0.5`

#### Scenario: Placement and domain are fixed strings

- **GIVEN** a valid session token and a YAML inventory in which no entry
  declares a zone or a region
- **WHEN** a guest reads `placement/availability-zone` and `services/domain`
- **THEN** the service responds `200` with `proxmox` and `localdomain`
  respectively, regardless of which VM is asking

#### Scenario: A declared zone and region are served

- **GIVEN** a valid session token and a caller bound to a VM record that
  declares zone `rack-b` and region `bgl-1`
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with `rack-b` and `bgl-1` respectively, with
  no trailing newline

#### Scenario: Nothing declared gives the host's placement

- **GIVEN** a valid session token and a caller bound to a VM record that
  declares neither a zone nor a region, on a host whose zone is `hogan` and
  whose region is `bgl-cluster`
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with `hogan` and `bgl-cluster`

#### Scenario: One value declared, the other taken from the host

- **GIVEN** a caller bound to a VM record that declares zone `rack-b` and no
  region, on a host whose region is `bgl-cluster`
- **WHEN** it reads both placement keys
- **THEN** the zone is `rack-b` and the region is `bgl-cluster`

#### Scenario: An unbound caller is served the host's placement

- **GIVEN** a valid session token and a caller with no bound VM record
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with the host's zone and region, and no VM's
  declaration is visible to it

#### Scenario: Public address is served when declared

- **GIVEN** a valid session token and a caller bound to a VM whose declared
  public address is `203.0.113.7`
- **WHEN** it sends `GET /latest/meta-data/public-ipv4`
- **THEN** the service responds `200` with the body `203.0.113.7`

#### Scenario: Public address is unavailable when none is declared

- **GIVEN** a valid session token and a caller bound to a VM with no declared
  public address
- **WHEN** it sends `GET /latest/meta-data/public-ipv4`
- **THEN** the service responds `404` with an empty body, and in particular
  never serves the caller's own peer address

#### Scenario: Unknown metadata key returns the index

- **GIVEN** a valid session token
- **WHEN** a guest sends `GET /latest/meta-data/no-such-key`
- **THEN** the service responds `200` with the metadata index listing

#### Scenario: Index path without a trailing slash redirects

- **WHEN** a guest sends `GET /latest/meta-data`
- **THEN** the service responds `301` with `Location: /latest/meta-data/`

### Requirement: Serve The Instance Identity Document Without A Signature

With `mds.enable_ec2_compat` enabled,
`GET /latest/dynamic/instance-identity/document` SHALL respond `200` with
`Content-Type: application/json` and a JSON object carrying exactly the keys
`instanceId`, `imageId`, `instanceType`, `region`, `availabilityZone`,
`privateIp`, `devpayProductCodes`, `version`, `billingProducts` and `accountId`.

`imageId` SHALL be `proxmox-unknown`, `instanceType` SHALL be `vm`, `version`
SHALL be `2017-09-30`, `accountId` SHALL be `012345678901`, and
`devpayProductCodes` and `billingProducts` SHALL both be `null`. `region` and
`availabilityZone` SHALL be the caller's region and zone exactly as
`placement/region` and `placement/availability-zone` serve them. `instanceId`,
`privateIp`, `region` and `availabilityZone` vary by caller.

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
  `region` and `availabilityZone` equal to what `placement/region` and
  `placement/availability-zone` return to the same caller, and `privateIp`
  `10.0.0.5`; with a YAML inventory that declares neither, that is `local` and
  `proxmox`

#### Scenario: The document and the placement keys agree

- **GIVEN** a caller bound to a VM record that declares zone `rack-b` and region
  `bgl-1`
- **WHEN** it reads the identity document
- **THEN** `region` is `bgl-1` and `availabilityZone` is `rack-b`

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
