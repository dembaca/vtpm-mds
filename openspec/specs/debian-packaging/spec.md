# Debian Packaging

## Purpose

Ship the hypervisor daemon and the guest DevID enrollment client as two
independently installable Debian packages from the same source, so guests can
install the client without enabling a host IMDS service.

## Requirements

### Requirement: Two Binary Packages From One Source

The Debian source package `vtpm-mds` SHALL produce two architecture-dependent
binary packages, `vtpm-mds` and `devid-enroll`, versioned together from the
same changelog.

`make deb` SHALL stage both `.deb` files. A GitHub Release for a version tag
SHALL attach both artifacts.

#### Scenario: Build produces both packages

- **WHEN** the Debian package is built
- **THEN** the output contains `vtpm-mds_<version>_<arch>.deb` and
  `devid-enroll_<version>_<arch>.deb` with matching versions

#### Scenario: Release attaches both artifacts

- **WHEN** a GitHub Release is published for a version tag
- **THEN** both `.deb` files are attached and documented as host versus guest
  packages

### Requirement: Host Package Ships The Daemon Only

The `vtpm-mds` binary package SHALL install the metadata daemon at
`/usr/sbin/vtpm-mds`, the legacy aliases `prox-mds` and `qemu-mds`, the
`vtpm-mds` systemd unit, host configuration under `/etc/vtpm-mds`, and the
state directory `/var/lib/vtpm-mds`.

It SHALL NOT install `/usr/bin/devid-enroll`.

#### Scenario: Host package contents

- **WHEN** `vtpm-mds` is installed on a hypervisor
- **THEN** `/usr/sbin/vtpm-mds` and the systemd unit are present, and
  `/usr/bin/devid-enroll` is absent

#### Scenario: Host package does not enable a guest client

- **WHEN** an operator inspects the installed `vtpm-mds` file list
- **THEN** no guest-only enrollment client path is included

### Requirement: Guest Package Ships The Client Only

The `devid-enroll` binary package SHALL install the guest enrollment client at
`/usr/bin/devid-enroll` and its man page.

It SHALL NOT install the metadata daemon, the `vtpm-mds` systemd unit, host
configuration under `/etc/vtpm-mds`, or `/var/lib/vtpm-mds`.

Installing `devid-enroll` SHALL NOT enable or start an IMDS listener.

#### Scenario: Guest package contents

- **WHEN** `devid-enroll` is installed on a guest
- **THEN** `/usr/bin/devid-enroll` is present and `/usr/sbin/vtpm-mds` is absent

#### Scenario: Guest install does not start the host daemon

- **WHEN** `devid-enroll` is installed on a machine with no `vtpm-mds` package
- **THEN** no `vtpm-mds` systemd unit is installed and nothing binds
  `169.254.169.1:80` as a result of that install

### Requirement: Packages Are Independently Installable

Neither binary package SHALL depend on the other.

Installing `devid-enroll` SHALL NOT pull in `vtpm-mds`. Installing `vtpm-mds`
SHALL NOT require `devid-enroll`.

#### Scenario: Guest install does not pull the host daemon

- **WHEN** `devid-enroll` is installed with no other packages from this source
- **THEN** `vtpm-mds` is not installed as a dependency

#### Scenario: Host install does not require the guest client

- **WHEN** `vtpm-mds` is installed with no other packages from this source
- **THEN** installation succeeds without `devid-enroll`

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
