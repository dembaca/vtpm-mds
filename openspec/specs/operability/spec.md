# Operability

## Purpose

Let an operator determine which build of the daemon and of the guest client is
installed, without starting the service or reaching a TPM, and make the host
service's restart rate limit effective.

## Requirements

### Requirement: Both Binaries Report Their Build Version

`vtpm-mds` and `devid-enroll` SHALL each accept a `-version` flag, print
`<program name> <version>` on stdout, and exit 0.

The reported version SHALL be the string injected at build time. For a binary
installed from a Debian package of this source, that SHALL equal the package
`Version`.

#### Scenario: Daemon reports its version

- **WHEN** `vtpm-mds -version` is run
- **THEN** it prints `vtpm-mds <version>` and exits 0

#### Scenario: Guest client reports its version

- **WHEN** `devid-enroll -version` is run
- **THEN** it prints `devid-enroll <version>` and exits 0

#### Scenario: Packaged binary matches its package version

- **GIVEN** a binary installed from a `.deb` built from this source
- **WHEN** its `-version` output is compared with the `Version` field of the
  package that owns it
- **THEN** the two match

### Requirement: Version Reporting Has No Side Effects

Printing the version SHALL NOT open a TPM device, read a configuration file,
bind a listen address, or write to disk.

#### Scenario: Client version works without a TPM

- **GIVEN** a machine with no TPM device and no metadata service reachable
- **WHEN** `devid-enroll -version` is run as an unprivileged user
- **THEN** it prints its version and exits 0 without attempting to open a TPM

#### Scenario: Daemon version works without a config file

- **WHEN** `vtpm-mds -version` is run with no `-config` argument and no
  configuration file present
- **THEN** it prints its version and exits 0 without starting a server

### Requirement: Host Service Restart Rate Limit Is Effective

The `vtpm-mds` systemd unit SHALL declare its start-limit directives where
systemd applies them, so that a daemon which fails repeatedly is rate limited
rather than restarted without bound.

#### Scenario: Unit parses without unknown-key warnings

- **WHEN** the shipped unit is checked with `systemd-analyze verify`
- **THEN** it reports no unknown or ignored directive

### Requirement: A Requested Shutdown Is Not A Failure

When the daemon receives `SIGTERM` or `SIGINT` it SHALL run its graceful
shutdown path and SHALL terminate with exit status 0.

`http.ErrServerClosed` is the defined result of serving a listener that has
been shut down. The daemon SHALL treat it as the expected outcome of a
requested shutdown and SHALL NOT terminate the process on account of it. In
particular the goroutine that serves the listener SHALL NOT end the process
when it observes that sentinel, so it cannot race the shutdown path and
pre-empt the exit status.

The shipped systemd unit SHALL therefore reach `inactive (dead)` after
`systemctl stop`, and `systemctl restart` SHALL complete without an
intervening failed state. The unit SHALL achieve this without declaring a
non-zero exit status to be a success.

#### Scenario: Stopping the service leaves it inactive

- **GIVEN** a running `vtpm-mds` unit
- **WHEN** an operator runs `systemctl stop vtpm-mds`
- **THEN** the service state is `inactive (dead)`, the result is `success`,
  and `systemctl is-failed vtpm-mds` reports `inactive`

#### Scenario: Restart does not pass through failed

- **GIVEN** a running `vtpm-mds` unit
- **WHEN** an operator runs `systemctl restart vtpm-mds`
- **THEN** the unit is active afterwards and its recorded result is never
  `exit-code`

#### Scenario: Graceful shutdown completes before exit

- **GIVEN** a running daemon
- **WHEN** it receives `SIGTERM`
- **THEN** it logs that it is shutting down, logs that the server stopped
  gracefully, and exits 0

### Requirement: A Listener Failure Is Still A Failure

An error from serving the listener that is not the shutdown sentinel SHALL be
logged and SHALL terminate the process with a non-zero exit status, so that a
daemon which cannot serve is reported as `failed` and is rate limited by the
unit's start-limit directives.

#### Scenario: Listen address cannot be bound

- **GIVEN** a configuration whose `listen_addr` is an address the host cannot
  bind
- **WHEN** the daemon is started
- **THEN** it logs the error, exits non-zero, and the unit enters `failed`

#### Scenario: Repeated startup failure is rate limited

