## ADDED Requirements

### Requirement: Name No Particular Operator's Site In The Product

The product (code, tests, `openspec/specs/`, man pages, `debian/` files
other than the changelog, shipped configuration, `README.md`,
`ARCHITECTURE.md` and the scripts) SHALL NOT name a host, cluster, location,
VM ID, MAC address or certificate authority of a particular operator. It SHALL
NOT make behaviour depend on any such name. Examples and test fixtures SHALL
use neutral names. The project's own records (`AGENTS.md`, archived changes,
`debian/changelog` and the verification notes of a change's `tasks.md`) MAY
name the hosts and VMs they were verified on.

#### Scenario: A test needs a Proxmox node and a guest

- **GIVEN** a test that builds an `/etc/pve` tree
- **WHEN** it names the node, the cluster and a guest's VM ID and MAC
- **THEN** it uses neutral names such as `pve-a`, `cluster-1`, `199` and a
  `bc:24:11` address that belongs to no real NIC

#### Scenario: A man page gives an example

- **GIVEN** a man page that shows an override or a path
- **WHEN** it gives an example
- **THEN** the example names no particular operator's host or VM
