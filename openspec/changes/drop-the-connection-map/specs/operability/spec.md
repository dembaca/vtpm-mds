# Spec Delta

## ADDED Requirements

### Requirement: Keep No Per-Connection State After A Connection Closes

The host service SHALL NOT keep any data that is keyed by a connection, or that
refers to one, after that connection has closed. What it learns about a caller
when a connection opens, the VM the caller is bound to, SHALL live in the
connection and in the context of its requests, and SHALL be released with them.
The number of connections a guest opens SHALL NOT, by itself, make the service's
memory grow without end.

#### Scenario: Many closed connections leave nothing behind

- **GIVEN** a service bound to a VM record for its callers
- **WHEN** a caller opens and closes thousands of connections, each sending a
  request
- **THEN** the service's live heap afterwards is not larger by an amount that
  grows with the number of connections

#### Scenario: A request still reaches its VM

- **GIVEN** a caller bound to VM `100`
- **WHEN** it sends a request that reads its instance id
- **THEN** the answer is `i-100`, as before
