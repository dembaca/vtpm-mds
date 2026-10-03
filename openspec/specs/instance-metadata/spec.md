# Instance Metadata

## Purpose

Give a QEMU or Proxmox guest an EC2-IMDSv2-compatible view of itself over plain
HTTP on the host's link-local metadata address, so cloud-init, the AWS SDKs and
other unmodified guest tooling can discover an instance id, hostname, address
and placement without being taught anything Proxmox-specific. This capability
covers the session token flow, the `/latest/meta-data/` tree, the EC2-compat
instance identity endpoints and the service health check. Mapping a caller to a
VM belongs to the `vm-inventory` capability and TPM-backed credentials belong to
`devid-enrollment`; this capability only consumes the result of the former.

## Requirements

### Requirement: Mint Session Tokens Without Authentication

The service SHALL expose `PUT /latest/api/token` and SHALL register that route
unconditionally, independently of `mds.enable_ec2_compat`.

The handler SHALL NOT authenticate, authorize or rate-limit the caller, and
SHALL NOT require a request body. On success it SHALL respond `200` with
`Content-Type: text/plain` and a body consisting of the standard base64
encoding of 32 bytes drawn from the cryptographic random source, 44 characters
with no trailing newline, and SHALL record the token in the service's
process-wide token store.

The handler SHALL read the request header
`X-aws-ec2-metadata-token-ttl-seconds` and SHALL discard its value: a token's
lifetime is taken from `mds.token_ttl` alone and a caller cannot request a
different one. The response SHALL NOT carry any TTL header, so a client cannot
learn the granted lifetime from the response.

If the random source fails, the handler SHALL respond `500` with the body
`Internal server error` and SHALL NOT record a token.

#### Scenario: Guest opens a metadata session

- **WHEN** a guest sends `PUT /latest/api/token` with no headers and no body
- **THEN** the service responds `200` with `Content-Type: text/plain` and a
  44-character base64 token as the entire body

#### Scenario: Requested token lifetime is ignored

- **WHEN** a guest sends `PUT /latest/api/token` with
  `X-aws-ec2-metadata-token-ttl-seconds: 21600`
- **THEN** the service responds `200` and the returned token still expires after
  `mds.token_ttl`, and the response carries no TTL header

#### Scenario: Token endpoint stays available when EC2 compatibility is off

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `PUT /latest/api/token`
- **THEN** the service responds `200` with a token

#### Scenario: Random source failure

- **WHEN** token generation cannot read from the cryptographic random source
- **THEN** the service responds `500` with the body `Internal server error`

### Requirement: Require A Valid Session Token For Every Metadata Read

Every handler under `/latest/meta-data/` and
`/latest/dynamic/instance-identity/` SHALL require the request header
`X-Aws-Ec2-Metadata-Token` to carry a token that is present in the token store
and not expired. Header name matching SHALL follow ordinary HTTP header
canonicalization, so a client may spell the name in any case.

When the header is absent, carries an empty value, carries a value the store
does not know, carries an expired token, or the token store has not been
initialized, the handler SHALL reject the request identically: status `401`,
response header `X-Aws-Ec2-Metadata-Token: required`, content type
`text/plain; charset=utf-8`, `X-Content-Type-Options: nosniff`, and the body
`IMDSv2 token required` followed by a newline. The rejection SHALL NOT
distinguish a missing token from an expired or forged one.

A token SHALL NOT be bound to the caller that minted it. The store is a single
process-wide map keyed only by the token value, so any token it holds
authenticates any caller. A caller presenting another guest's token is served
its own metadata, because instance data is derived from the connection rather
than from the token.

Tokens SHALL be held in memory only and SHALL NOT survive a service restart: a
restarted service builds an empty store, so every previously issued token is
thereafter unknown.

#### Scenario: Metadata read without a token

- **WHEN** a guest sends `GET /latest/meta-data/instance-id` with no
  `X-Aws-Ec2-Metadata-Token` header
- **THEN** the service responds `401` with header
  `X-Aws-Ec2-Metadata-Token: required` and the body `IMDSv2 token required`

#### Scenario: Metadata read with an unknown token

