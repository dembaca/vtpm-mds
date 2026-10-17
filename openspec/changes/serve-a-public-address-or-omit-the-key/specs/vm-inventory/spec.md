# Spec Delta

## MODIFIED Requirements

### Requirement: Load VM Records From The YAML Inventory

The YAML inventory SHALL be a mapping whose `vms` key holds the sequence of
entries; other top-level keys SHALL be ignored. Each entry SHALL support `id`,
`name`, `macs`, `ek_sha256` and `public_ipv4`, and keys outside that set SHALL
be ignored.

`id` SHALL be the record key, and an entry with an empty or missing `id` SHALL
be skipped silently. `name`, `macs` and `ek_sha256` SHALL all be optional. Each
MAC address SHALL be trimmed of surrounding whitespace and lowercased, and
entries that are empty after trimming SHALL be dropped. `ek_sha256` SHALL
likewise be trimmed and lowercased.

`public_ipv4` SHALL be optional, and SHALL declare the public IPv4 address the
`instance-metadata` capability serves for that VM. It SHALL be trimmed. A value
that is empty after trimming SHALL be treated as absent. A value that is not a
valid IPv4 address — including a valid IPv6 address, a CIDR prefix and a
host-and-port pair — SHALL fail the load with an error naming both the entry's
`id` and the field, and SHALL NOT yield a partial inventory. The address is
validated here rather than where it is served because this file is the
project's own format and is written by an operator: a typo is a configuration
error, and degrading it to "no public address" would be indistinguishable from
not having declared one.

The record's raw configuration map SHALL contain `name`, holding the entry's
`name` value, and SHALL contain `public-ipv4` holding the validated address
when, and only when, one is declared. That key is spelled as the Proxmox
backend spells it, so a reader of a VM record need not know which backend
produced it.

The Proxmox backend validates no address: it stores every `key: value` line
verbatim, as `Derive VM Records From Proxmox Guest Configuration` requires, and
a malformed `public-ipv4:` line there is therefore carried into the record and
rejected where it is read instead. That asymmetry is deliberate. Failing the
whole inventory load over one typo in a file this service does not own would
deny metadata to every VM on the host, which is a worse outcome than the typo.

When two entries share an `id`, the last one read SHALL win. The loader SHALL
NOT validate MAC syntax, SHALL NOT reject a MAC that appears under more than
one `id`, and SHALL NOT require any entry to have a MAC at all.

A file that cannot be read, or whose contents are not valid YAML of this
shape, SHALL produce an error rather than a partial inventory. A file that
parses but declares no usable entry SHALL produce an empty inventory without
an error.

#### Scenario: A lab inventory entry is loaded

- **GIVEN** an inventory whose only entry has `id: "100"`, `name:
  mds-lab-guest` and one MAC `52:54:00:a1:b2:c3`
- **WHEN** the inventory is loaded
- **THEN** the cache holds one record keyed `100`, named `mds-lab-guest`, with
  that single lowercase MAC

#### Scenario: An entry without an id is skipped

- **GIVEN** an inventory containing an entry with no `id` alongside a valid
  entry
- **WHEN** the inventory is loaded
- **THEN** only the valid entry is present in the cache and no error is
  reported

#### Scenario: Malformed YAML is rejected

- **WHEN** the configured inventory file is not parseable YAML
- **THEN** loading fails with a parse error and no records are produced

#### Scenario: A declared public address reaches the record

- **GIVEN** an inventory whose entry `100` declares `public_ipv4: 203.0.113.7`
- **WHEN** the inventory is loaded
- **THEN** that record's raw configuration map holds `public-ipv4` as
  `203.0.113.7`, alongside `name`

#### Scenario: An entry declaring no public address carries no such key

- **GIVEN** an inventory whose entry `101` has no `public_ipv4`, or has one
  that is empty after trimming
- **WHEN** the inventory is loaded
- **THEN** that record's raw configuration map has no `public-ipv4` key at all,
  which is distinguishable from holding an empty one

#### Scenario: A malformed public address fails the load

- **GIVEN** an inventory whose entry `100` declares `public_ipv4` as
  `not-an-address`, `2001:db8::1`, `203.0.113.7/32` or `203.0.113.999`
- **WHEN** the inventory is loaded
- **THEN** loading fails with an error naming both `100` and `public_ipv4`, and
  no records are produced
