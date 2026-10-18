# Spec Delta

## ADDED Requirements

### Requirement: Prefer The CA Key From A systemd Credential

When the environment variable `CREDENTIALS_DIRECTORY` names a directory that
holds a file `devid_ca.key`, the service SHALL load the DevID CA private key from
that file instead of the configured `devid_ca_key`, and SHALL log once that the
key came from a systemd credential. Without such a file the configured path SHALL
be used as before.

#### Scenario: A credential replaces the configured path

- **GIVEN** `CREDENTIALS_DIRECTORY` holding `devid_ca.key` and a `devid_ca_key`
  that points elsewhere
- **WHEN** the service starts
- **THEN** it loads the key from the credential and logs that it did

#### Scenario: No credential keeps the configured path

- **GIVEN** no `CREDENTIALS_DIRECTORY`, or one without `devid_ca.key`
- **WHEN** the service starts
- **THEN** it loads the key from `devid_ca_key`, as before
