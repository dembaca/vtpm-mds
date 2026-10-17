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
when, and only when, one is declared.

The YAML inventory is the only backend that can declare a public address. The
Proxmox backend never stores a `public-ipv4` key, as `Derive VM Records From
Proxmox Guest Configuration` requires: Proxmox itself cannot write such a key,
so a line carrying one was added by hand and disappears on the next edit of
that VM.

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

### Requirement: Derive VM Records From Proxmox Guest Configuration

With the Proxmox backend the service SHALL read the guest configuration of
the node it runs on, from `/etc/pve/local/qemu-server`. `/etc/pve/local` is the
link Proxmox maintains to this node's directory under `/etc/pve/nodes`. The
service SHALL NOT scan `/etc/pve/nodes` or choose a node by any other rule, and
SHALL fail with an error naming `/etc/pve/local/qemu-server` when that
directory cannot be read. A guest of another node cannot reach this service
over the IMDS bridge, so this node's guests are the only callers there are.

Every `*.conf` file in that directory SHALL yield one VM record whose ID is the
file name without its `.conf` suffix. A config file that cannot be read SHALL
be skipped without failing the load.

Only the live configuration SHALL be read: the lines before the first line
whose trimmed form starts with `[`. Proxmox writes the live configuration
first and every snapshot, pending-change and special section after it, under
such a header, so nothing in those sections describes the running guest. Their
lines SHALL contribute neither keys nor MAC addresses.

Within the live configuration, blank lines and lines starting with `#` SHALL be
ignored. Every other line SHALL be split on the first `:` — or, failing that,
the first `=` — and stored key/value in the record's raw configuration map
with both sides trimmed, except that a `public-ipv4` key SHALL NOT be stored.
The record name SHALL be the `name` key of that map when present.

A MAC address SHALL be taken only from a line matching
`^net<N>[:=]\s*virtio=<17 characters of hex and colons>`, and SHALL be stored
lowercased. A NIC declared with any other model therefore SHALL NOT contribute
a MAC address, and the resulting record SHALL still exist with an empty MAC
list.

This backend SHALL NOT populate the EK pin; `ek_sha256` is only available from
the YAML inventory.

#### Scenario: A guest config becomes a VM record

- **GIVEN** `/etc/pve/nodes/<node>/qemu-server/100.conf` containing
  `name: guest-a` and `net0: virtio=52:54:00:A1:B2:C3,bridge=vmbr0`
- **WHEN** the inventory cache is loaded
- **THEN** the cache holds a record with ID `100`, name `guest-a` and the MAC
  `52:54:00:a1:b2:c3`

#### Scenario: A non-virtio NIC yields no MAC

- **GIVEN** a guest config whose only NIC line is
  `net0: e1000=52:54:00:A1:B2:C3,bridge=vmbr0`
- **WHEN** the inventory cache is loaded
- **THEN** the record for that VM exists with an empty MAC list, so no caller
  can be bound to it

#### Scenario: No Proxmox node directory

- **GIVEN** `/etc/pve/local/qemu-server` does not exist or cannot be read
- **WHEN** the inventory cache is loaded
- **THEN** the load fails with an error naming that path and the cache is left
  unchanged

#### Scenario: This node is read, not the first node by name

- **GIVEN** `/etc/pve/nodes` holds `aaa` and `bbb`, both with a `qemu-server`
  directory, and `/etc/pve/local` links to `nodes/bbb`
- **WHEN** the inventory cache is loaded
- **THEN** the records are exactly those of `bbb`

#### Scenario: A snapshot does not bind a removed NIC

- **GIVEN** a guest config whose live section has `name: guest-b` and
  `net0: virtio=52:54:00:00:00:01,bridge=vmbr0`, followed by a `[before-change]`
  snapshot section with `name: guest-old` and
  `net0: virtio=52:54:00:00:00:02,bridge=vmbr0`
- **WHEN** the inventory cache is loaded
- **THEN** the record has name `guest-b` and the single MAC `52:54:00:00:00:01`,
  and no caller with MAC `52:54:00:00:00:02` is bound to it

#### Scenario: Pending and special sections are not live configuration

- **GIVEN** a guest config whose live section has one `virtio=` NIC, followed
  by a `[PENDING]` section adding `net1: virtio=52:54:00:00:00:03` and a
  `[special:cloudinit]` section
- **WHEN** the inventory cache is loaded
- **THEN** the record has only the live NIC's MAC

#### Scenario: A hand-written public address is not stored

- **GIVEN** a guest config whose live section contains
  `public-ipv4: 203.0.113.7`
- **WHEN** the inventory cache is loaded
- **THEN** the record's raw configuration map has no `public-ipv4` key, so
  `instance-metadata` neither lists nor serves a public address for that VM
