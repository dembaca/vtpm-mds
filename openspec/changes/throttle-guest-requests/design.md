# Design

## Context

See `proposal.md` for why. Observed on `main@407bfca` (2026-10-18):

- `imds/handlers.go` `HandleCreateToken` and the
  `Mint Session Tokens Without Authentication` requirement forbid rate
  limits on `PUT /latest/api/token`.
- `imds/token.go` `TokenStore` is an unbounded `map[string]*Token` with a
  one-minute expiry sweep and no capacity check in `GenerateToken`.
- `internal/server/server.go` registers routes on a bare `ServeMux`, with
  `ConnContext` for inventory binding and a `connectionMap` that Mac's
  `drop-the-connection-map` will remove; there is no `ConnState` hook and
  no HTTP middleware around the mux.
- Peer identity for inventory already goes through `imds.PeerIP(RemoteAddr)`.
- DevID enroll is `POST /latest/devid/enroll/{start,finish}` on the same
  listener.

The coordinator assigned this change with a hard file split: new limiter
file, `imds/token.go`, and **one** middleware line in `server.go`, rebased
after Mac's log-change merge.

## Goals / Non-Goals

**Goals:**

- Cap what one peer IP can do to token mint, general HTTP, enroll, and
  concurrent TCP connections, with a stable `429` answer.
- Cap the process-wide token map so memory cannot grow with mint floods.
- Keep a quiet, correct cloud-init + SPIRE enroll path under the defaults.
- Leave `/health` usable for probes under load from other peers.

**Non-Goals:**

- Binding a session token to the peer that minted it (spec today says the
  opposite; a separate change if wanted).
- Honouring `X-aws-ec2-metadata-token-ttl-seconds` or changing the default
  TTL (Hogan point 9).
- Bridge-level MAC/IP anti-spoofing (Hogan point 10 / separate changes).
- YAML or flag knobs for the limits in this change (constants; see D-f).
- Fixing the `connectionMap` leak (Mac's `drop-the-connection-map`).
- Quoting or shaping the access log (Mac's `quiet-the-log-and-record-issuance`).

## Decisions

### Decided (assignment + maintainer ask, 2026-10-18)

| Said | Decision |
|---|---|
| Throttling against DoS from a VM | Per-peer limits; answer `429`. |
| Limits named | Token-PUT, requests/s, concurrent connections, enroll; token-map size. |
| File split | New `limit.go`, `imds/token.go`, one middleware line in `server.go`. |
| Order vs Mac | Record now; code after ACK; rebase behind Mac's log-change merge. |

### Decided by this record's author, for the maintainer to overrule

- **D-a. Peer key.** The limiter keys on the string form of `imds.PeerIP(r.RemoteAddr)`.
  An unparseable `RemoteAddr` shares one bucket `"unknown"`, so a flood of
  garbage addresses cannot open unbounded buckets. Alternatives considered:
  VMID (fails for the unbound token endpoint and for pre-ARP peers), or
  raw `RemoteAddr` including port (each connection would be its own peer).
- **D-b. Where the code lives.** `internal/server/limit.go` owns the
  middleware, the per-peer counters, and `ConnState` accounting.
  `imds/token.go` only enforces the map capacity inside `GenerateToken`
  (returns a distinct error the token handler maps to `429`). Alternative:
  everything under `imds/` — rejected because concurrent connections are
  an `http.Server` concern and enroll is outside `imds`.
- **D-c. Default numbers** (constants, not config):

  | Limit | Default | Rationale |
  |---|---|---|
  | Token PUTs per peer | 30 / 60 s | cloud-init retries; blocks mint floods |
  | HTTP requests per peer | 120 / 60 s | metadata tree walks; blocks sustained flood |
  | Concurrent connections per peer | 16 | parallel SDK clients; blocks conn exhaustion |
  | Enroll start+finish per peer | 6 / 60 s | one enroll is two POSTs; allows a retry |
  | Token map capacity | 4096 | ~hours of normal mint at lab scale |

  Sliding window of 60 s for the rate buckets (fixed window also fine;
  implementer's choice, tested). Numbers are mine; the maintainer can
  tighten them in review without a new change if only the constants move.
- **D-d. What counts as a request.** Every HTTP request that reaches the
  middleware counts toward the general request limit, including token PUT
  and enroll. Token PUT and enroll additionally count toward their own
  buckets. A peer may therefore trip the general limit without tripping
  the token limit, and vice versa.
- **D-e. `/health` is exempt.** Registered outside the wrapped mux (or
  short-circuited first in the middleware) so probes and Hogan's readiness
  work do not compete with a flooding guest. Unmatched routes stay inside
  the wrapper so a flood of unknown paths is still limited.
- **D-f. No config keys in this change.** Limits are package constants.
  Exposing them under `mds.*` is a small follow-up if operations need to
  tune without a rebuild. Alternative: YAML now — rejected to keep the
  file set and the ACK surface small.
- **D-g. `429` shape.** Status `429`, `Content-Type: text/plain`, body
  `Too many requests` with a trailing newline, header
  `Retry-After: 1` (seconds). Same for every limit. No JSON, no which-limit
  detail to the guest (that belongs in the log). Distinct from `401`
  (token) and `404` (unbound identity).
- **D-h. Token map full.** `GenerateToken` refuses when `len(tokens) >= cap`
  after a best-effort sweep of expired entries; the handler answers `429`
  with the same body as D-g. No eviction of live tokens: a full map means
  wait for expiry or restart.
- **D-i. Concurrent connections.** Track in `http.Server.ConnState` for
  `StateNew` / `StateClosed` / `StateHijacked` per peer IP of the
  `net.Conn`. When at the cap, the middleware rejects new HTTP requests
  on that peer with `429` (the TCP accept still happens; Go does not give
  a clean pre-accept filter without a custom listener). Alternative:
  close the conn from `ConnState` — harsher and harder to test; rejected.
- **D-j. Logging.** One log line when a peer first trips a limit in a
  window (`rate limit: peer=%s kind=%s`), not one line per refused
  request. Exact format aligns with Mac's access-log change after rebase.
- **D-k. Rebase rule.** Implementation lands on a branch from `main` after
  Mac's `quiet-the-log-and-record-issuance` (and `drop-the-connection-map`
  if it merges in the same PR) is on `main`, so the single `server.go`
  middleware line does not fight their edits.

## Risks / Trade-offs

- **[Risk] Defaults too tight for a real guest agent** → Mitigation: lab
  measure with cloud-init + a SPIRE enroll before merge; raise constants
  in the same PR if a legitimate path trips `429`.
- **[Risk] Peer IP spoofing on `vmbr_imds`** → Mitigation: out of scope
  here; Hogan's bind-to-bridge-port / firewall work. Throttling still
  bounds damage from one source address.
- **[Risk] Shared `"unknown"` bucket** → Mitigation: only unparseable
  peers; normal TCP guests always parse.
- **[Risk] Token map full blocks everyone, not just the flooder** →
  Mitigation: capacity 4096 and per-peer mint limit make a single peer
  unable to fill it alone within one TTL; document restart as the escape
  hatch.

## Migration Plan

1. Coordinator ACKs this record on the bus.
2. Wait for Mac's log-change merge (D-k), rebase, implement, open PR.
3. Lab: flood from guest100, confirm `429` and that a second quiet guest
   still serves; cloud-init + one enroll still succeed.
4. Hogan optional: install and confirm VM 399 cloud-init still mints.
5. Archive after merge.

Rollback: revert the PR; behaviour returns to unbounded.

## Open Questions

None that block the record. Constant values (D-c) may be edited in review
without a new change.
