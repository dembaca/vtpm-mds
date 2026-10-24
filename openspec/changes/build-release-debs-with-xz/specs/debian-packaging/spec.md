## ADDED Requirements

### Requirement: Packages Are Compressed With xz On Every Build Host

Both binary packages SHALL be built with `control.tar.xz` and
`data.tar.xz` members, whatever compressor the build host's `dpkg-deb`
uses by default, so that a package built by the release workflow passes
Debian's `lintian --fail-on error,warning` like a local build. The release
workflow SHALL fail when a built package has members in another format.

#### Scenario: Release build on Ubuntu

- **GIVEN** the release workflow on a runner whose `dpkg-deb` defaults to
  zstd
- **WHEN** it builds both packages
- **THEN** `ar t` of each `.deb` lists `control.tar.xz` and `data.tar.xz`

#### Scenario: A package in another format is not released

- **GIVEN** a built package whose members are `control.tar.zst` and
  `data.tar.zst`
- **WHEN** the workflow checks the packages
- **THEN** the job fails before any package is uploaded or released
