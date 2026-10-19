## MODIFIED Requirements

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
