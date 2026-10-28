## ADDED Requirements

### Requirement: Name No Particular Operator's Site In The Product

Code, tests, specs, man pages, shipped configuration, the README,
`ARCHITECTURE.md` and the scripts SHALL NOT name a host, cluster, location, VM ID, MAC address or
certificate authority of a particular operator, and no behaviour SHALL depend
on such a name. Examples and test fixtures SHALL use neutral names.

#### Scenario: A test needs a Proxmox node and a guest

- **GIVEN** a test that builds an `/etc/pve` tree
- **WHEN** it names the node, the cluster and a guest's VM ID and MAC
- **THEN** it uses neutral names such as `node-a`, `cluster-1`, `350` and a
  `bc:24:11` address that belongs to no real NIC

#### Scenario: A man page gives an example

- **GIVEN** a man page that shows an override or a path
- **WHEN** it gives an example
- **THEN** the example names no particular operator's host or VM

#### Scenario: The project's own records name the lab

- **GIVEN** an archived change, `debian/changelog` or the verification notes in a change's `tasks.md`
- **WHEN** it records where something was verified
- **THEN** it may name that host and VM, because it records the project's
  history rather than describing the product

## MODIFIED Requirements

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

- **GIVEN** a guest bound to VM `350` connecting from `10.0.0.5`
- **WHEN** it reads `GET /latest/meta-data/instance-id` and gets `200`
- **THEN** the access line holds `GET`, `"/latest/meta-data/instance-id"`, `200`,
  a duration, `10.0.0.5` and `350`, in that order

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
