# Spec Delta

## ADDED Requirements

### Requirement: Write Debug Lines Only When Started With The Debug Flag

The host service SHALL write the lines it marks as debug output, those whose text
starts with `[DEBUG`, only when it was started with the `-debug` flag. Without the
flag it SHALL write none of them, for any request, connection or reload. With the
flag it SHALL write them as it did before this requirement. The flag SHALL be read
once, at start: changing it takes a restart. The service SHALL NOT write a debug
line as a side effect of another log level or of any setting of the
configuration file.

Lines that report a start, a reload, a refusal, a failure or an enrolment, and the
access line, are not debug output and SHALL be written either way.

#### Scenario: A normal run writes no debug line

- **GIVEN** the service started without `-debug`
- **WHEN** a guest sends a request that is bound to a VM, and another that is not
- **THEN** no line starting with `[DEBUG` is written, and the access line of each
  request is

#### Scenario: A debug run writes the debug lines

- **GIVEN** the service started with `-debug`
- **WHEN** a guest sends a request
- **THEN** the debug lines for its connection are written, as before

#### Scenario: A reload writes no debug line without the flag

- **GIVEN** the service started without `-debug`
- **WHEN** it receives a SIGHUP
- **THEN** the line that reports the reload and its VM count is written, and no
  line starting with `[DEBUG` is

### Requirement: Write One Access Line Per Request That Names The Peer And The VM

For every request the host service SHALL write one access line holding, in this
order, the method, the path, the response status, the time the request took, the
IP address of the connection's peer without the port, and the ID of the VM record
the connection is bound to, or `-` when it is bound to none. The peer SHALL be the
address of the connection and SHALL NOT be taken from `X-Forwarded-For` or any
other request header. A peer whose address cannot be parsed SHALL be written as
`-`. The path SHALL be written quoted, so that a character in it that would end a
journal line is written as an escape and the request writes exactly one line.

#### Scenario: A bound caller is named

- **GIVEN** a guest bound to VM `399` connecting from `10.0.0.5`
- **WHEN** it reads `GET /latest/meta-data/instance-id` and gets `200`
- **THEN** the access line holds `GET`, `"/latest/meta-data/instance-id"`, `200`,
  a duration, `10.0.0.5` and `399`, in that order

#### Scenario: An unbound caller is named too

- **GIVEN** a caller bound to no VM record connecting from `10.0.0.9`
- **WHEN** it reads `GET /health`
- **THEN** the access line holds `10.0.0.9` as the peer and `-` as the VM

#### Scenario: The peer is not taken from a header

- **GIVEN** a request from `10.0.0.5` carrying `X-Forwarded-For: 10.9.9.9`
- **WHEN** the access line is written
- **THEN** the peer is `10.0.0.5`

#### Scenario: A path cannot write a second line

- **GIVEN** a request whose path, decoded, contains a line break followed by text
  that looks like a log line
- **WHEN** the access line is written
- **THEN** exactly one line is written for the request, and the break appears in it
  as an escape
