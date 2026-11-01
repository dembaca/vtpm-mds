## MODIFIED Requirements

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

## ADDED Requirements

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
  with no shared Go or C libraries of this project installed
- **THEN** it prints `vtpm-mds-reconcile <version>` and exits 0

## REMOVED Requirements

### Requirement: Two Binary Packages From One Source

**Reason:** The source now builds a third package, `vtpm-mds-reconcile`;
the requirement is replaced by "Three Binary Packages From One Source", which
keeps both scenarios for three packages.
**Migration:** None for installed systems; the host and guest packages keep
their names, contents and versions.
