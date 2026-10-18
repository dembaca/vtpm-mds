# Spec Delta

## MODIFIED Requirements

### Requirement: Mint Session Tokens Without Authentication

The service SHALL expose `PUT /latest/api/token` and SHALL register that route
unconditionally, independently of `mds.enable_ec2_compat`.

The handler SHALL NOT authenticate or authorize the caller and SHALL NOT
require a request body. The handler MAY refuse a caller that has exceeded the
per-peer token-mint limit or that would grow the token store past its capacity
(see "Limit Guests Per Peer" and "Bound The Session Token Store"); such a
refusal SHALL be the `429` answer of those requirements and SHALL NOT record a
token.

On success it SHALL respond `200` with `Content-Type: text/plain` and a body
consisting of the standard base64 encoding of 32 bytes drawn from the
cryptographic random source, 44 characters with no trailing newline, and SHALL
record the token in the service's process-wide token store.

The handler SHALL read the request header
`X-aws-ec2-metadata-token-ttl-seconds` and SHALL discard its value: a token's
lifetime is taken from `mds.token_ttl` alone and a caller cannot request a
different one. The response SHALL NOT carry any TTL header, so a client cannot
learn the granted lifetime from the response.

If the random source fails, the handler SHALL respond `500` with the body
`Internal server error` and SHALL NOT record a token.

#### Scenario: Guest opens a metadata session

- **WHEN** a guest sends `PUT /latest/api/token` with no headers and no body
  and is under the per-peer token-mint limit with room in the token store
- **THEN** the service responds `200` with `Content-Type: text/plain` and a
  44-character base64 token as the entire body

#### Scenario: Requested token lifetime is ignored

- **WHEN** a guest sends `PUT /latest/api/token` with
  `X-aws-ec2-metadata-token-ttl-seconds: 21600`
  and is under the per-peer token-mint limit with room in the token store
- **THEN** the service responds `200` and the returned token still expires after
  `mds.token_ttl`, and the response carries no TTL header

#### Scenario: Token endpoint stays available when EC2 compatibility is off

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `PUT /latest/api/token` under the per-peer token-mint
  limit with room in the token store
- **THEN** the service responds `200` with a token

#### Scenario: Random source failure

- **WHEN** token generation cannot read from the cryptographic random source
- **THEN** the service responds `500` with the body `Internal server error`

#### Scenario: Token mint refused when the peer is over the mint limit

- **WHEN** a guest whose peer has already used the per-peer token-mint budget
  for the current window sends `PUT /latest/api/token`
- **THEN** the service responds `429` with the body `Too many requests`
  followed by a newline, does not record a token, and does not answer `200`

## ADDED Requirements

### Requirement: Limit Guests Per Peer

The service SHALL apply per-peer limits to every guest-facing HTTP request
except `GET /health`. The peer key SHALL be the IP address parsed from the
request's `RemoteAddr` by the same rules as metadata peer parsing; when that
parse yields no IP, the peer key SHALL be a single shared bucket for
unparseable addresses.

Within each rolling or fixed window of 60 seconds the service SHALL allow at
most 30 successful or attempted `PUT /latest/api/token` requests per peer and
at most 120 HTTP requests per peer on the limited surface. A request that
exceeds either budget SHALL be refused with status `429`,
`Content-Type: text/plain`, body `Too many requests` followed by a newline, and
header `Retry-After` set to a positive integer number of seconds. The refusal
SHALL NOT run the route handler.

The general request budget SHALL count token mints and every other limited
path. The token-mint budget SHALL count only `PUT /latest/api/token`.

#### Scenario: Sustained metadata flood from one peer is refused

- **WHEN** a single peer sends more than 120 limited HTTP requests within 60
  seconds
- **THEN** further limited requests from that peer in the window receive `429`
  with body `Too many requests` and a `Retry-After` header

#### Scenario: Token mint flood from one peer is refused

- **WHEN** a single peer sends more than 30 `PUT /latest/api/token` requests
  within 60 seconds
- **THEN** further token mints from that peer in the window receive `429` and
  no new token is recorded

#### Scenario: A second peer is unaffected

- **GIVEN** peer A has exhausted its request budget for the window
- **WHEN** peer B, under budget, sends `GET /latest/meta-data/instance-id`
  with a valid session token
- **THEN** the service serves peer B normally and does not answer `429`

#### Scenario: Health stays unlimited

- **GIVEN** a peer that has exhausted its request budget for the window
- **WHEN** that peer sends `GET /health`
- **THEN** the service responds `200` and does not apply the guest throttle

### Requirement: Bound The Session Token Store

The process-wide session token store SHALL hold at most 4096 tokens at once.
Before recording a newly minted token, the service SHALL discard expired
entries it still holds. If the store is at capacity after that discard, the
mint SHALL fail: the token handler SHALL respond `429` with the same body and
`Retry-After` header as "Limit Guests Per Peer", and SHALL NOT record a token.

Tokens SHALL remain unbound to the minting peer, held in memory only, and
invalid after restart, as required by "Require A Valid Session Token For Every
Metadata Read".

#### Scenario: Mint refused when the store is full

- **GIVEN** the token store already holds 4096 unexpired tokens
- **WHEN** a peer under the per-peer mint limit sends `PUT /latest/api/token`
- **THEN** the service responds `429` with body `Too many requests` and does
  not grow the store

#### Scenario: Expiry frees capacity

- **GIVEN** the token store is at capacity and at least one held token is
  expired
- **WHEN** a peer under the per-peer mint limit sends `PUT /latest/api/token`
- **THEN** the service discards expired entries, records a new token, and
  responds `200`
