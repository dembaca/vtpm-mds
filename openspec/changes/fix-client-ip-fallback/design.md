# Design

## Context

See proposal.md — Why.

`InstanceID` has two branches. A bound VM record yields `i-<vmid>`. Otherwise
it yields `i-` plus the peer address with `.` replaced by `-`, taken from
`getClientIP`. `getClientIP` parses `RemoteAddr` with `PeerIP` and substitutes
`127.0.0.1` when `PeerIP` returns nil. `getClientIP` has no other caller, so
the substitution exists only to feed the fallback id.

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
- The IPv6 form of the fallback id; see Open Questions.

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

### Answer `404 page not found`, exactly as a refusal does

A caller with no derivable id is answered like a caller that
`require_vm_identity` would refuse: `404`, body `404 page not found`. Reasons:

- There is no `200`-shaped value to give. The identity document has a fixed
  key set, and `instanceId` is the one key whose whole purpose is the identity;
  an empty string there reads as "this instance has no id" rather than "the
  service could not tell", and a consumer would cache it.
- The status and body already mean "this endpoint will not tell you who you
  are" for this caller class, so a caller learns nothing new and the
  unauthenticated-probing argument made for the refusal applies unchanged.
- `500` was rejected. The service is functioning; the request has no
  derivable answer, which is a client-side condition, and `500` would page an
  operator for a connection the listener produced.

The refusal helper in `imds` is reused rather than copied, so the bytes cannot
drift.

### The signature endpoint is untouched

`GET /latest/dynamic/instance-identity/signature` serves a constant and derives
nothing from the caller. It SHALL stay as it is; the spec already says it is
refused with the document only under `require_vm_identity`.

### `/latest/identity` follows without a spec edit

`workload-identity` defines the subject as derived exactly as the metadata
instance id is. Once `InstanceID` can report "none", `/latest/identity` must
refuse rather than sign a token whose `sub` is `i-`. The handler change is in
the tasks; the requirement text needs none, and task 3.2 verifies the handler.

## Risks / Trade-offs

- **[Risk] An operator running with `require_vm_identity: false` has a client
  that today receives `i-127-0-0-1` from an unparseable peer and now gets
  `404`** → Accepted. No such client can be relying on the value: it is
  identical for every unparseable peer, so it identifies nothing.
- **[Risk] `RemoteAddr` is unparseable more often than assumed** (a unix
  socket, a proxy-protocol listener added later) → Mitigation: the effect is a
  `404` on the identity endpoints for those callers, the same as the default
  configuration gives every unbound caller. That is the conservative failure.
- **[Trade-off] Loopback callers keep `i-127-0-0-1`** even though the same id is
  a plausible-looking default. It is a real peer address and the spec already
  documents it; changing it would be a different change.

## Migration Plan

None. No configuration or wire-format change beyond the status for the
unparseable-peer case.

## Open Questions

- An IPv6 peer with `require_vm_identity: false` is served `i-fe80::1`: the
  fallback replaces only `.`, so the id keeps its colons. That is not invented,
  but it is not an EC2-shaped id either. Out of scope here because the spec
  states the `.`-to-`-` rule explicitly; if the maintainer wants IPv6 peers
  refused or encoded differently, it needs its own change.
