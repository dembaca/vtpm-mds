# Spec Delta

## ADDED Requirements

### Requirement: Let Each Handler Decide What An Unbound Caller Receives

An unbound caller SHALL NOT be refused at the connection or middleware layer:
the connection is accepted and dispatched exactly like a bound one, and each
handler decides for itself what a missing VM record means. Resolution failing
is information, not an error.

The inventory layer SHALL NOT itself consult `mds.require_vm_identity` or any
other policy setting. It reports whether a record was bound; the policy belongs
to the handlers.

Handlers that require a VM identity SHALL refuse the request:

- DevID enrollment SHALL reject an unbound caller with `401` and the message
  `VM identity required (MAC not in inventory)`, as specified by
  `devid-enrollment`.
- The EC2-compatible metadata handlers SHALL refuse an unbound caller as
  specified by `instance-metadata`, which also defines the one configuration
  setting that relaxes it.
- `GET /latest/identity` SHALL refuse an unbound caller as specified by
  `workload-identity`.

No handler SHALL derive a caller's identity from a request header. Because a
bound record comes from the ARP table and the cached inventory, and nothing
else may stand in for it, a caller cannot present an identity it was not
resolved to.

`PUT /latest/api/token` and `GET /health` SHALL be served to any caller, bound
or not.

#### Scenario: An unbound connection is still accepted

- **GIVEN** a caller whose IP address has no complete entry in
  `/proc/net/arp`, such as a process connecting over loopback
- **WHEN** it opens a connection and sends a request
- **THEN** the connection is accepted and dispatched, and the request reaches a
  handler

#### Scenario: Enrollment refuses an unbound caller

- **GIVEN** a caller bound to no VM record
- **WHEN** it posts to a DevID enrollment endpoint
- **THEN** the request is rejected with `401` because VM identity is required

#### Scenario: Metadata refuses an unbound caller

- **GIVEN** a caller bound to no VM record and a configuration that leaves
  `mds.require_vm_identity` at its built-in default
- **WHEN** it requests `/latest/meta-data/instance-id` with a valid IMDSv2
  token
- **THEN** the request is refused, as `instance-metadata` specifies

#### Scenario: An unbound caller can still mint a token and check health

- **GIVEN** a caller bound to no VM record
- **WHEN** it sends `PUT /latest/api/token` and `GET /health`
- **THEN** both respond `200`

## REMOVED Requirements

### Requirement: Serve Unbound Callers Without A VM Identity

**Reason**: Its third paragraph and its scenarios
`An unknown caller receives a synthesised instance ID` and
`The caller steers the synthesised instance ID` state the defect as the
contract — that the EC2-compatible handlers must not refuse an unbound caller,
that such a caller is served an id synthesised from its address, and that
`X-Forwarded-For` takes precedence so the caller picks its own id. The
replacement inverts those outcomes, so the scenarios cannot be carried forward.
Replaced by `Let Each Handler Decide What An Unbound Caller Receives`, which
keeps the connection-layer rule and the enrollment refusal verbatim.

**Migration**: None at the inventory layer — binding, caching and the ARP
lookup are unchanged. What an unbound caller is served afterwards is specified
by `instance-metadata`, whose own migration note covers it.
