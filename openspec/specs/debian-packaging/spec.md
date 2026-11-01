# Debian Packaging

## Purpose

Ship the hypervisor daemon, the guest DevID enrollment client and the SPIRE
node alias reconciler as three independently installable Debian packages from
the same source: the host package for hypervisors, the guest package for VM
images, and the reconciler package for the SPIRE server host. Guests can
install the client without enabling a host IMDS service, and nothing of the
reconciler lands on a hypervisor.

## Requirements

### Requirement: Host Package Ships The Daemon Only

The `vtpm-mds` binary package SHALL install the metadata daemon at
`/usr/sbin/vtpm-mds`, the legacy aliases `prox-mds` and `qemu-mds`, the
`vtpm-mds` systemd unit, host configuration under `/etc/vtpm-mds`, and the
state directory `/var/lib/vtpm-mds`. It SHALL also install the port binder at
`/usr/sbin/vtpm-mds-bind` and its systemd unit `vtpm-mds-bind.service`, which
keeps the `bridge vtpm_mds_bind` nftables table.

It SHALL NOT install `/usr/bin/devid-enroll`.

#### Scenario: Host package contents

- **WHEN** `vtpm-mds` is installed on a hypervisor
- **THEN** `/usr/sbin/vtpm-mds`, `/usr/sbin/vtpm-mds-bind` and both systemd
  units are present, and `/usr/bin/devid-enroll` is absent

#### Scenario: Host package does not enable a guest client

- **WHEN** an operator inspects the installed `vtpm-mds` file list
- **THEN** no guest-only enrollment client path is included

#### Scenario: Removing the host package removes the table

- **GIVEN** a node where `vtpm-mds-bind` has applied its table
- **WHEN** the `vtpm-mds` package is removed
- **THEN** the `bridge vtpm_mds_bind` table no longer exists

### Requirement: Guest Package Ships The Client Only

The `devid-enroll` binary package SHALL install the guest enrollment client at
`/usr/bin/devid-enroll`, its man page, and the systemd oneshot and timer that
run the LDevID renewal check on boot and daily.

It SHALL NOT install the metadata daemon, the `vtpm-mds` systemd unit, host
configuration under `/etc/vtpm-mds`, or `/var/lib/vtpm-mds`.

Installing `devid-enroll` SHALL NOT enable or start an IMDS listener. Installing
it SHALL enable the renewal timer when a systemd manager is available.

#### Scenario: Guest package contents

- **WHEN** `devid-enroll` is installed on a guest
- **THEN** `/usr/bin/devid-enroll` is present, the renewal oneshot and timer
  units are present, and `/usr/sbin/vtpm-mds` is absent

#### Scenario: Guest install does not start the host daemon

- **WHEN** `devid-enroll` is installed on a machine with no `vtpm-mds` package
- **THEN** no `vtpm-mds` systemd unit is installed and nothing binds
  `169.254.169.1:80` as a result of that install

#### Scenario: Renewal timer is enabled

- **GIVEN** a guest with a running systemd manager
- **WHEN** `devid-enroll` is installed or upgraded
- **THEN** the renewal timer is enabled

### Requirement: Packages Are Independently Installable

No binary package of this source SHALL depend on another.

Installing `devid-enroll` SHALL NOT pull in `vtpm-mds` or
`vtpm-mds-reconcile`. Installing `vtpm-mds` SHALL NOT require `devid-enroll`
or `vtpm-mds-reconcile`. Installing `vtpm-mds-reconcile` SHALL NOT pull in
`vtpm-mds` or `devid-enroll`.

#### Scenario: Guest install does not pull the host daemon

- **WHEN** `devid-enroll` is installed with no other packages from this source
- **THEN** `vtpm-mds` is not installed as a dependency

#### Scenario: Host install does not require the guest client

- **WHEN** `vtpm-mds` is installed with no other packages from this source
- **THEN** installation succeeds without `devid-enroll`

#### Scenario: Reconciler install stands alone

- **WHEN** `vtpm-mds-reconcile` is installed with no other packages from this
  source
- **THEN** installation succeeds and neither `vtpm-mds` nor `devid-enroll` is
  installed as a dependency

### Requirement: Upgrade Moves The Client Off The Host Package

When upgrading from a `vtpm-mds` version that shipped `/usr/bin/devid-enroll`,
upgrading `vtpm-mds` alone SHALL remove that path from the host.

