# Spec Delta

## Purpose

Let an operator determine which build of the daemon and of the guest client is
installed, without starting the service or reaching a TPM, and make the host
service's restart rate limit effective.

## ADDED Requirements

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

## Notes

Man pages `vtpm-mds(8)` and `devid-enroll(1)` document the flag; that is
packaging, covered by the `debian-packaging` capability's requirement that each
package ships its own man page.
