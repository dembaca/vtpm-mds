# Spec Delta

## MODIFIED Requirements

### Requirement: Load VM Records From The YAML Inventory

The YAML inventory SHALL be a mapping whose `vms` key holds the sequence of
entries; other top-level keys SHALL be ignored. Each entry SHALL support `id`,
`name`, `macs`, `ek_sha256`, `public_ipv4`, `zone` and `region`, and keys outside
that set SHALL be ignored.

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

`zone` and `region` SHALL be optional and SHALL each declare the placement the
`instance-metadata` capability serves for that VM. Each SHALL be trimmed. A
value that is empty after trimming SHALL be treated as absent. A value that is
not a valid placement name, as defined for tags below, SHALL be logged with the
entry's `id` and the field and SHALL be treated as absent: unlike a malformed
`public_ipv4`, it SHALL NOT fail the load, because an absent placement degrades
to the host's value and not to no value. The record's raw configuration map SHALL
contain `zone` and `region` holding the validated values when, and only when,
they are declared. The host's placement for this backend is zone `proxmox` and
region `local`.

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

#### Scenario: A declared zone and region reach the record

- **GIVEN** an inventory whose entry `100` declares `zone: rack-b` and `region:
  bgl-1`
- **WHEN** the inventory is loaded
- **THEN** that record's raw configuration map holds `zone` as `rack-b` and
  `region` as `bgl-1`

#### Scenario: An invalid zone is logged and ignored

- **GIVEN** an inventory whose entry `100` declares `zone: "Rack B!"` and a valid
  `region`, alongside a valid entry `101`
- **WHEN** the inventory is loaded
- **THEN** the load succeeds, record `100` carries its region and no `zone` key,
  record `101` is unaffected, and the log names `100` and `zone`

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

The key `zone` and the key `region` SHALL NOT be stored from a line either, as
`public-ipv4` is not: their only source on this backend is the VM's tags, so a
line that someone added by hand cannot declare a placement. The `tags` key SHALL
be stored like any other.

**Placement tags.** A VM's zone SHALL be declared by a tag of the form
`mds.zone.<name>` and its region by a tag `mds.region.<name>`, read from the
live configuration's `tags` key. The tags SHALL be split at `;`, `,` and
whitespace. `<name>` SHALL be every character after the prefix and SHALL be
valid when it is 1 to 63 characters from `a-z`, `0-9`, `_`, `-`, `+` and `.`,
starting with a letter or digit. A VM with exactly one valid tag of a kind
SHALL carry that value in its raw configuration map, under `zone` or `region`.
A VM with two or more tags of one kind, valid or not, SHALL carry no value of
that kind, and the service SHALL log the VM id and the kind. A tag with the
prefix and an invalid name SHALL be logged and ignored. A fault in one VM's
tags SHALL NOT affect another VM's record or the load.

**Host placement.** The backend SHALL also determine the host's placement: the
zone is the name of the node, the target name of the `/etc/pve/local` link, and
the region is the `cluster_name` of `/etc/pve/corosync.conf`, or `local` when
that file is absent, unreadable or names no cluster. It SHALL be determined
whenever the inventory is loaded, so a SIGHUP reload refreshes it. The node name
and the cluster name SHALL be lowercased before they are checked, because Proxmox
allows capitals in both and a placement name has none. One that is still not a
valid placement name SHALL be logged and replaced by `proxmox` and `local`
respectively. Only the guests of this node reach this
service, so this node's name is the zone of every caller it can bind.

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

#### Scenario: An unreadable config file is skipped

- **GIVEN** this node's `qemu-server` directory holds a readable `100.conf`
  and a `101.conf` that cannot be read
- **WHEN** the inventory cache is loaded
- **THEN** the load succeeds with a record for `100` and none for `101`

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

#### Scenario: A zone tag becomes the VM's zone

- **GIVEN** a guest config whose live section has `tags: mds.zone.rack-b;web`
- **WHEN** the inventory cache is loaded
- **THEN** the record's raw configuration map holds `zone` as `rack-b` and has no
  `region` key

#### Scenario: Two zone tags declare nothing

- **GIVEN** a guest config with `tags: mds.zone.a;mds.zone.b;mds.region.r`
- **WHEN** the inventory cache is loaded
- **THEN** the record has no `zone` key, `region` is `r`, and the log names the
  VM id and `zone`

#### Scenario: An invalid placement tag is ignored

- **GIVEN** a guest config with `tags: mds.zone.` and another guest with a valid
  `mds.zone.rack-b`
- **WHEN** the inventory cache is loaded
- **THEN** the first record has no `zone` key and the second has `rack-b`

#### Scenario: A placement tag in a snapshot does not count

- **GIVEN** a guest config whose live section has no placement tag and whose
  `[before-change]` snapshot section has `tags: mds.zone.old`
- **WHEN** the inventory cache is loaded
- **THEN** the record has no `zone` key

#### Scenario: A hand-written zone line is not stored

- **GIVEN** a guest config whose live section contains `zone: rack-x` and
  `region: bgl-x`
- **WHEN** the inventory cache is loaded
- **THEN** the record's raw configuration map has neither key

#### Scenario: The host's placement is this node and this cluster

- **GIVEN** `/etc/pve/local` links to `nodes/hogan` and
  `/etc/pve/corosync.conf` names the cluster `bgl-cluster`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `hogan` and its region is `bgl-cluster`

#### Scenario: Capitals in a node or cluster name are lowercased

- **GIVEN** `/etc/pve/local` links to `nodes/Hogan` and `/etc/pve/corosync.conf`
  names the cluster `BGL-Cluster`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `hogan` and its region is `bgl-cluster`

#### Scenario: A host without a cluster file has region local

- **GIVEN** `/etc/pve/corosync.conf` does not exist
- **WHEN** the inventory cache is loaded
- **THEN** the host's region is `local` and the load does not fail

#### Scenario: A hand-written public address is not stored

- **GIVEN** a guest config whose live section contains
  `public-ipv4: 203.0.113.7`, or the same line written as
  `public-ipv4=203.0.113.7`
- **WHEN** the inventory cache is loaded
- **THEN** the record's raw configuration map has no `public-ipv4` key, so
  `instance-metadata` neither lists nor serves a public address for that VM
