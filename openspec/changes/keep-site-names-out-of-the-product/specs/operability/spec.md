## ADDED Requirements

### Requirement: Name No Particular Operator's Site In The Product

Code, tests, specs, man pages, shipped configuration, the README and the
scripts SHALL NOT name a host, cluster, location, VM ID, MAC address or
certificate authority of a particular operator, and no behaviour SHALL depend
on such a name. Examples and test fixtures SHALL use neutral names.

#### Scenario: A test needs a Proxmox node and a guest

- **GIVEN** a test that builds an `/etc/pve` tree
- **WHEN** it names the node, the cluster and a guest's VM ID and MAC
- **THEN** it uses neutral names such as `pve-a`, `cluster-1`, `199` and a
  `bc:24:11` address that belongs to no real NIC

#### Scenario: A man page gives an example

- **GIVEN** a man page that shows an override or a path
- **WHEN** it gives an example
- **THEN** the example names no particular operator's host or VM

#### Scenario: The project's own records name the lab

- **GIVEN** `AGENTS.md`, an archived change, `debian/changelog` or the
  verification notes in a change's `tasks.md`
- **WHEN** it records where something was verified
- **THEN** it may name that host and VM, because it records the project's
  history rather than describing the product