- **GIVEN** a daemon that exits non-zero on every start because it cannot bind
  its listen address
- **WHEN** systemd restarts it under `Restart=on-failure`
- **THEN** the unit's `StartLimitIntervalSec` / `StartLimitBurst` take effect
  and systemd stops retrying

### Requirement: Write Debug Lines Only When Started With The Debug Flag

The host service SHALL write the lines it marks as debug output, those whose text
starts with `[DEBUG`, only when it was started with the `-debug` flag. Without the
flag it SHALL write none of them, for any request, connection or reload. With the
flag it SHALL write them as it did before this requirement. The flag SHALL be read
once, at start: changing it takes a restart. The service SHALL NOT write a debug
line as a side effect of another log level or of any setting of the
configuration file.

Lines that report a start, a reload, a refusal, a failure or an enrolment, and the
access line, are not debug output and SHALL be written either way, and SHALL NOT
start with `[DEBUG`: a line that reports an error does not carry the debug marker.

#### Scenario: A normal run writes no debug line

- **GIVEN** the service started without `-debug`
- **WHEN** a guest sends a request that is bound to a VM, and another that is not
- **THEN** no line starting with `[DEBUG` is written, and the access line of each
  request is

#### Scenario: A debug run writes the debug lines

- **GIVEN** the service started with `-debug`
- **WHEN** a guest sends a request
- **THEN** the debug lines for its connection are written, as before

#### Scenario: An error is written without the flag

- **GIVEN** the service started without `-debug`
- **WHEN** a start or a reload fails to load the VM configs
- **THEN** a line naming the failure and its source is written, and it does not
  start with `[DEBUG`

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

### Requirement: Confine The Daemon To What It Uses

The shipped systemd unit SHALL confine `vtpm-mds` to the resources it uses. It
SHALL run the daemon as the system user `vtpm-mds` with the supplementary group
`www-data`, never as root, and SHALL limit the capability bounding set to
`CAP_NET_BIND_SERVICE`, so that the daemon reads only what that user or its
groups may read: the Proxmox cluster files (`root:www-data 0640`) but not
`/etc/pve/priv/`. It cannot load kernel modules or change kernel tunables. The
CA key SHALL reach it as a systemd credential, without a change to the key
file's owner or mode. It SHALL mount the file system read-only for the daemon
except for its state directory, deny it devices, new namespaces, set-user-ID
programs and real-time scheduling, restrict its system calls to the
`@system-service` set without `@privileged` and `@resources`, and restrict its
sockets to `AF_INET`, `AF_INET6` and `AF_UNIX`; a system call the filter blocks
SHALL fail with `EPERM` rather than end the process. It SHALL deny
traffic to and from every address except link-local IPv4 and loopback.

`systemd-analyze security` of systemd 257 SHALL rate the installed unit at
**4.0 or lower**. The unit SHALL be part of `pve-cluster.service`, so that a
restart of the cluster file system restarts the daemon with a fresh read-only
view of `/etc/pve`.

#### Scenario: The installed unit is rated OK

- **GIVEN** the host package installed and the service started
- **WHEN** an operator runs `systemd-analyze security vtpm-mds`
- **THEN** the overall exposure is 4.0 or lower

#### Scenario: The daemon cannot use root's file privileges

- **GIVEN** the running service
- **WHEN** its capability sets are read from `/proc/<pid>/status`
- **THEN** the bounding set holds `CAP_NET_BIND_SERVICE` and nothing else

#### Scenario: Metadata and enrollment still work under the confinement

- **GIVEN** the confined service on a Proxmox host with a bound guest
- **WHEN** the guest reads the metadata tree and enrolls a DevID
- **THEN** both succeed as before, and a reload rereads the guest configs

#### Scenario: The daemon runs as its own user

- **GIVEN** the running service
- **WHEN** its credentials are read from `/proc/<pid>/status`
- **THEN** its user is `vtpm-mds`, its groups are `vtpm-mds` and `www-data`, and
  it cannot list `/etc/pve/priv/`

#### Scenario: The CA key stays readable

- **GIVEN** the confined service on a host whose CA key is `root:root 0600` and
  is passed as a systemd credential
- **WHEN** a bound guest enrolls
- **THEN** a certificate is issued, and the journal holds no `DevID enrollment
  disabled` line

#### Scenario: The address filter is configured

