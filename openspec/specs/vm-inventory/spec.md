# VM Inventory

## Purpose

This capability is how `vtpm-mds` decides which guest it is talking to. It
loads VM identity records either from a YAML inventory file or from Proxmox
guest configuration under `/etc/pve`, binds an incoming TCP connection to one
of those records by looking the caller's IP address up in the host ARP table,
and publishes the bound record to request handlers through the request
context. It also covers what a record contains and how the in-memory cache is
reloaded. The metadata endpoints are specified by `instance-metadata` and the
enrollment protocol by `devid-enrollment`.

## Requirements

### Requirement: Select The Inventory Backend From Configuration

The service SHALL choose its inventory backend from the `mds.inventory_path`
configuration value. When `inventory_path` is a non-empty string, the service
SHALL load the YAML inventory at that path. When `inventory_path` is empty —
the built-in default — the service SHALL instead parse Proxmox guest
configuration under `/etc/pve`.

The two backends SHALL be mutually exclusive: a failure of the selected
backend SHALL NOT cause the other backend to be tried.

#### Scenario: Configured path selects the YAML backend

- **GIVEN** `mds.inventory_path` is set to a readable YAML inventory
- **WHEN** the inventory cache is loaded
- **THEN** the records come from that file and `/etc/pve` is not read

#### Scenario: Empty path selects the Proxmox backend

- **GIVEN** `mds.inventory_path` is empty or absent from the configuration
- **WHEN** the inventory cache is loaded
- **THEN** the records come from the Proxmox guest configuration under
  `/etc/pve`

#### Scenario: A failing YAML load does not fall back to Proxmox

- **GIVEN** `mds.inventory_path` points at a file that cannot be read or
  parsed
- **WHEN** the inventory cache is loaded
- **THEN** the load fails with that error and no Proxmox configuration is
  parsed

### Requirement: Load VM Records From The YAML Inventory

The YAML inventory SHALL be a mapping whose `vms` key holds the sequence of
entries; other top-level keys SHALL be ignored. Each entry SHALL support `id`,
`name`, `macs` and `ek_sha256`, and keys outside that set SHALL be ignored.

`id` SHALL be the record key, and an entry with an empty or missing `id` SHALL
be skipped silently. `name`, `macs` and `ek_sha256` SHALL all be optional. Each
MAC address SHALL be trimmed of surrounding whitespace and lowercased, and
entries that are empty after trimming SHALL be dropped. `ek_sha256` SHALL
likewise be trimmed and lowercased. The record's raw configuration map SHALL
contain exactly one key, `name`, holding the entry's `name` value.

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

### Requirement: Derive VM Records From Proxmox Guest Configuration

With the Proxmox backend the service SHALL locate a node directory by scanning
`/etc/pve/nodes` for the first entry that contains a `qemu-server`
subdirectory, and SHALL fail with an error when the nodes directory cannot be
read or when no such node exists.

Every `*.conf` file in that node's `qemu-server` directory SHALL yield one VM
record whose ID is the file name without its `.conf` suffix. A config file
that cannot be read SHALL be skipped without failing the load.

Within a config file, blank lines and lines starting with `#` SHALL be
ignored. Every other line SHALL be split on the first `:` — or, failing that,
the first `=` — and stored key/value in the record's raw configuration map
with both sides trimmed. The record name SHALL be the `name` key of that map
when present.

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

- **GIVEN** `/etc/pve/nodes` is unreadable or holds no directory with a
  `qemu-server` subdirectory
- **WHEN** the inventory cache is loaded
- **THEN** the load fails with an error and the cache is left unchanged

### Requirement: Bind A Caller To A VM Record By ARP Lookup

The listener SHALL wrap every accepted TCP connection so that the caller is
resolved at most once per connection, on connection setup or the first read,
whichever happens first. The result SHALL be reused for every request on that
connection, including keep-alive reuse, and SHALL NOT depend on the IMDSv2
token or on any request header.

Resolution SHALL proceed in this order and SHALL stop at the first step that
fails, leaving the connection bound to no VM record:

1. The connection's remote address SHALL be taken as a TCP address and
   rendered as an IP address string. A remote address that is not a TCP
   address, or that carries no IP, SHALL end resolution.