- **WHEN** a guest sends `GET /latest/meta-data/instance-id` with
  `X-Aws-Ec2-Metadata-Token: invalid-token`
- **THEN** the service responds `401` with the same body and headers as a
  request that carried no token at all

#### Scenario: Header name is matched case-insensitively

- **GIVEN** a token minted by `PUT /latest/api/token`
- **WHEN** a guest presents it as `x-aws-ec2-metadata-token`
- **THEN** the service responds `200` with the metadata value

#### Scenario: A token is accepted from any caller

- **GIVEN** a token minted over one guest's connection
- **WHEN** a different guest presents that same token on
  `GET /latest/meta-data/instance-id`
- **THEN** the service responds `200` and returns the second guest's own
  instance id

#### Scenario: Tokens do not survive a restart

- **GIVEN** a guest holding a token minted before the service was restarted
- **WHEN** it presents that token on a metadata read
- **THEN** the service responds `401`

### Requirement: Serve The EC2 Metadata Tree

With `mds.enable_ec2_compat` enabled, the service SHALL serve the following
paths, each to an authenticated caller and each with `Content-Type: text/plain`
and no trailing newline unless stated otherwise:

- `GET /latest/meta-data/` SHALL return the six-line index `instance-id`,
  `local-hostname`, `local-ipv4`, `public-ipv4`, `placement/`, `services/`, one
  per line and each terminated by a newline.
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
- `GET /latest/meta-data/placement/availability-zone` SHALL return the fixed
  string `proxmox`.
- `GET /latest/meta-data/services/domain` SHALL return the fixed string
  `localdomain`.

`GET /latest/meta-data/public-ipv4` SHALL respond `404` with an empty body and
`Content-Type: text/plain` for every caller, because no source of a public
address is implemented. The path SHALL nonetheless remain listed in the index.

The metadata index SHALL be registered as a subtree, so any path under
`/latest/meta-data/` that is not one of the registered keys SHALL be answered by
the index handler with `200` and the index listing rather than `404`. This
includes the `placement/` and `services/` prefixes that the index advertises.

`GET /latest/meta-data` without the trailing slash SHALL be answered with a
`301` redirect to `/latest/meta-data/`.

#### Scenario: Guest lists the metadata tree

- **GIVEN** a valid session token
- **WHEN** a guest sends `GET /latest/meta-data/`
- **THEN** the service responds `200` with
  `instance-id`, `local-hostname`, `local-ipv4`, `public-ipv4`, `placement/` and
  `services/`, one per line

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

- **GIVEN** a valid session token
- **WHEN** a guest reads `placement/availability-zone` and `services/domain`
- **THEN** the service responds `200` with `proxmox` and `localdomain`
  respectively, regardless of which VM is asking

#### Scenario: Public address is never available

- **GIVEN** a valid session token
- **WHEN** a guest sends `GET /latest/meta-data/public-ipv4`
- **THEN** the service responds `404` with an empty body

#### Scenario: Unknown metadata key returns the index

- **GIVEN** a valid session token
- **WHEN** a guest sends `GET /latest/meta-data/no-such-key`
- **THEN** the service responds `200` with the metadata index listing

#### Scenario: Index path without a trailing slash redirects

- **WHEN** a guest sends `GET /latest/meta-data`
- **THEN** the service responds `301` with `Location: /latest/meta-data/`

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

### Requirement: Answer Health Checks Without A Token

The service SHALL register `GET /health` unconditionally and SHALL answer it
with `200`, `Content-Type: application/json` and the body `{"status":"ok"}`
followed by a newline. The endpoint SHALL NOT require a session token and SHALL
NOT depend on `mds.enable_ec2_compat`, on the VM inventory, or on any TPM
device, so it reports only that the HTTP listener is up.

Requests to `/health` with any other method SHALL fall through to the catch-all
handler and receive `404`.

#### Scenario: Unauthenticated health check

- **WHEN** a caller sends `GET /health` with no session token
- **THEN** the service responds `200` with `Content-Type: application/json` and
  the body `{"status":"ok"}`

#### Scenario: Health check with the wrong method

- **WHEN** a caller sends `POST /health`
- **THEN** the service responds `404` with the body `404 page not found`

