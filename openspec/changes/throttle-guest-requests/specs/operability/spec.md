# Spec Delta

## ADDED Requirements

### Requirement: Cap Concurrent Connections Per Peer

The service SHALL allow at most 16 concurrent TCP connections from a single
peer IP on the metadata listener. The peer IP SHALL be taken from the accepted
connection's remote address with the same parsing rules as metadata peer
parsing; unparseable addresses share one bucket.

When a peer is at that cap, further HTTP requests from that peer on the limited
surface SHALL be refused with status `429`, `Content-Type: text/plain`, body
`Too many requests` followed by a newline, and a `Retry-After` header, without
running the route handler. Closing or hijacking a connection SHALL free its
slot for that peer.

`GET /health` SHALL remain exempt from this refusal, as required by the
instance-metadata guest-throttle rules.

#### Scenario: Peer at the connection cap is refused

- **GIVEN** a peer already holds 16 accepted connections to the metadata
  listener
- **WHEN** that peer sends another limited HTTP request on a further
  connection
- **THEN** the service responds `429` with body `Too many requests` and does
  not run the route handler

#### Scenario: Closing a connection frees a slot

- **GIVEN** a peer was at the connection cap and then closes one connection
- **WHEN** that peer opens a new connection and sends a limited request under
  its other budgets
- **THEN** the service serves the request normally