2. `/proc/net/arp` SHALL be read and scanned for a line whose first field
   equals that IP string exactly. A line whose hardware address is
   `00:00:00:00:00:00` is an incomplete entry and SHALL be skipped. A read
   failure, or reaching the end of the table with no complete matching line,
   SHALL end resolution.
3. The hardware address of the matching line, lowercased, SHALL be looked up
   in the cached inventory. A MAC that no cached record claims SHALL end
   resolution.
4. The matching record SHALL be bound to the connection.

`/proc/net/arp` SHALL be the only source consulted for the caller's MAC
address: no `ip neigh` call, no DHCP lease file, no client-supplied header and
no other fallback participates. Because that table holds IPv4 neighbours only,
a caller reaching the service over IPv6, or over the loopback interface, SHALL
NOT be bound to a VM record.

When more than one cached record claims the same MAC address, the record
chosen SHALL be unspecified: the lookup iterates the cache in Go map order.

#### Scenario: A guest MAC in the inventory binds the connection

- **GIVEN** a cached record whose MAC matches the ARP entry for the caller's
  IP address
- **WHEN** the guest opens a connection to the service
- **THEN** that record is bound to the connection and used for every request
  on it

#### Scenario: No ARP entry for the caller

- **GIVEN** a caller whose IP address has no complete entry in
  `/proc/net/arp`, such as a process connecting over loopback
- **WHEN** the connection is accepted
- **THEN** no MAC address is resolved and the connection is bound to no VM
  record

#### Scenario: A MAC that the inventory does not know

- **GIVEN** an ARP entry exists for the caller's IP address but no cached
  record lists that MAC
- **WHEN** the connection is accepted
- **THEN** the connection is bound to no VM record

#### Scenario: The binding is not re-evaluated per request

- **GIVEN** a connection already bound to a VM record
- **WHEN** further requests arrive on that same connection
- **THEN** they all see the record resolved when the connection was accepted

### Requirement: Match MAC Addresses Case-Insensitively

MAC address matching SHALL be case-insensitive. Inventory MACs SHALL be
lowercased and trimmed when loaded, the MAC read from `/proc/net/arp` SHALL be
lowercased, and the lookup SHALL trim and lowercase its argument and compare
case-insensitively against each stored MAC.

Case and surrounding whitespace SHALL be the only normalisation applied.
Comparison SHALL otherwise be exact string equality, so a MAC written with a
separator other than `:`, or without separators, SHALL NOT be treated as
equivalent to the colon-separated form that `/proc/net/arp` reports.

#### Scenario: Uppercase inventory MAC matches a lowercase ARP entry

- **GIVEN** an inventory entry listing `52:54:00:AA:BB:CC`
- **WHEN** a caller whose ARP entry reads `52:54:00:aa:bb:cc` connects
- **THEN** the lookup matches and that VM record is bound

#### Scenario: Surrounding whitespace is ignored

- **GIVEN** an inventory entry whose MAC value has leading and trailing spaces
- **WHEN** the inventory is loaded
- **THEN** the stored MAC is trimmed and is matched like any other MAC

#### Scenario: A different separator does not match

- **GIVEN** an inventory entry listing `52-54-00-aa-bb-cc`
- **WHEN** a caller whose ARP entry reads `52:54:00:aa:bb:cc` connects
- **THEN** the lookup does not match and the connection is bound to no VM
  record

### Requirement: Publish The Bound VM Record To Request Handlers

A bound VM record SHALL be placed in the connection's context under the
inventory context key, and handlers SHALL retrieve it from the request
context. The accessor SHALL return no record when the context carries no
value, when the value is of another type, or when it is nil, so handlers can
distinguish a bound caller from an unbound one.

A VM record SHALL carry exactly: the VM ID, an optional human-readable name,
an optional key/value map of raw configuration, the list of MAC addresses used
for caller identification, and an optional `ek_sha256` EK certificate pin.

The inventory layer SHALL only normalise and carry `ek_sha256`; it SHALL NOT
interpret or enforce it. Enforcement of the pin against a presented EK
certificate belongs to `devid-enrollment`.

