# Spec Delta

## MODIFIED Requirements

### Requirement: Answer Health Checks Without A Token

The service SHALL register `GET /health` unconditionally and SHALL answer it
with `200`, `Content-Type: application/json`, and a JSON object that reports
readiness without requiring a session token and without depending on
`mds.enable_ec2_compat`.

Every caller SHALL receive a `status` field whose value is `"ok"` when the VM
inventory is loaded and, if `mds.enable_tpm_attestation` is true, the DevID CA
certificate and key the service would use for enrollment are readable;
otherwise `"degraded"`. The HTTP status SHALL remain `200` for both values.

When the caller is **bound to a VM record**, the body SHALL contain only
`status` (no inventory counts, timestamps, CA flags, or error classes).

When the caller is **bound to no VM record**, the body SHALL also include:

- `inventory_loaded`: boolean
- `vm_count`: non-negative integer count of VM records in the loaded inventory,
  or `0` when none is loaded
- `last_reload`: RFC3339 UTC timestamp of the last successful inventory load,
  or an empty string when none has succeeded yet
- `last_error`: a short class string from a closed set (for example
  `inventory_load_failed`, `devid_ca_unreadable`), or an empty string when the
  last successful path left no error
- `devid_ca_readable`: boolean, **only when** `mds.enable_tpm_attestation` is
  true; when attestation is disabled this field SHALL be omitted

The JSON object SHALL NOT include filesystem paths, MAC addresses, VM
identifiers or names, peer addresses, certificate subjects, or free-form
error text from failed operations. Full errors remain in the journal.

Requests to `/health` with any other method SHALL fall through to the catch-all
handler and receive `404`.

#### Scenario: Unauthenticated health check

- **GIVEN** the service has loaded an inventory and is ready
- **WHEN** a caller bound to no VM sends `GET /health` with no session token
- **THEN** the service responds `200` with `Content-Type: application/json`
  and a body whose `status` is `ok` and whose `inventory_loaded` is true

#### Scenario: Health check with the wrong method

- **WHEN** a caller sends `POST /health`
- **THEN** the service responds `404`

#### Scenario: Bound guest sees only status

- **GIVEN** a caller bound to a VM record
- **WHEN** it sends `GET /health`
- **THEN** the JSON body contains only the `status` field

#### Scenario: Health reports degraded after a failed reload

- **GIVEN** the service had a loaded inventory and a later reload failed
- **WHEN** an unbound caller sends `GET /health`
- **THEN** the service responds `200` with `status` `degraded`,
  `inventory_loaded` still true, `vm_count` unchanged from before the failed
  reload, and `last_error` set to a short class string with no path in it

#### Scenario: Health body does not leak paths

- **GIVEN** the last inventory load failed because a file was missing
- **WHEN** an unbound caller sends `GET /health`
- **THEN** the JSON body contains no filesystem path and no copy of the
  underlying error string

#### Scenario: Attestation off omits devid_ca_readable

- **GIVEN** `mds.enable_tpm_attestation` is false and the inventory is loaded
- **WHEN** an unbound caller sends `GET /health`
- **THEN** the body has `status` `ok` and does not contain the key
  `devid_ca_readable`