Installing `devid-enroll` alongside or after that upgrade SHALL own
`/usr/bin/devid-enroll` without an unpack conflict.

#### Scenario: Host upgrade drops the bundled client

- **GIVEN** a system with the pre-split `vtpm-mds` package that included
  `/usr/bin/devid-enroll`
- **WHEN** only `vtpm-mds` is upgraded to the split version
- **THEN** `/usr/bin/devid-enroll` is no longer provided by `vtpm-mds`

#### Scenario: Guest package takes over the client path

- **GIVEN** a system with the pre-split `vtpm-mds` package
- **WHEN** `devid-enroll` and the split `vtpm-mds` are installed together
- **THEN** `/usr/bin/devid-enroll` is owned by `devid-enroll` and unpack
  succeeds

### Requirement: Installing The Host Package Leaves systemd In Step

Installing or upgrading the host package SHALL leave the systemd manager
holding the unit definition the package just installed. The package's
maintainer scripts SHALL reload the manager after the unit file is in place,
so that no manual `systemctl daemon-reload` is needed for the installed unit
to be the effective one.

A first installation SHALL NOT start the daemon. The daemon binds a
link-local address that is absent on build hosts and on lab hosts without the
IMDS bridge, so starting it from `postinst` would fail there; whether the
service runs stays an operator decision.

An upgrade SHALL NOT stop the daemon before the new files are unpacked. After
the manager has been reloaded, an upgrade SHALL restart the daemon if, and only
if, it is running at that point, so a running daemon ends the upgrade running
the new version and a stopped daemon stays stopped. A restart that the service
manager refuses SHALL be reported on standard error and SHALL NOT fail the
installation. A daemon that the manager starts and that exits afterwards is
reported by the service manager, not by the installation.

The host package carries two units, `vtpm-mds` and `vtpm-mds-bind`. Every
rule above about stopping and restarting the daemon on upgrade SHALL apply to
each unit alike. In addition, whenever the package is configured (a first
installation, an upgrade, or `dpkg-reconfigure`), it SHALL start
`vtpm-mds-bind` if `vtpm-mds` is active at that point, and SHALL NOT start it
otherwise: the binder protects a running daemon, and a node without one starts
neither. A start that the service manager refuses SHALL be reported on
standard error and SHALL NOT fail the installation.

A reload, restart or start SHALL be attempted only where a systemd manager is
running and the installation targets the running system, and a failure to reload SHALL NOT fail the installation.

#### Scenario: Unit change takes effect without a manual reload

- **GIVEN** a host with the package installed and the service running
- **WHEN** a package whose `vtpm-mds.service` differs is installed with
  `dpkg -i --force-confold`
- **THEN** `systemctl status vtpm-mds` reports no warning that the unit file
  changed on disk, and the next `systemctl start` uses the new definition

#### Scenario: Installation does not start the daemon

- **GIVEN** a host where `vtpm-mds` is not running and the listen address
  cannot be bound
- **WHEN** the host package is installed
- **THEN** the installation succeeds, the unit is enabled, and no daemon was
  started

#### Scenario: Installing without a running systemd manager

- **GIVEN** a container or chroot with no systemd manager running
- **WHEN** the host package is installed
- **THEN** the installation succeeds and no reload is attempted

#### Scenario: Upgrade restarts a running daemon

- **GIVEN** a host where `vtpm-mds` is running an older version
- **WHEN** a newer host package is installed with `dpkg -i --force-confold`
- **THEN** after the installation `vtpm-mds` is active, its journal shows the
  new version starting, and no manual `systemctl start` was needed

#### Scenario: Upgrade leaves a stopped daemon stopped

- **GIVEN** a host where the package is installed and `vtpm-mds` is stopped
- **WHEN** a newer host package is installed
- **THEN** after the installation `vtpm-mds` is still inactive

#### Scenario: A refused restart does not fail the upgrade

- **GIVEN** a host where `vtpm-mds` is running and the service manager
  refuses to restart it
- **WHEN** the newer host package is installed
- **THEN** the installation succeeds, the package is configured, and standard
  error names the refused restart

#### Scenario: The upgrade that introduces the binder starts it

- **GIVEN** a host where `vtpm-mds` is running a version without
  `vtpm-mds-bind`
