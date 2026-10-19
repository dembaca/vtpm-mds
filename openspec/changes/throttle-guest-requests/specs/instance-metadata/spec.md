# Spec Delta

## MODIFIED Requirements

### Requirement: Mint Session Tokens Without Authentication

The service SHALL expose `PUT /latest/api/token` and SHALL register that route
unconditionally, independently of `mds.enable_ec2_compat`.

The handler SHALL NOT authenticate or authorize the caller and SHALL NOT
require a request body. The handler MAY refuse a caller that has exceeded the
per-peer token-mint limit or that would trip the token store's global
emergency brake (see "Limit Guests Per Peer" and "Bound Session Tokens Per
Peer"); such a refusal SHALL be the `429` answer of those requirements and
SHALL NOT record a token. Reaching a peer's live-token quota SHALL NOT refuse
the mint: the store SHALL evict that peer's oldest live token instead.

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
  and is under the per-peer token-mint limit and the global emergency brake
- **THEN** the service responds `200` with `Content-Type: text/plain` and a
  44-character base64 token as the entire body

#### Scenario: Requested token lifetime is ignored

- **WHEN** a guest sends `PUT /latest/api/token` with
  `X-aws-ec2-metadata-token-ttl-seconds: 21600`
  and is under the per-peer token-mint limit and the global emergency brake
- **THEN** the service responds `200` and the returned token still expires after
  `mds.token_ttl`, and the response carries no TTL header

#### Scenario: Token endpoint stays available when EC2 compatibility is off

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `PUT /latest/api/token` under the per-peer token-mint
  limit and the global emergency brake
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

### Requirement: Bound Session Tokens Per Peer

The session token store SHALL associate each recorded token with the peer IP
that minted it, for quota and eviction only. Validation of a presented token
SHALL remain independent of the caller's peer, as required by "Require A Valid
Session Token For Every Metadata Read".

Each peer SHALL hold at most 32 unexpired tokens. Before recording a new token
for a peer, the service SHALL discard that peer's expired entries. If the peer
still holds 32 live tokens, the service SHALL delete the oldest of those
(by creation time) and then record the new token. Eviction SHALL NOT delete
another peer's tokens. The mint SHALL respond `200` with the new token.

The store SHALL also enforce a global emergency capacity of 65536 tokens. If
after per-peer expiry discard and per-peer eviction the store would still
exceed that capacity, the mint SHALL fail: the token handler SHALL respond
`429` with the same body and `Retry-After` header as "Limit Guests Per Peer",
SHALL NOT record a token, and the service SHALL emit one log line for the
event. This global brake is a last resort; without bridge-port binding of
callers it can still be approached by a guest that spoofs many peer IPs.

Tokens SHALL be held in memory only and SHALL NOT survive a service restart.

#### Scenario: Peer over the live-token quota keeps minting by eviction

- **GIVEN** a peer already holds 32 unexpired tokens
- **WHEN** that peer, under the per-peer mint rate limit, sends
  `PUT /latest/api/token`
- **THEN** the service deletes the oldest of that peer's tokens, records the
  new token, responds `200`, and does not delete any other peer's tokens

#### Scenario: Another peer's tokens are untouched

- **GIVEN** peer A holds 32 unexpired tokens and peer B holds at least one
- **WHEN** peer A mints another token
- **THEN** peer B's tokens remain in the store and peer A's count of live
  tokens stays 32 including the new one

#### Scenario: Global emergency brake refuses the mint

- **GIVEN** the store already holds 65536 unexpired tokens and per-peer
  eviction cannot bring it under that capacity
- **WHEN** a peer under the per-peer mint rate limit sends
  `PUT /latest/api/token`
- **THEN** the service responds `429` with body `Too many requests`, does not
  record a token, and logs the emergency brake once for the event
