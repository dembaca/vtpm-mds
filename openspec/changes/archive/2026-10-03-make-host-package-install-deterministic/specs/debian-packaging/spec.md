# Spec Delta

## ADDED Requirements

### Requirement: Installing The Host Package Leaves systemd In Step

Installing or upgrading the host package SHALL leave the systemd manager
holding the unit definition the package just installed. The package's
maintainer scripts SHALL reload the manager after the unit file is in place,
so that no manual `systemctl daemon-reload` is needed for the installed unit
to be the effective one.

The package SHALL NOT start, restart or stop the daemon on installation. The
daemon binds a link-local address that is absent on build hosts and on lab
hosts without the IMDS bridge, so starting it from `postinst` would fail
there; whether the service runs stays an operator decision.

A reload SHALL be attempted only where a systemd manager is running, and a
failure to reload SHALL NOT fail the installation.

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
