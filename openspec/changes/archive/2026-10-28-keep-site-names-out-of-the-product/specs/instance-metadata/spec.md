## MODIFIED Requirements

### Requirement: Serve The EC2 Metadata Tree With A Caller-Dependent Index

With `mds.enable_ec2_compat` enabled, the service SHALL serve the following
paths, each to an authenticated caller and each with `Content-Type: text/plain`
and no trailing newline unless stated otherwise:
- `GET /latest/meta-data/` SHALL return an index listing one key per line, each
  terminated by a newline. It SHALL always list `instance-id`,
  `local-hostname`, `local-ipv4`, `placement/` and `services/`. It SHALL list
  `public-ipv4` if and only if the service has a public address to serve the
  asking caller, as defined below, and `tags/` if and only if the asking
  caller's VM record carries at least one instance tag, as
  `Serve Instance Tags From The Bound VM Record` defines. The index is therefore
  a metadata value that depends on which VM is asking: a caller with a declared
  public address and tags receives seven lines, one with neither receives five. The index SHALL NOT be
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
- `GET /latest/meta-data/placement/<key>` SHALL return the value of that key of the
  host's topology, with no trailing newline, as described below.
- `GET /latest/meta-data/services/domain` SHALL return the fixed string
  `localdomain`.
Every key of the host's topology, as `vm-inventory` defines it, SHALL be served
under `placement/` to every caller, whichever VM is asking and also to a caller with
no bound VM record: `availability-zone` and `region` always, `node` and `cluster`
when the host knows them, and every other key when the node's description declares
it. A key SHALL be served with the value and the case its source gives it and SHALL
NOT be derived from the connection or from any request header. A key the topology
does not have, `rack` on a node whose description declares none for example, SHALL
be answered `404` with an empty body and `Content-Type: text/plain`, distinct from
the catch-all handler's `404 page not found`, and SHALL NOT be answered by the
metadata index; this applies to any path `placement/<key>`. A key of the topology
MAY be one that EC2 serves under `placement/` with another meaning, such as
`group-name` or `host-id`: a node's description, which only an administrator with
`Sys.Modify` on `/` can write, decides what the guests of that node read, and the
service SHALL NOT keep a list of keys it refuses.

`GET /latest/meta-data/placement` without the trailing slash SHALL be answered with
a `301` redirect to `/latest/meta-data/placement/`, without a token, as the
directory is a path of its own. Keys SHALL be case-sensitive: `Region` is not
`region`.

`GET /latest/meta-data/placement/` SHALL return the keys of the host's topology,
one per line in ascending byte order, each terminated by a newline. It was answered
by the metadata index before this requirement, as every unregistered path under
`/latest/meta-data/` is.

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

- **GIVEN** a valid session token, a host whose node description declares zone
  `AZ4` and region `region-a`, and a caller bound to any VM record
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with `AZ4` and `region-a` respectively, with no
  trailing newline

#### Scenario: Nothing declared gives the host's placement

- **GIVEN** a valid session token and a caller bound to a VM record, on a host
  whose description declares neither a zone nor a region, whose zone is `node-a`
  and whose region is `cluster-1`
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with `node-a` and `cluster-1`

#### Scenario: One value declared, the other taken from the host

- **GIVEN** a caller bound to a VM record, on a host whose description declares a
  zone `AZ4` and no region, and whose cluster is named `cluster-1`
- **WHEN** it reads both placement keys
- **THEN** the zone is `AZ4` and the region is `cluster-1`

#### Scenario: An unbound caller is served the host's placement

- **GIVEN** a valid session token and a caller with no bound VM record
- **WHEN** it reads `placement/availability-zone` and `placement/region`
- **THEN** the service responds `200` with the host's zone and region, the same
  values a bound caller reads

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

#### Scenario: A VM-level placement declaration is not served

- **GIVEN** a caller bound to a VM record whose Proxmox tags include
  `mds.zone.rack-b` and whose YAML entry once declared `zone: rack-b`
- **WHEN** it reads `placement/availability-zone`
- **THEN** the service responds with the host's zone and not `rack-b`

#### Scenario: Index lists tags when the VM has some

- **GIVEN** a valid session token and a caller bound to a VM record carrying one
  instance tag
- **WHEN** it sends `GET /latest/meta-data/`
- **THEN** the listing contains `tags/`, and for a caller whose VM carries none it
  does not

#### Scenario: A key of the node description is served

- **GIVEN** a valid session token, a host whose node description declares
  `topology.vtpm_mds.io/rack: rack-01`, and a caller bound to any VM record
- **WHEN** it reads `placement/rack`
- **THEN** the service responds `200` with `rack-01`, with no trailing newline

#### Scenario: Node and cluster are served by default

- **GIVEN** a host on node `node-a` in cluster `cluster-1` whose description
  declares neither
- **WHEN** a caller reads `placement/node` and `placement/cluster`
- **THEN** the service responds `200` with `node-a` and `cluster-1`

#### Scenario: A key with no value is not the index

- **GIVEN** a host whose topology has no `rack`
- **WHEN** a caller reads `placement/rack`
- **THEN** the service responds `404` with an empty body, and in particular not
  `200` with the metadata index

#### Scenario: An EC2 key can be overridden by the node

- **GIVEN** a host whose description declares `topology.vtpm_mds.io/host-id: h-01`
- **WHEN** a caller reads `placement/host-id`
- **THEN** the service responds `200` with `h-01`

#### Scenario: The placement directory lists the keys

- **GIVEN** a host whose topology has `availability-zone`, `region`, `node`,
  `cluster` and `rack`
- **WHEN** a caller reads `placement/`
- **THEN** the service responds `200` with the five lines `availability-zone`,
  `cluster`, `node`, `rack` and `region`, each terminated by a newline

#### Scenario: The placement path without a slash redirects

- **WHEN** a guest sends `GET /latest/meta-data/placement`
- **THEN** the service responds `301` with `Location: /latest/meta-data/placement/`

#### Scenario: Keys are case-sensitive

- **GIVEN** a host whose topology has `region`
- **WHEN** a caller reads `placement/Region`
- **THEN** the service responds `404` with an empty body

#### Scenario: An unbound caller reads the same topology

- **GIVEN** a valid session token and a caller with no bound VM record
- **WHEN** it reads `placement/rack` on a host that declares one
- **THEN** the service responds `200` with the host's value

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

- **GIVEN** a host whose node description declares zone `AZ4` and region `region-a`,
  and a caller bound to any VM record
- **WHEN** it reads the identity document
- **THEN** `region` is `region-a` and `availabilityZone` is `AZ4`, as the placement
  keys serve them

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

