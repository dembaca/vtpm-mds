# Spec Delta

## ADDED Requirements

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