- **GIVEN** the running service
- **WHEN** an operator runs `systemctl show -p IPAddressDeny -p IPAddressAllow vtpm-mds`
- **THEN** every address is denied except `169.254.0.0/16` and loopback

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

### Requirement: Cap Concurrent Connections Per Peer

The service SHALL allow at most 16 concurrent TCP connections from a single
peer IP on the metadata listener. The peer IP SHALL be taken from the accepted
connection's remote address with the same parsing rules as metadata peer
parsing; unparseable addresses share one bucket.

When a peer already holds 16 counted connections, a further accepted
connection SHALL be closed at `StateNew` and SHALL NOT consume a slot.
Requests on the first 16 connections SHALL continue to be served under the
other budgets. Closing or hijacking a counted connection SHALL free its slot
for that peer; closing a rejected over-cap connection SHALL NOT.

`GET /health` SHALL remain exempt from guest throttles, as required by the
instance-metadata guest-throttle rules.

#### Scenario: Peer at the connection cap loses the further connection

- **GIVEN** a peer already holds 16 accepted connections to the metadata
  listener
- **WHEN** that peer opens a further connection
- **THEN** the service closes that further connection without counting it,
  and limited HTTP requests on the existing 16 connections are still served

#### Scenario: Closing a connection frees a slot

- **GIVEN** a peer was at the connection cap and then closes one counted
  connection
- **WHEN** that peer opens a new connection and sends a limited request under
  its other budgets
- **THEN** the service serves the request normally

### Requirement: Forget Idle Peer Budgets

The per-peer throttle state SHALL NOT retain a peer entry forever. The service
SHALL remove a peer's budget when that peer has no open connections, no
request, token or enroll timestamps inside the current 60 second window, and
no rate-limit log line newer than that window. Removal SHALL run at least once
per minute while the service is up.

#### Scenario: Spoofed peer entries disappear after the window

- **GIVEN** many distinct peer IPs have each made one limited request and then
  gone idle with no open connections
- **WHEN** more than 60 seconds pass with no further activity from those peers
- **THEN** the service holds no budget entries for those peers

### Requirement: Reject Unknown Keys In Service Config YAML

When the service reads its YAML configuration file at start, it SHALL reject
the file if it contains a key that is not part of the configuration schema.
An unknown key or a mistyped field name SHALL be a startup error: the process
SHALL exit non-zero and SHALL NOT listen. A file that only uses known keys
SHALL continue to overlay onto the built-in defaults as today.

#### Scenario: Typo in a config key fails start

- **GIVEN** a configuration file that sets `mds.listen_addres` instead of
  `mds.listen_addr`
- **WHEN** the service starts with that file
- **THEN** it exits with a non-zero status and does not bind the listen address

#### Scenario: Known keys still load

- **GIVEN** a configuration file that sets only known `mds.*` keys
- **WHEN** the service starts with that file
- **THEN** it starts and serves `GET /health`

### Requirement: Reject Unknown Keys In YAML Inventory

When the service loads a YAML inventory from `mds.inventory_path`, it SHALL
reject the file if it contains a key that is not part of the inventory schema
(at the document root or under a VM entry). An unknown key SHALL fail that
load. The same rule SHALL apply on a SIGHUP reload.

#### Scenario: Unknown VM field fails the load

- **GIVEN** a YAML inventory whose VM entry has an unknown field
- **WHEN** the service loads that file at start or on reload
- **THEN** the load fails and the failure is reported in the journal

### Requirement: Keep The Previous Inventory After A Failed Reload

When a running service reloads its inventory (SIGHUP) and the new load fails,
it SHALL keep serving from the inventory that was loaded before the reload.
It SHALL NOT clear or replace that inventory with an empty map because of the
failure. Readiness SHALL reflect the failure as required by the health-check
requirement (degraded, `last_error` set, previous `vm_count` retained).

#### Scenario: Bad reload leaves callers bound

- **GIVEN** a service that loaded a YAML inventory with one VM and is serving
  that VM's instance id
- **WHEN** a SIGHUP reload points at a file that fails strict decode
- **THEN** a bound caller still receives its instance id from the previous
  inventory, and `GET /health` reports `degraded` with the previous `vm_count`

## Notes

Man pages `vtpm-mds(8)` and `devid-enroll(1)` document the flag; that is
packaging, covered by the `debian-packaging` capability's requirement that each
package ships its own man page.
