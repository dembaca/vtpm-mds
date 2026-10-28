## MODIFIED Requirements

### Requirement: Load VM Records And Tags From The YAML Inventory

The YAML inventory SHALL be a mapping whose `vms` key holds the sequence of
entries; other top-level keys SHALL be ignored. Each entry SHALL support `id`,
`name`, `macs`, `ek_sha256`, `public_ipv4` and `tags`, and keys outside that set,
including the former `zone` and `region`, SHALL be ignored; a former `zone` or
`region` with a value SHALL be logged with the entry's `id`, once per load.

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

`tags` SHALL be optional and SHALL be a mapping from a tag key to a tag value;
each pair SHALL be served to that VM as an instance tag by the
`instance-metadata` capability. Keys and values SHALL be trimmed. A key or a
value that is not valid, as defined for tags below, SHALL be logged with the
entry's `id` and the key and SHALL be ignored, and SHALL NOT fail the load:
unlike a malformed `public_ipv4`, an ignored tag degrades to no tag and nothing
else about the VM. A key and a value SHALL be kept exactly as spelled. The record
SHALL carry the accepted pairs as its tags, and a record with none SHALL carry
none. The host's placement for this backend is zone `proxmox` and region `local`;
the YAML inventory declares no placement, neither for the host nor for a VM, so a
lab guest always reads `proxmox` and `local`.

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

#### Scenario: Declared tags reach the record

- **GIVEN** an inventory whose entry `100` declares `tags` with `owner: team-a`
  and `rack: b.2`
- **WHEN** the inventory is loaded
- **THEN** that record carries the tags `owner` as `team-a` and `rack` as `b.2`

#### Scenario: An invalid tag is logged and ignored

- **GIVEN** an inventory whose entry `100` declares `tags` with `owner: team-a`
  and `bad key: x`, alongside a valid entry `101`
- **WHEN** the inventory is loaded
- **THEN** the load succeeds, record `100` carries only `owner`, record `101` is
  unaffected, and the log names `100` and `bad key`

#### Scenario: A former zone and region field places nothing

- **GIVEN** an inventory whose entry `100` declares `zone: rack-b` and `region:
  region-1`
- **WHEN** the inventory is loaded
- **THEN** the load succeeds, the record carries neither value anywhere that
  `instance-metadata` reads a placement from, and the log names `100`, `zone` and
  `region`

#### Scenario: A malformed public address fails the load

- **GIVEN** an inventory whose entry `100` declares `public_ipv4` as
  `not-an-address`, `2001:db8::1`, `203.0.113.7/32` or `203.0.113.999`
- **WHEN** the inventory is loaded
- **THEN** loading fails with an error naming both `100` and `public_ipv4`, and
  no records are produced

### Requirement: Derive VM Records, Instance Tags And The Host Placement From Proxmox Guest Configuration

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

The `tags` key SHALL be stored like any other. A line `zone:` or `region:` is
stored like any other key too and SHALL have no meaning: the placement a VM is
served is the host's, as `instance-metadata` defines it.

**Instance tags.** A VM's instance tags SHALL be declared by Proxmox tags of the
form `vtpm_mds.io.<key>.<value>`, read from the live configuration's `tags` key.
The tags SHALL be split at `;`, `,` and whitespace. The prefix `vtpm_mds.io.`
SHALL be matched without regard to case. `<key>` SHALL be the characters after
the prefix up to the next `.`, and `<value>` everything after that dot, dots
included. Keys SHALL be case-sensitive, only the exact key `zone` SHALL be the short form of
`availability-zone`, and so `Zone` SHALL be a key of its own. The prefix
`topology.vtpm_mds.io/` SHALL be matched as it is spelled, case included.

A key SHALL be valid when it is 1 to 63 characters from `a-z`, `A-Z`,
`0-9`, `_`, `-` and `+`, and a value SHALL be valid when it is 1 to 255
characters from the same set and `.`. A key and a value SHALL be served exactly
as their source spells them: the service SHALL NOT change their case. A VM with
exactly one tag for a key, and a valid key and value, SHALL carry that pair as
its tags. A VM with two or more tags for one key, valid or not, SHALL carry no
value for that key, and the service SHALL log the VM id and the key. A tag with
the prefix and an invalid key or value SHALL be logged and ignored. A tag without
the prefix SHALL be ignored without a log entry, except that a tag beginning
`mds.zone.` or `mds.region.`, the former placement tags, SHALL be logged with the
VM id, once per load, and ignored. A fault in one VM's tags SHALL NOT
affect another VM's record or the load.

