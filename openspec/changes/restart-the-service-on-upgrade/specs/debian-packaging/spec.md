## MODIFIED Requirements

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

A reload or restart SHALL be attempted only where a systemd manager is
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