### Requirement: Expire Session Tokens On A Validated TTL

The token store SHALL take its lifetime from `mds.token_ttl`, parsed as a Go
duration once when the store is constructed, and SHALL stamp each token with an
expiry of its creation time plus that lifetime. The shipped default is `60s`.

Validation SHALL compare the current time against the stored expiry, so a token
stops being accepted the moment it expires. A background sweep SHALL run every
minute and delete expired entries from the store; that sweep is a memory
reclaim and SHALL NOT be what makes an expired token invalid.

A `mds.token_ttl` that is present but does not parse as a Go duration SHALL be
a fatal configuration error. Loading the configuration SHALL fail, the error
SHALL name the setting and the offending value, and the service SHALL NOT
start. The service SHALL NOT start with a zero token lifetime.

A configuration file that omits `token_ttl` SHALL take the built-in default,
so omission is not a way to reach a zero lifetime either.

#### Scenario: Token is accepted inside its lifetime

- **GIVEN** `mds.token_ttl` is `60s`
- **WHEN** a guest presents a token it minted a moment earlier
- **THEN** the service responds `200` with the metadata value

#### Scenario: Token is refused after its lifetime

- **GIVEN** `mds.token_ttl` is `1ms`
- **WHEN** a guest presents a token 10 milliseconds after minting it
- **THEN** the token no longer validates and the metadata read responds `401`

#### Scenario: Unparseable TTL stops the service from starting

- **GIVEN** a configuration file whose `mds.token_ttl` is a value such as `60`
  or `sixty` that is not a Go duration
- **WHEN** the service is started with that file
- **THEN** it exits non-zero with an error naming `token_ttl` and the offending
  value, and never binds its listen address

#### Scenario: Omitted TTL takes the built-in default

- **GIVEN** a configuration file that does not mention `mds.token_ttl`
- **WHEN** a guest mints a token and immediately presents it on a metadata read
- **THEN** the read responds `200`, because the store was built with the
  built-in `60s` lifetime

### Requirement: Gate The EC2 Metadata Surface On The Effective Configuration

The service SHALL register the `/latest/meta-data/` tree and the
`/latest/dynamic/instance-identity/` endpoints only when
`mds.enable_ec2_compat` is true. When it is false those routes SHALL NOT be
registered, and requests for them SHALL fall through to the catch-all handler,
which SHALL log the unmatched route and respond `404` with the body
`404 page not found`.

Disabling EC2 compatibility SHALL NOT affect `PUT /latest/api/token` or
`GET /health`, both of which are registered unconditionally.

Configuration loaded from a file SHALL start from the service's built-in
defaults and overlay the file's values on top. A setting the file does not
mention SHALL keep its built-in default; a setting the file states explicitly
SHALL win, including one stated as the zero value. A configuration file that
omits `enable_ec2_compat` SHALL therefore leave the metadata tree registered,
because the built-in default is true, and only an explicit
`enable_ec2_compat: false` SHALL disable it.

`listen_addr` SHALL continue to be validated as non-empty. Because it now has
a built-in default, that validation SHALL reject only a value the file states
explicitly as empty.

#### Scenario: Metadata tree is absent when compatibility is off

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `GET /latest/meta-data/instance-id` with a valid token
- **THEN** the service responds `404` with the body `404 page not found`

#### Scenario: Token and health endpoints survive the gate

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `PUT /latest/api/token` and `GET /health`
- **THEN** both respond `200`

#### Scenario: Omitting the setting keeps the tree registered

- **GIVEN** a configuration file that sets `listen_addr` but does not mention
  `enable_ec2_compat`
- **WHEN** the service is started with that file
- **THEN** the metadata tree is registered and a metadata read with a valid
  token responds `200`

#### Scenario: An explicit false still disables the tree

- **GIVEN** a configuration file that sets `enable_ec2_compat: false`
- **WHEN** the service is started with that file
- **THEN** the metadata tree is not registered and metadata reads respond `404`

#### Scenario: An explicitly empty listen address is rejected

- **GIVEN** a configuration file that sets `listen_addr: ""`
- **WHEN** the service is started with that file
- **THEN** loading fails with the `listen_addr is required` error and the
  service does not start

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