**Host topology.** The backend SHALL also determine the host's topology: keys,
each with a value, that describe the node the service runs on and that every caller
this node binds reads alike. Each key SHALL take its value from the first of the
sources named for it that gives a valid one, and a key with no source that gives
one SHALL have no value:

- `availability-zone`: the description line `topology.vtpm_mds.io/zone` or
  `topology.vtpm_mds.io/availability-zone`; then the node name, the target name of
  the `/etc/pve/local` link; then `proxmox`.
- `region`: the description line `topology.vtpm_mds.io/region`; then the
  `cluster_name` of `/etc/pve/corosync.conf`; then `local`.
- `node`: the description line `topology.vtpm_mds.io/node`; then the node name.
- `cluster`: the description line `topology.vtpm_mds.io/cluster`; then the
  `cluster_name`.
- every other `<key>`: the description line `topology.vtpm_mds.io/<key>`, and no
  other source.

The keys are resolved independently, so a description that names a zone only
leaves the region to the cluster name. `zone` is the short form of
`availability-zone` and the two SHALL count as one key.

The description of a node is stored in `/etc/pve/local/config`, which most nodes
do not have, as lines that start with `#`, each the URL-encoded text of one line
of the description, so that `topology.vtpm_mds.io/zone: AZ4` is stored as
`#topology.vtpm_mds.io/zone%3A AZ4`. The service SHALL decode each such line
and read a key from a decoded line of the form
`topology.vtpm_mds.io/<key>: <value>`. A line that does not decode or does not
have that form SHALL be ignored. A missing file SHALL declare nothing and SHALL
NOT be logged as a fault. Two or more lines for one key, valid or not, and a
line for `zone` together with one for `availability-zone`, SHALL declare nothing
for that key and SHALL be logged with the key; a single line whose key or value is
not valid SHALL be logged and ignored. The next source is then used, or no value
when there is none. At most 64 keys SHALL be read; those beyond the 64th in
ascending byte order SHALL be logged and ignored.

A key SHALL be valid when it is 1 to 63 characters from `a-z`, `A-Z`, `0-9`, `_`,
`-` and `+`. A value SHALL be valid when it is 1 to 63 characters from `a-z`,
`A-Z`, `0-9`, `_`, `-`, `+` and `.`, starting with a letter or digit. Both SHALL be
kept exactly as spelled: the service SHALL NOT change their case, so that a value
reads in the metadata service as it reads in the description. A node name or
cluster name that is not a valid value SHALL be logged and the next source used.
The host topology SHALL be determined whenever the inventory is loaded, so a
SIGHUP reload refreshes it. Only the guests of this node reach this service, so the
topology of this node is that of every caller it can bind.

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

#### Scenario: A tag becomes an instance tag

- **GIVEN** a guest config whose live section has `tags:
  vtpm_mds.io.owner.team-a;web`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries the tag `owner` as `team-a`, and `web` is ignored
  without a log entry

#### Scenario: A value keeps its dots

- **GIVEN** a guest config with `tags: vtpm_mds.io.rack.b.2.07`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries `rack` as `b.2.07`

#### Scenario: Case is kept and the prefix is matched without it

- **GIVEN** a guest config with `tags: VTPM_MDS.IO.Owner.Team-A`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries `Owner` as `Team-A`

#### Scenario: Two tags with one key declare nothing

- **GIVEN** a guest config with `tags: vtpm_mds.io.owner.a;vtpm_mds.io.owner.b;
  vtpm_mds.io.rack.7`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries `rack` as `7` and no `owner`, and the log names the
  VM id and `owner`

#### Scenario: An invalid tag is ignored

- **GIVEN** a guest config with `tags: vtpm_mds.io.owner.` and another guest with
  a valid `vtpm_mds.io.owner.team-a`
- **WHEN** the inventory cache is loaded
- **THEN** the first record carries no tags and the second carries `owner`

#### Scenario: A tag in a snapshot does not count

- **GIVEN** a guest config whose live section has no instance tag and whose
  `[before-change]` snapshot section has `tags: vtpm_mds.io.owner.old`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries no tags

#### Scenario: A former placement tag places nothing

- **GIVEN** a guest config with `tags: mds.zone.rack-b;mds.region.region-1`
- **WHEN** the inventory cache is loaded
- **THEN** the record carries no tags, no placement follows from them, and the
  log names the VM id and both tags

#### Scenario: A hand-written zone line has no meaning

- **GIVEN** a guest config whose live section contains `zone: rack-x` and
  `region: region-x`
- **WHEN** the inventory cache is loaded
- **THEN** the record's raw configuration map holds both lines like any other
  key, and no placement of any caller is derived from them

#### Scenario: The host's placement is this node and this cluster