- **WHEN** a host package that ships `vtpm-mds-bind` is installed
- **THEN** after the installation both `vtpm-mds` and `vtpm-mds-bind` are
  active, without a manual `systemctl start`

#### Scenario: No running daemon, no binder

- **GIVEN** a host where `vtpm-mds` is not running
- **WHEN** the host package is installed or reconfigured
- **THEN** `vtpm-mds-bind` is enabled but not started

### Requirement: Document Intermediate DevID CA Material On The Host Package

The host package's shipped configuration comments and unit `LoadCredential=`
documentation SHALL describe the DevID CA material on the node as the
**cluster intermediate** certificate and private key, not the offline root
key. They SHALL note that SPIRE (or another relying party) needs the offline
**root certificate** (not the root key) as its trust anchor, and that the
root private key MUST NOT be installed on hypervisors.

This is documentation and example paths only; it SHALL NOT change postinst
reload/restart behaviour, package split, or which binary owns which path.

The systemd credential **name** stays `devid_ca.key`; its source file is the
per-cluster intermediate private key (design Q2, code-go).

#### Scenario: Packaged comments name intermediate not root key

- **GIVEN** the host package as built from this change
- **WHEN** an operator reads the shipped `config.yaml` comments and the unit
  credential documentation
- **THEN** they describe loading a cluster intermediate for signing, and warn
  that the offline root private key must not be placed on the node

#### Scenario: Packaging behaviour otherwise unchanged

- **GIVEN** a host upgrade that only updates those comments
- **WHEN** the package is installed
- **THEN** postinst reload/restart rules and package contents other than the
  commented paths remain as in the living `debian-packaging` requirements

### Requirement: Local Lab Builds Pass lintian Whatever The Branch

