# Proposal

## Why

A single guest can flood the metadata service today: `PUT /latest/api/token`
is explicitly not rate-limited, the in-memory token map grows without bound,
and neither request rate, concurrent connections nor DevID enroll have a
per-peer ceiling. Hogan's production-readiness pass (2026-10-18, point 8) and
the maintainer's ask for throttling against DoS from a VM make that a P2 gap
before the lab setup is treated as production-ready. Without limits, a
compromised or buggy guest can exhaust memory and crowd out every other VM
on the IMDS bridge.

## What Changes

- **Per-peer limits** on the guest-facing HTTP surface (keyed by peer IP from
  `RemoteAddr` via the existing `PeerIP` helper): token mint rate, general
  request rate, concurrent connections, and DevID enroll rate. A peer over a
  limit gets `429` with a short plain body; `/health` stays unlimited.
- **Bound the token map.** When the process-wide store is at capacity, a new
  mint is refused with `429` instead of growing forever. Expired tokens are
  still reaped as today.
- **One middleware line in `server.go`.** The limiter lives in a new file
  (`internal/server/limit.go` or `imds/limit.go`); `imds/token.go` learns the
  map capacity. Concurrent-connection accounting uses `http.Server.ConnState`.
- **Lab verification** with a flooding guest on the QEMU lab, after the
  coordinator's ACK of this record. Code rebases behind Mac's log-change
  merge when that lands, so `server.go` conflict surface stays one line.
- **Not in scope:** binding tokens to the minting peer, changing `mds.token_ttl`
  or honouring the client TTL header, MAC/IP anti-spoofing on `vmbr_imds`
  (separate changes), and configurable limit knobs in YAML unless the
  maintainer asks for them in review (defaults are constants in this record).

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `instance-metadata`: the token-mint requirement stops forbidding rate
  limits; new requirements cover per-peer token and request limits, the
  token-map capacity, and the `429` answer shape.
- `operability`: new requirement for a per-peer concurrent-connection limit
  and for `/health` remaining exempt from guest throttles.
- `devid-enrollment`: enroll start/finish are subject to the same per-peer
  enroll limit and answer `429` when it is exceeded.

## Impact

- **Code, after the coordinator's ACK:** new `imds/limit.go` or
  `internal/server/limit.go`, `imds/token.go` (capacity), `internal/server/server.go`
  (one middleware wrap + `ConnState` hook), tests. No `debian/` and no
  inventory change.
- **Behaviour change visible to guests:** a flooding peer starts seeing
  `429` instead of unbounded `200`/`401` traffic; a well-behaved cloud-init
  or SPIRE enroll path stays under the defaults.
- **Operators:** journal should show a single rate-limit line per peer when
  a limit trips (exact log shape follows Mac's quiet-log change once rebased).
- **Not in this PR's code half until ACK:** nothing lands before the
  coordinator ACKs the record on the bus.
