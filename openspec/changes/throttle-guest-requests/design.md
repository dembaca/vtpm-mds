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
- Cap live tokens **per peer** so one guest cannot fill the store; keep only a
  high global emergency brake.
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
  `imds/token.go` records the minting peer on each token (for quota only)
  and enforces D-h inside `GenerateToken`. Alternative: everything under
  `imds/` — rejected because concurrent connections are an `http.Server`
  concern and enroll is outside `imds`.
- **D-c. Default numbers** (constants, not config):

  | Limit | Default | Rationale |
  |---|---|---|
  | Token PUTs per peer | 30 / 60 s | cloud-init retries; blocks mint floods |
  | HTTP requests per peer | 120 / 60 s | metadata tree walks; blocks sustained flood |
  | Concurrent connections per peer | 16 | parallel SDK clients; blocks conn exhaustion |
  | Enroll start+finish per peer | 6 / 60 s | one enroll is two POSTs; allows a retry |
  | Live tokens per peer | 32 | oldest of that peer evicted (D-h) |
  | Global token emergency cap | 65536 | Notbremse + log; see D-h |

  Sliding window of 60 s for the rate buckets (fixed window also fine;
  implementer's choice, tested). Numbers are mine; the maintainer can
  tighten them in review without a new change if only the constants move.
- **D-d. What counts as a request.** Every HTTP request that reaches the
  middleware counts toward the general request limit, including token PUT
  and enroll. Token PUT and enroll additionally count toward their own
  buckets. A peer may therefore trip the general limit without tripping
  the token limit, and vice versa.
- **D-e. `GET /health` is exempt.** Only `GET /health` is short-circuited in
  the middleware (not `POST`/`HEAD`/… on that path), so probes stay free
  without opening an unlimited write surface. Unmatched routes stay inside
  the wrapper so a flood of unknown paths is still limited.
- **D-f. No config keys in this change.** Limits are package constants.
  Exposing them under `mds.*` is a small follow-up if operations need to
  tune without a rebuild. Alternative: YAML now — rejected to keep the
  file set and the ACK surface small.
- **D-g. `429` shape.** Status `429`, `Content-Type: text/plain`, body
  `Too many requests` with a trailing newline, headers
  `Retry-After: 60` (the limit window in seconds) and `Connection: close`.
  Same for every limit. No JSON, no which-limit detail to the guest (that
  belongs in the log). Distinct from `401` (token) and `404` (unbound
  identity).
- **D-h. Token store quota (coordinator, 2026-10-18, overrules the first
  draft).** A **global** hard cap that answers `429` to every VM when full
  is itself a DoS, and without bridge MAC-binding a guest can mint under
  many peer IPs. Instead:
  1. **Per peer:** at most 32 unexpired tokens. Before recording a 33rd for
     that peer, discard expired entries for the peer, then **evict the
     oldest live token of that peer only** — never another peer's. Mint
     still returns `200` with the new token.
  2. **Global emergency brake:** if the store would exceed 65536 tokens
     after per-peer eviction and expiry sweep, refuse the mint with `429`
     (D-g) and one log line. This brake only really holds after callers are
     bound to bridge ports (Hogan's separate change); until then a spoofing
     guest can still approach it via many IPs, which the per-peer mint rate
     (D-c) bounds but does not eliminate.
  3. The store **remembers the minting peer** for quota/eviction only.
     `ValidateToken` stays peer-agnostic: a token still authenticates any
     caller (unchanged "Require A Valid Session Token" rule).
- **D-i. Concurrent connections (revised after Fable pre-review).** Track
  in `http.Server.ConnState` for `StateNew` / `StateClosed` /
  `StateHijacked` per peer IP. On `StateNew`, if the peer already holds
  16 counted connections, **close that connection immediately** and do
  not increment the count — so the first 16 keep serving. A rejected
  connection is remembered until `StateClosed` so the close does not
  free a real slot. The TCP accept still happens (Go has no pre-accept
  filter without a custom listener); the excess conn does not stay open
  for Keep-Alive or ARP work.
- **D-j. Logging.** One log line when a peer first trips a limit in a
  window (`rate limit: peer=%s kind=%s`), not one line per refused
  request. Exact format aligns with Mac's access-log change after rebase.
- **D-k. Rebase rule.** Implementation lands on a branch from `main` after
  Mac's `quiet-the-log-and-record-issuance` (and `drop-the-connection-map`
  if it merges in the same PR) is on `main`, so the single `server.go`
  middleware line does not fight their edits.
- **D-l. Peer map cleanup (coordinator, 2026-10-18).** `guestLimiter.peers`
  must not grow without bound when a guest spoofs many source IPs. Once a
  minute (or on access), drop a peer entry when all three time lists are
  empty after prune, `conns == 0`, and every `logged` timestamp is older
  than the limit window. Same class of leak as `connectionMap` (#52).
- **D-m. Per-peer token index (Fable pre-review, 2026-10-18).**
  `GenerateToken` MUST NOT walk the whole token map under the write lock.
  Keep `byPeer map[string][]*Token` ordered by `CreatedAt`; mint, expiry
  discard and eviction for a peer touch only that slice. `liveCount`
  drives the emergency brake. The minute ticker may still sweep all peers.
  Log the emergency brake once per engagement (while at the cap), not once
  per refused mint.

## Risks / Trade-offs

- **[Risk] Defaults too tight for a real guest agent** → Mitigation: lab
  measure with cloud-init + a SPIRE enroll before merge; raise constants
  in the same PR if a legitimate path trips `429`.
- **[Risk] Peer IP spoofing on `vmbr_imds`** → Mitigation: out of scope
  here; Hogan's bind-to-bridge-port / firewall work. Throttling still
  bounds damage from one source address.
- **[Risk] Shared `"unknown"` bucket** → Mitigation: only unparseable
  peers; normal TCP guests always parse.
- **[Risk] Global emergency brake still shared across VMs** → Mitigation:
  set very high (65536); per-peer eviction is the normal path; name in the
  design that the brake is honest only after bridge binding.
- **[Risk] Evicting a peer's oldest token breaks a still-running client** →
  Mitigation: 32 is above cloud-init's usual mint count; clients remint on
  `401`.

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
