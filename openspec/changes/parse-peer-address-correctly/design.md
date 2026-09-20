# Design

## Context

See proposal.md — Why.

Three helpers derive an address from `r.RemoteAddr` by splitting on `:` and
taking the first field: `getLocalIP` and `getClientIP` in `imds/handlers.go`,
and a second `getClientIP` in `identity/handlers.go`. `RemoteAddr` is whatever
the listener produced, which for a TCP listener is `host:port` with the host
bracketed when it is an IPv6 literal. The split is correct for IPv4 and wrong
for everything else.

`net.SplitHostPort` is the standard parser for exactly this string and strips
the brackets. It returns an error when there is no port, which is worth
tolerating: tests and `httptest` sometimes set `RemoteAddr` to a bare address.

## Goals / Non-Goals

**Goals:**

- One helper that turns `RemoteAddr` into a peer IP, used everywhere.
- An IPv6 peer is never served a fragment of its own address.
- The IPv4 path is byte-for-byte unchanged.

**Non-Goals:**

- Serving IPv6 addresses from `local-ipv4`. The key names its address family
  and the EC2 contract that guest tooling implements treats it as an IPv4
  dotted quad; handing it an IPv6 literal moves the parse failure rather than
  removing it.
- Adding an `ipv6` metadata key. EC2 has one, this service has no source for
  it, and adding a key is new product behaviour that needs its own change.
- Deciding *whether* an unbound or IPv6 caller is served at all — that is
  `refuse-unbound-metadata-callers`. This change only fixes what the value is
  when one is served.
- Binding IPv6 callers to VM records. `vm-inventory` resolves through
  `/proc/net/arp`, which is IPv4-only, and changing that is separate work.

## Decisions

### Parse with `net.SplitHostPort`, fall back to the raw string

The helper tries `net.SplitHostPort(r.RemoteAddr)` and, on error, treats
`RemoteAddr` as a bare host. The result is parsed with `net.ParseIP`. A value
that parses to nothing yields no address, which callers render as a `404` or an
empty string.

Splitting on the *last* colon by hand was rejected: it gets `[fe80::1]:5000`
right only until the brackets have to be stripped, at which point it is a worse
copy of the standard function.

### `To4()` decides the address family

`net.IP.To4()` returns non-nil for both a real IPv4 address and an IPv4-mapped
IPv6 address, and renders the dotted quad in both cases. That is exactly the
question `local-ipv4` asks, so it is the test the helper uses, rather than
inspecting the string for dots.

### `404` for a peer with no IPv4, empty string in the document

`local-ipv4` is a single value with no way to say "absent" other than the
status code, and `public-ipv4` already answers `404` for the same reason — so
an absent IPv4 is expressed the way this service already expresses one.

The identity document cannot do that: its key set is fixed and documented as
exactly ten keys, and dropping a key would break a consumer that indexes it.
The empty string keeps the shape and is unambiguous. Emitting `null` was
rejected because the document already uses `null` for the two fields that are
always `null`, and a third meaning would be read as "not applicable" rather
than "not available".

### One shared helper, two packages

`identity/handlers.go` keeps its own copy of the address helpers today.
Rather than leave two implementations that must be fixed twice, the corrected
helper lives in one place and `identity` uses it. Which package it lands in is
an implementation choice; what matters is that there is one.

## Risks / Trade-offs

- **[Risk] A consumer today reads `local-ipv4` and tolerates `[fe80`, and will
  now see a `404`** → Accepted. There is no consumer for which `[fe80` is
  usable, and a `404` is the documented answer for an unavailable address on
  the neighbouring key.
- **[Risk] `RemoteAddr` has a form neither branch expects, for example a unix
  socket path** → Mitigation: the fallback treats it as a bare host and
  `net.ParseIP` rejects it, so the result is "no address" rather than a
  fragment. The service only ever listens on TCP.
- **[Trade-off] An IPv6-only guest gets `404` on `local-ipv4` and an empty
  `privateIp`, so it learns less than before** → Accepted: what it learned
  before was wrong. Such a guest is also unbound in any case, which
  `refuse-unbound-metadata-callers` addresses on its own terms.

## Migration Plan

None. No configuration, state or wire-format change; the corrected values take
effect with the new binary.

## Open Questions

None.
