# Design

## Context

See proposal.md — Why.

The binding itself is sound: `vm-inventory` resolves the caller once per
connection through `/proc/net/arp` and the cached inventory, and puts the
record on the connection context. What is missing is any consequence for a
connection where that produced nothing.

Three pieces of code turn "no record" into an identity:

- `imds.getInstanceID` falls through to `getClientIP`, which prefers
  `X-Forwarded-For` over the peer address.
- `imds.HandleInstanceIdentityDocument` calls the same helper, so the document
  carries the same value.
- `identity.getInstanceID` never looks at the inventory at all — it calls its
  own copy of `getClientIP` unconditionally, so even a bound caller is named by
  a header.

The last one means `/latest/identity` is not a weaker version of the metadata
path; it is a different path that never had the check.

`scripts/qemu-lab/e2e-netns.sh` already asserts `i-100`, so the lab's own smoke
test expects a bound caller and is unaffected.

## Goals / Non-Goals

**Goals:**

- Being in the inventory is what gets a caller instance data.
- No request header can name the caller, anywhere.
- An operator whose inventory is incomplete has a way to keep a deployment
  running while they fix it, and that way is visible in the configuration file.
- A new handler that reports instance data cannot forget the check.

**Non-Goals:**

- Changing how a caller is bound: the ARP lookup, the per-connection
  resolution and the cache are untouched. Binding IPv6 or loopback callers is
  separate work.
- Refusing unbound callers at the connection or middleware layer. Enrollment
  answers `401` and metadata answers `404`, and a blanket refusal would flatten
  that distinction and change the enrollment contract.
- Changing `PUT /latest/api/token` or `GET /health`. Both are deliberately
  unauthenticated and serve nothing about the caller.
- Real signing keys, quote verification or an attestation-aware `level` claim
  for `/latest/identity`. The new `workload-identity` capability records the
  placeholder signing as current behaviour precisely so that work can be
  proposed against a written contract instead of an undescribed endpoint.
- Rate limiting or logging policy for refused callers beyond one log line.

## Decisions

### A positive setting, `require_vm_identity`, defaulting to true

The setting reads as the security property it protects, which is what an
operator scanning `config.yaml` needs.

The cost is that its default depends on `config.Load` merging the built-in
defaults, which is what `fail-fast-on-unusable-configuration` fixes — a
configuration file that omits the key would otherwise load it as `false` and
silently disable the refusal. That is the same class of defect this repository
has already shipped once with `enable_ec2_compat`.

The alternative, an inverted `allow_unbound_callers` whose Go zero value is
already the safe one, was considered and rejected: it makes the configuration
file read as a list of permissions rather than of requirements, and it hides
the ordering dependency rather than removing the reason for it. Instead the
dependency is declared in the proposal, and a task pins the default by
verifying that a configuration file which does not mention the setting refuses
unbound callers.

### `404` with the catch-all body, not `403`

A refused caller gets `404` and the body `404 page not found`, byte for byte
what an unregistered route returns. An unknown caller therefore cannot map the
service's endpoints or learn that it failed an identity check rather than a
spelling check.

`403` was rejected because it confirms both the endpoint and the reason to a
caller that has given no account of itself. The operator's need is met by the
log line, which names the peer address and is on the side of the connection
that is trusted.

### Check after the token, before deriving anything

The token check stays first, so the two failures compose predictably and an
unbound caller with no token still sees `401`. The identity check then runs
before any instance value is computed, so a refused request never reads the
inventory, the ARP table or the peer address for a value it will not send.

### Wrap the routes at registration rather than checking in each handler

The handlers that report instance data are registered together in
`server.New`, already behind `cfg.MDS.EnableEC2Compat` and
`cfg.MDS.EnableTPMAttestation`. The check is applied by wrapping those
registrations, so the configuration value reaches it without a package-level
variable, and a handler added to that group inherits the check instead of
needing to remember it.

Per-handler calls were rejected for the reason the current defect exists:
`identity/handlers.go` is a second copy of the same logic that was never
updated.

### Delete the `X-Forwarded-For` branch rather than make it conditional

The service listens directly on the link-local metadata address with nothing in
front of it, so the header is never evidence. Keeping it behind a
"trusted proxy" setting would add a second thing to get wrong for a deployment
shape that does not exist here. Both copies of `getClientIP` collapse onto the
peer-address helper introduced by `parse-peer-address-correctly`.

### `identity` reuses the `imds` derivation

`identity.getInstanceID` is replaced by the same derivation the metadata
handler uses, rather than being fixed in parallel. One implementation is the
point: two copies is how this endpoint came to ignore the inventory.

## Risks / Trade-offs

- **[Risk] A deployment with an incomplete inventory loses metadata for its
  unlisted guests on upgrade** → Mitigation: `require_vm_identity: false`
  restores service while the inventory is completed, and the refusal is logged
  per caller with its address, so the list of guests to add is in the journal.
  The proposal marks the change BREAKING so it reaches the changelog.
- **[Risk] A guest reaching the service over IPv6 or loopback can never be
  bound, so it is refused permanently** → Accepted and stated in the proposal.
  `vm-inventory` resolves through `/proc/net/arp`, which holds IPv4 neighbours
  only; that limitation is already specified, and extending it is separate
  work. Such a caller was previously served an id derived from its own headers,
  which was not an identity either.
- **[Risk] `require_vm_identity: false` reads as a supported mode and becomes
  permanent** → Mitigation: the spec says in normative text that the
  synthesized id names the connection and is not an identity, and the setting
  is documented as a migration aid in `vtpm-mds(8)`.
- **[Risk] The `404` makes a misconfigured inventory look like a broken route
  to whoever debugs it** → Mitigation: the log line is explicit, and task 5
  verifies it names the caller's address.
- **[Trade-off] `/latest/identity` gains a contract that documents its
  placeholder signing rather than fixing it** → Deliberate. The endpoint is
  reachable today and names callers by header; stopping that is this change.
  Writing down that the token cannot be verified is what makes the remaining
  gap a proposable change rather than a rumour.

## Migration Plan

1. Before upgrading, check the journal of the running service for callers that
   resolve to no VM record, and add them to the inventory.
2. Deploy the package. Guests in the inventory are unaffected.
3. If unlisted guests must keep working, set `mds.require_vm_identity: false`
   and reload. They are served an id derived from their peer address, which is
   not caller-selectable.
4. Complete the inventory, remove the setting, and restart.

Rollback is reinstalling the previous package. No state or on-disk format
changes. Tokens are in memory and are invalidated by the restart either way.

## Open Questions

None.