A local lab build (`make deb-local`) SHALL stamp the changelog with an entry
that names the commit, the branch and whether the working tree was dirty, and
every line the stamp adds under its `*` item SHALL be at most 80 columns
long, whatever the branch is called, so that `lintian --fail-on
error,warning` does not fail on the stamp. (lintian measures only those
lines, not the entry's header or its ` -- maintainer  date` trailer.) A branch name too long for its line SHALL be shortened; the commit
SHALL never be shortened beyond the 12-character abbreviation the version
carries.

#### Scenario: Long branch name

- **GIVEN** a checkout on a branch whose name is 90 characters long
- **WHEN** `make deb-local` builds the packages
- **THEN** no line under the stamped entry's `*` item exceeds 80 columns, and
  `lintian --fail-on error,warning` reports no `debian-changelog-line-too-long`

#### Scenario: The build is still identifiable

- **GIVEN** any checkout
- **WHEN** `make deb-local` builds the host package
- **THEN** its version is `<base>+git<date>.<12-character sha>`, with `.dirty`
  when the tree had changes, and the changelog entry names the same commit

### Requirement: Packages Are Compressed With xz On Every Build Host

Every binary package of this source SHALL be built with `control.tar.xz` and
`data.tar.xz` members, whatever compressor the build host's `dpkg-deb`
uses by default, so that a package built by the release workflow passes
Debian's `lintian --fail-on error,warning` like a local build. The release
workflow SHALL fail when a built package has members in another format.

#### Scenario: Release build on Ubuntu

- **GIVEN** the release workflow on a runner whose `dpkg-deb` defaults to
  zstd
- **WHEN** it builds the packages
- **THEN** `ar t` of each `.deb` lists `control.tar.xz` and `data.tar.xz`

#### Scenario: A package in another format is not released

- **GIVEN** a built package whose members are `control.tar.zst` and
  `data.tar.zst`
- **WHEN** the workflow checks the packages
- **THEN** the job fails before any package is uploaded or released

### Requirement: Three Binary Packages From One Source

The Debian source package `vtpm-mds` SHALL produce three
architecture-dependent binary packages, `vtpm-mds`, `devid-enroll` and
`vtpm-mds-reconcile`, versioned together from the same changelog.

`make deb` SHALL stage all three `.deb` files. A GitHub Release for a version
tag SHALL attach all three artifacts.

#### Scenario: Build produces all three packages

- **WHEN** the Debian package is built
- **THEN** the output contains `vtpm-mds_<version>_<arch>.deb`,
  `devid-enroll_<version>_<arch>.deb` and
  `vtpm-mds-reconcile_<version>_<arch>.deb` with matching versions

#### Scenario: Release attaches all three artifacts

- **WHEN** a GitHub Release is published for a version tag
- **THEN** all three `.deb` files are attached and documented as host, guest
  and SPIRE server host packages

### Requirement: Reconciler Package Ships The Reconciler Only

The `vtpm-mds-reconcile` binary package SHALL install the reconciler at
`/usr/sbin/vtpm-mds-reconcile`, the systemd unit
`vtpm-mds-reconcile.service`, the man page `vtpm-mds-reconcile(8)`, a
`sysusers.d` entry for the system user `vtpm-mds-reconcile`, and an example
configuration under `/usr/share/doc/vtpm-mds-reconcile/examples/`.

It SHALL NOT install a configuration file under `/etc`, the metadata daemon,
the port binder, the guest client, or any of their units. Neither `vtpm-mds`
nor `devid-enroll` SHALL install `/usr/sbin/vtpm-mds-reconcile`.

The unit SHALL run the reconciler as `vtpm-mds-reconcile` with no
capabilities and SHALL NOT name a group for the SPIRE admin socket or a
credential for a cluster token; both are added by the operator.

#### Scenario: Reconciler package contents

- **WHEN** `vtpm-mds-reconcile` is installed
- **THEN** `/usr/sbin/vtpm-mds-reconcile`, its unit and its man page are
  present, the user `vtpm-mds-reconcile` exists, and `/usr/sbin/vtpm-mds`,
  `/usr/sbin/vtpm-mds-bind`, `/usr/bin/devid-enroll` and
  `/etc/vtpm-mds-reconcile/config.yaml` are absent

#### Scenario: Host and guest packages do not carry the reconciler

- **WHEN** the file lists of `vtpm-mds` and `devid-enroll` are inspected
- **THEN** neither contains `/usr/sbin/vtpm-mds-reconcile`

### Requirement: Installing The Reconciler Package Starts Nothing Unconfigured

Installing `vtpm-mds-reconcile` SHALL enable its unit and SHALL NOT start it.
The unit SHALL start only when `/etc/vtpm-mds-reconcile/config.yaml` exists;
without it a start is skipped, not failed.

The package's maintainer scripts SHALL reload the systemd manager after the
unit file is in place. An upgrade SHALL NOT stop the reconciler before the new
files are unpacked, and SHALL restart it after the reload if, and only if, it
is running at that point. A restart the service manager refuses SHALL be
reported on standard error and SHALL NOT fail the installation. A reload or
restart SHALL be attempted only where a systemd manager is running and the
installation targets the running system.

#### Scenario: First install without configuration

- **GIVEN** a host with no `/etc/vtpm-mds-reconcile/config.yaml`
- **WHEN** `vtpm-mds-reconcile` is installed
- **THEN** the installation succeeds, the unit is enabled and inactive, and a
  later `systemctl start` leaves it inactive with an unmet condition, not
  failed

#### Scenario: Upgrade restarts a running reconciler

- **GIVEN** a host where the reconciler is running an older version
- **WHEN** a newer `vtpm-mds-reconcile` package is installed
- **THEN** after the installation the unit is active and its journal shows
  the new version starting

#### Scenario: Upgrade leaves a stopped reconciler stopped

- **GIVEN** a host where the package is installed and the unit is stopped
- **WHEN** a newer package is installed
- **THEN** the unit is still inactive afterwards

### Requirement: Release Attaches A Static Reconciler Binary

A GitHub Release for a version tag SHALL also attach
`vtpm-mds-reconcile_<version>_linux_amd64`, a statically linked binary
byte-identical to `/usr/sbin/vtpm-mds-reconcile` in that release's
`vtpm-mds-reconcile` package, and
`vtpm-mds-reconcile_<version>_linux_amd64.sha256` with its SHA-256. The
binary's `-version` SHALL print the release's changelog version. The workflow
SHALL fail before releasing when the binary is not statically linked or its
version differs.

#### Scenario: Binary matches the package

- **GIVEN** a published release
- **WHEN** the standalone binary and the one extracted from the
  `vtpm-mds-reconcile` package are hashed
- **THEN** both hashes equal the published `.sha256`

#### Scenario: Binary runs without a Debian userland

- **WHEN** the standalone binary is run with `-version` on a Linux amd64 host
  without any of this project's packages or shared libraries installed
- **THEN** it prints `vtpm-mds-reconcile <version>` and exits 0
