# Spec Delta

## ADDED Requirements

### Requirement: Cap Concurrent Connections Per Peer

The service SHALL allow at most 16 concurrent TCP connections from a single
peer IP on the metadata listener. The peer IP SHALL be taken from the accepted
connection's remote address with the same parsing rules as metadata peer
parsing; unparseable addresses share one bucket.

When a peer already holds 16 counted connections, a further accepted
connection SHALL be closed at `StateNew` and SHALL NOT consume a slot.
Requests on the first 16 connections SHALL continue to be served under the
other budgets. Closing or hijacking a counted connection SHALL free its slot
for that peer; closing a rejected over-cap connection SHALL NOT.

`GET /health` SHALL remain exempt from guest throttles, as required by the
instance-metadata guest-throttle rules.

#### Scenario: Peer at the connection cap loses the further connection

- **GIVEN** a peer already holds 16 accepted connections to the metadata
  listener
- **WHEN** that peer opens a further connection
- **THEN** the service closes that further connection without counting it,
  and limited HTTP requests on the existing 16 connections are still served

#### Scenario: Closing a connection frees a slot

- **GIVEN** a peer was at the connection cap and then closes one counted
  connection
- **WHEN** that peer opens a new connection and sends a limited request under
  its other budgets
- **THEN** the service serves the request normally

### Requirement: Forget Idle Peer Budgets

The per-peer throttle state SHALL NOT retain a peer entry forever. The service
SHALL remove a peer's budget when that peer has no open connections, no
request, token or enroll timestamps inside the current 60 second window, and
no rate-limit log line newer than that window. Removal SHALL run at least once
per minute while the service is up.

#### Scenario: Spoofed peer entries disappear after the window

- **GIVEN** many distinct peer IPs have each made one limited request and then
  gone idle with no open connections
- **WHEN** more than 60 seconds pass with no further activity from those peers
- **THEN** the service holds no budget entries for those peers
