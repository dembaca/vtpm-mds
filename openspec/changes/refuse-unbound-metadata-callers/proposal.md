# Proposal

## Why

A caller the service cannot identify is served an identity anyway, and gets to
choose it.

`getInstanceID` falls back to `i-` plus the caller's IP address with dots
replaced by dashes whenever no VM record is bound to the connection. The
address it uses is the first element of `X-Forwarded-For` when that header is
present, and the peer address otherwise — and nothing in front of this service
sets or strips that header, because it listens directly on the link-local
metadata address. So an unknown guest asks for `i-10-9-9-9` by sending
`X-Forwarded-For: 10.9.9.9`, and receives it with `200`.

The same value is served as `instanceId` in
`/latest/dynamic/instance-identity/document`, the document cloud-init and the
AWS SDKs read as the instance's own identity.

`/latest/identity` is worse: `identity/handlers.go` has its own `getInstanceID`
that consults no inventory at all, so *every* caller — bound or not — gets a
JWT whose subject and `instance_id` claim are derived from
`X-Forwarded-For`. A guest that is in the inventory is issued a token naming a
machine it is not.

The result is that being in the inventory buys nothing at these endpoints. An
unenrolled caller is indistinguishable from an enrolled one, which is the
opposite of what the inventory is for, and the synthesised id is not even a
property of the caller — it is a request parameter.

## What Changes

- New configuration setting `mds.require_vm_identity`, default true. When it is
  true, a caller the inventory could not bind is refused at the endpoints that
  report its instance identity, and only those: `/latest/meta-data/instance-id`,
  both `/latest/dynamic/instance-identity/` endpoints, and `/latest/identity`.
- The rest of the metadata tree stays available to an unbound caller.
  `local-hostname` echoes its own `Host` header, `local-ipv4` reports its own
  peer address, `placement/availability-zone` and `services/domain` are fixed
  strings, and the index is a static listing — none of them asserts an identity
  the service vouches for, so refusing them would withhold information that is
  already the caller's own.
- **BREAKING** A refused request is answered `404` with the body
  `404 page not found` — exactly what the catch-all returns — so an unbound
  caller cannot tell a missing identity from a route that does not exist. The
  service logs the refusal with the caller's address so an operator can.
- **BREAKING** `X-Forwarded-For` is never consulted. With
  `require_vm_identity` set to false the old `i-<caller ip>` fallback returns,
  but it is derived from the connection's peer address only, so a caller can no
  longer choose the id it is served.
- **BREAKING** `/latest/identity` derives its subject and `instance_id` claim
  from the bound VM record, and refuses unbound callers under the same setting.
  It ignores inventory entirely today.
- `PUT /latest/api/token` and `GET /health` are unchanged: both stay
  unauthenticated and are served to any caller.
- DevID enrollment is unchanged. It already refuses an unbound caller with
  `401`.
- No change to the metadata values served to a *bound* caller.

## Capabilities

### New Capabilities

- `workload-identity`: `GET /latest/identity` and the JWKS endpoint that
  accompanies it — who may obtain a JWT identity document, whom it names, and
  what its signature is and is not worth. The endpoint exists and is
  unspecified today, which is why nothing contradicted it ignoring the
  inventory.

### Modified Capabilities

- `instance-metadata`: `Derive The Instance Id From Inventory Or The Caller
  Address` is replaced — its fallback and its `X-Forwarded-For` rule are the
  defect. `Serve An Unsigned Instance Identity Document` loses the clause
  stating that `instanceId` honours `X-Forwarded-For`. A new requirement
  enumerates the refused paths, so the individual endpoint requirements keep
  describing what a caller is served and `Serve The EC2 Metadata Tree` needs no
  change at all.
- `vm-inventory`: `Serve Unbound Callers Without A VM Identity` is replaced. It
  currently requires the EC2 handlers not to refuse an unbound caller and
  specifies the caller-steerable fallback as the contract.

## Impact

- `internal/config/config.go`: the new setting and its default.
- `imds/handlers.go`: `getInstanceID`, `getClientIP`, and a refusal check on
  the handlers that report an instance identity.
- `identity/handlers.go`: `getInstanceID` and `getClientIP`, which duplicate the
  `imds` helpers and ignore the inventory.
- `internal/server/server.go`: the setting has to reach the handlers.
- `imds/handlers_test.go`, `identity/handlers_test.go`: the
  `X-Forwarded-For` and unbound cases invert.
- `config.yaml` and `debian/vtpm-mds.8`: the new setting is documented where
  the others are.
- Any guest reaching the service over IPv6 or loopback, which `vm-inventory`
  cannot bind at all: such a caller is served an instance id today and is
  refused one after this change unless `require_vm_identity` is set to false.
  It keeps the rest of the metadata tree either way.
- Ordering: this change must be applied **after**
  `parse-peer-address-correctly`, which also modifies
  `Serve An Unsigned Instance Identity Document`; the delta here is written
  against the text that change leaves behind. It must also be applied **after**
  `fail-fast-on-unusable-configuration`, because the default of
  `require_vm_identity` is only effective once `config.Load` merges the
  built-in defaults — without it, a configuration file that omits the setting
  would load it as false and disable the refusal.
