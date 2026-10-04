# Design

## Context

See proposal.md — Why.

`InstanceID` has two branches. A bound VM record yields `i-<vmid>`. Otherwise
it yields `i-` plus the peer address with `.` replaced by `-`, taken from
`getClientIP`. `getClientIP` parses `RemoteAddr` with `PeerIP` and substitutes
`127.0.0.1` when `PeerIP` returns nil, and returns the textual form of an IPv6
address unchanged. `getClientIP` has no other caller, so the substitution
exists only to feed the fallback id.

`RequireVMIdentity` already refuses an unbound caller before the handler runs
when `mds.require_vm_identity` is true, so the fallback branch is reachable only
in the opt-out configuration.

## Goals / Non-Goals

**Goals:**

- The service never serves an instance id derived from a value it invented.
- The way "no id" is expressed matches how the service already expresses an
  unavailable value.

**Non-Goals:**

- Changing which callers are served. `refuse-unbound-metadata-callers` owns
  that, and its default (refuse) is untouched.
- Making the fallback id a good identity. It names a connection, not an
  instance; the spec already says so.
- Adding a `public-ipv4` source. Unrelated, though in the same file.

## Decisions

### `InstanceID` returns the empty string when nothing can be derived

`InstanceID(r)` returns `""` for an unbound caller whose peer does not parse,
and callers treat the empty string as "no instance id". The alternative, a
`(string, bool)` or `(string, error)` signature, was rejected: `InstanceID` is
exported and used by `identity`, and the empty string is unambiguous because a
derived id is never empty (it always starts with `i-`). If the repository's
convention at implementation time is an explicit second return value, that is an
acceptable substitute — the observable behaviour in the spec delta is what is
fixed.

### The fallback id needs an IPv4 peer

The maintainer decided that an IPv6 peer is treated like an unparseable one.
The fallback id is therefore built only from `LocalIPv4(r)`, which already
returns the empty string for a peer with no IPv4 address and renders an
IPv4-mapped address as its dotted quad. An empty result means "no id". This
removes the need for `getClientIP` altogether, and it matches `local-ipv4`,
which answers an IPv6 peer with "no address" rather than a fragment.

### Answer `422`, not the `404` of a refusal

A caller with no derivable id is answered `422` with an empty body and
`Content-Type: text/plain`; the maintainer chose `422` over `404`. Reasons:

- There is no `200`-shaped value to give. The identity document has a fixed
  key set, and `instanceId` is the one key whose whole purpose is the identity;
  an empty string there reads as "this instance has no id" rather than "the
  service could not tell", and a consumer would cache it.
- The endpoint exists and the request is well formed; what cannot be processed
  is the caller's connection. `404` would claim the path does not exist and
  would be indistinguishable from the `require_vm_identity` refusal, so an
  operator debugging an opt-out deployment could not tell the two apart.
  The refusal's `404 page not found` exists to hide which paths are
  identity-bearing from an unbound caller; this case is only reachable with the
  refusal switched off, so there is nothing left to hide.
- `500` was rejected. The service is functioning; the request has no
  derivable answer, which is a client-side condition, and `500` would page an
  operator for a connection the listener produced.

The three handlers share one response helper, so the status and body cannot
drift between them. The body is empty, matching `local-ipv4` and `public-ipv4`
when no value is available.

### The signature endpoint is untouched

`GET /latest/dynamic/instance-identity/signature` serves a constant and derives
nothing from the caller. It SHALL stay as it is; the spec already says it is
refused with the document only under `require_vm_identity`.

### `/latest/identity` follows without a spec edit

`workload-identity` defines the subject as derived exactly as the metadata
instance id is. Once `InstanceID` can report "none", `/latest/identity` must
answer `422` rather than sign a token whose `sub` is `i-`. The handler change is in
the tasks; the requirement text needs none, and task 3.2 verifies the handler.

## Risks / Trade-offs

- **[Risk] An operator running with `require_vm_identity: false` has a client
  that today receives `i-127-0-0-1` from an unparseable peer, or `i-fe80::1`
  from an IPv6 peer, and now gets `422`** → Accepted. The first value is
  identical for every unparseable peer, so it identifies nothing; the second is
  not an id any EC2 consumer parses.
- **[Risk] `RemoteAddr` is unparseable more often than assumed** (a unix
  socket, a proxy-protocol listener added later) → Mitigation: the effect is a
  `422` on the identity endpoints for those callers. That is the conservative
  failure: nothing is served.
- **[Trade-off] Loopback callers keep `i-127-0-0-1`** even though the same id is
  a plausible-looking default. It is a real peer address and the spec already
  documents it; changing it would be a different change.

## Migration Plan

None. No configuration or wire-format change beyond the status for the
unparseable-peer case.

## Open Questions

None. The status code (`422`) and the treatment of IPv6 peers were decided by
the maintainer in the `~vtpm-mds` channel on 2026-10-04.