- **GIVEN** `/etc/pve/local` links to `nodes/node-a` and
  `/etc/pve/corosync.conf` names the cluster `cluster-1`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `node-a` and its region is `cluster-1`

#### Scenario: Capitals in a node or cluster name are kept

- **GIVEN** `/etc/pve/local` links to `nodes/Node-A` and `/etc/pve/corosync.conf`
  names the cluster `Cluster-1`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `Node-A` and its region is `Cluster-1`

#### Scenario: The node description gives the host's placement

- **GIVEN** the node's `config` holds the description lines `Location:
  AZ4.M1.02.03`, `topology.vtpm_mds.io/region: region-a` and
  `topology.vtpm_mds.io/zone: AZ4`, each stored as a `#` line with its `:`
  encoded as `%3A`, and the node and cluster have other names
- **WHEN** the inventory cache is loaded
- **THEN** the host's `availability-zone` is `AZ4` and its `region` is `region-a`

#### Scenario: A description naming one kind leaves the other to the names

- **GIVEN** a description with `topology.vtpm_mds.io/zone: AZ4` only, on node
  `node-a` in cluster `cluster-1`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `AZ4` and its region is `cluster-1`

#### Scenario: An invalid description value falls back to the names

- **GIVEN** a description with `topology.vtpm_mds.io/zone: bad!zone` and a
  valid region, on node `node-a`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `node-a`, the region is the description's, and the
  log names the invalid value

#### Scenario: Two description lines of one kind declare nothing

- **GIVEN** a description with two `topology.vtpm_mds.io/zone` lines and one
  region line, on node `node-a`
- **WHEN** the inventory cache is loaded
- **THEN** the host's zone is `node-a` and the region is the description's

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

#### Scenario: A description adds a key of its own

- **GIVEN** a description with `topology.vtpm_mds.io/rack: rack-01` and no other
  topology line
- **WHEN** the inventory cache is loaded
- **THEN** the host's topology has `rack` as `rack-01`, besides the defaults of
  `availability-zone`, `region`, `node` and `cluster`

#### Scenario: A description line can use the long form of the zone

- **GIVEN** a description with `topology.vtpm_mds.io/availability-zone: room-a`
- **WHEN** the inventory cache is loaded
- **THEN** the host's `availability-zone` is `room-a`

#### Scenario: The short and the long form together declare nothing

- **GIVEN** a description with `topology.vtpm_mds.io/zone: room-a` and
  `topology.vtpm_mds.io/availability-zone: attic`, on node `node-a`
- **WHEN** the inventory cache is loaded
- **THEN** the host's `availability-zone` is `node-a` and the log names the key

#### Scenario: Node and cluster default from the host

- **GIVEN** `/etc/pve/local` links to `nodes/node-a`, `/etc/pve/corosync.conf` names
  the cluster `cluster-1` and the description names neither
- **WHEN** the inventory cache is loaded
- **THEN** the host's `node` is `node-a` and its `cluster` is `cluster-1`

#### Scenario: A description overrides node and cluster

- **GIVEN** a description with `topology.vtpm_mds.io/node: node-x` and
  `topology.vtpm_mds.io/cluster: lab`, on node `node-a` in cluster `cluster-1`
- **WHEN** the inventory cache is loaded
- **THEN** the host's `node` is `node-x` and its `cluster` is `lab`

#### Scenario: A host without a cluster file has no cluster

- **GIVEN** `/etc/pve/corosync.conf` does not exist and the description names no
  cluster
- **WHEN** the inventory cache is loaded
- **THEN** the host has no `cluster` key, its `region` is `local`, and the load does
  not fail

#### Scenario: A line with an invalid key or value is ignored

- **GIVEN** a description with `topology.vtpm_mds.io/ra ck: x`,
  `topology.vtpm_mds.io/row: a/b` and a valid `topology.vtpm_mds.io/rack: rack-01`
- **WHEN** the inventory cache is loaded
- **THEN** the host's topology has `rack` as `rack-01` and no `row`, and the log
  names the ignored lines

#### Scenario: Keys and the prefix are case-sensitive

- **GIVEN** a description with `topology.vtpm_mds.io/Zone: x` and
  `TOPOLOGY.VTPM_MDS.IO/rack: y`, on node `node-a`
- **WHEN** the inventory cache is loaded
- **THEN** the host's topology has `Zone` as `x`, no `rack`, and its
  `availability-zone` is `node-a`

#### Scenario: Keys beyond the limit are ignored

- **GIVEN** a description with 70 distinct valid `topology.vtpm_mds.io/` keys
- **WHEN** the inventory cache is loaded
- **THEN** 64 are read, the six last in ascending byte order are ignored, and the
  log says so