#### Scenario: A handler reads the bound record

- **GIVEN** a connection bound to the record for VM `100`
- **WHEN** a handler asks for the VM config of the request
- **THEN** it receives that record, including its VM ID, name, MACs and any
  `ek_sha256` pin

#### Scenario: A handler sees no record for an unbound caller

- **GIVEN** a connection that resolution left unbound
- **WHEN** a handler asks for the VM config of the request
- **THEN** it receives no record

### Requirement: Serve Unbound Callers Without A VM Identity

An unbound caller SHALL NOT be refused at the connection or middleware layer:
the connection is accepted and dispatched exactly like a bound one, and each
handler decides for itself what a missing VM record means.

Handlers that require a VM identity SHALL refuse the request. DevID enrollment
SHALL reject an unbound caller with `401` and the message
`VM identity required (MAC not in inventory)`, as specified by
`devid-enrollment`.

The EC2-compatible metadata handlers SHALL NOT refuse an unbound caller. When
no VM record is bound, the instance ID SHALL fall back to a value synthesised
from the caller's IP address with every `.` replaced by `-` and prefixed with
`i-`, and that synthesised value SHALL be served with `200`. The IP address
used for that fallback SHALL be the first entry of the `X-Forwarded-For`
request header when that header is present, and the connection's remote
address otherwise, so a caller that is in no inventory can choose the instance
ID it is served. The fallback SHALL apply to every EC2-compatible handler that
derives an instance ID, including
`/latest/dynamic/instance-identity/document`.

#### Scenario: An unknown caller receives a synthesised instance ID

- **GIVEN** an inventory that contains only VM `100` with MAC
  `52:54:00:a1:b2:c3`, and a caller from `127.0.0.1` that matches no record
- **WHEN** the caller obtains an IMDSv2 token and requests
  `/latest/meta-data/instance-id` with it
- **THEN** the response is `200` with the body `i-127-0-0-1`

#### Scenario: The caller steers the synthesised instance ID

- **GIVEN** an unbound caller with a valid IMDSv2 token
- **WHEN** it requests `/latest/meta-data/instance-id` with the header
  `X-Forwarded-For: 10.9.9.9`
- **THEN** the response is `200` with the body `i-10-9-9-9`

#### Scenario: Enrollment refuses an unbound caller

- **GIVEN** a caller bound to no VM record
- **WHEN** it posts to a DevID enrollment endpoint
- **THEN** the request is rejected with `401` because VM identity is required

### Requirement: Load The Cache At Startup And Refresh It On SIGHUP

The inventory SHALL be held in a single process-wide cache, guarded for
concurrent access, and populated once at startup. A startup load failure SHALL
be logged as a warning and SHALL NOT prevent the service from starting; the
service then runs with an empty cache, so every caller is unbound.

`SIGHUP` SHALL reload the inventory of the running service through the same
backend selection, and SHALL report an error when no server has been
initialised. `SIGHUP` SHALL reload the inventory only: the configuration file
is not re-read, so `inventory_path` and every other setting keep the value
they had at startup.

A failed reload SHALL leave the previously cached records in place and SHALL
log the error. A successful reload SHALL replace the cache wholesale and SHALL
log the source, the number of VMs and the number of MAC addresses cached.

A reload SHALL NOT re-resolve connections that are already bound, because the
binding is made once per connection; a guest whose record changed SHALL be
matched against the new cache only on its next connection.

#### Scenario: Startup without a usable inventory still serves

- **GIVEN** `mds.inventory_path` points at a file that does not exist
- **WHEN** the service starts
- **THEN** it logs a warning, starts its listener, and binds no caller to a VM
  record

#### Scenario: SIGHUP picks up an added VM

- **GIVEN** a running service whose cache holds one VM with one MAC
- **WHEN** a second VM with two MACs is added to the inventory file and the
  service is sent `SIGHUP`
- **THEN** the cache is reloaded and holds two VMs with three MAC addresses

#### Scenario: A failed reload keeps the previous inventory

- **GIVEN** a running service with a populated cache
- **WHEN** the inventory file is made unparseable and the service is sent
  `SIGHUP`
- **THEN** the error is logged and the previously cached records remain in
  effect
