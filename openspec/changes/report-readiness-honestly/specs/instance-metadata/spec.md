# Spec Delta

## MODIFIED Requirements

### Requirement: Answer Health Checks Without A Token

The service SHALL register `GET /health` unconditionally and SHALL answer it
with `200`, `Content-Type: application/json`, and a JSON object that reports
readiness without requiring a session token and without depending on
`mds.enable_ec2_compat`. The object SHALL include at least:

- `status`: `"ok"` when the VM inventory is loaded and, if
  `mds.enable_tpm_attestation` is true, the DevID CA certificate and key the
  service would use for enrollment are readable; otherwise `"degraded"`
- `inventory_loaded`: boolean
- `vm_count`: non-negative integer count of VM records in the loaded inventory,
  or `0` when none is loaded
- `devid_ca_readable`: boolean; when `mds.enable_tpm_attestation` is false the
  value SHALL be `true` (CA material is not required)
- `last_reload`: RFC3339 UTC timestamp of the last successful inventory load,
  or an empty string when none has succeeded yet
- `last_error`: a short class string from a closed set (for example
  `inventory_load_failed`, `devid_ca_unreadable`), or an empty string when the
  last successful path left no error

The JSON object SHALL NOT include filesystem paths, MAC addresses, VM
identifiers or names, peer addresses, certificate subjects, or free-form
error text from failed operations. Full errors remain in the journal.

The HTTP status SHALL remain `200` for both `"ok"` and `"degraded"` so that
probes which only check the status code keep working. Requests to `/health`
with any other method SHALL fall through to the catch-all handler and receive
`404`.

#### Scenario: Unauthenticated health check when ready

- **GIVEN** the service has loaded an inventory and DevID CA material is
  readable (or attestation is disabled)
- **WHEN** a caller sends `GET /health` with no session token
- **THEN** the service responds `200` with `Content-Type: application/json`
  and a body whose `status` is `ok` and whose `inventory_loaded` is true

#### Scenario: Health reports degraded after a failed reload

- **GIVEN** the service had a loaded inventory and a later reload failed
- **WHEN** a caller sends `GET /health`
- **THEN** the service responds `200` with `status` `degraded`,
  `inventory_loaded` still true, `vm_count` unchanged from before the failed
  reload, and `last_error` set to a short class string with no path in it

#### Scenario: Health check with the wrong method

- **WHEN** a caller sends `POST /health`
- **THEN** the service responds `404`

#### Scenario: Health body does not leak paths

- **GIVEN** the last inventory load failed because a file was missing
- **WHEN** a caller sends `GET /health`
- **THEN** the JSON body contains no filesystem path and no copy of the
  underlying error string
