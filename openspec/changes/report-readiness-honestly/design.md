# Design

## Context

Assigned by the coordinator on 2026-10-19 (`ASSIGNMENTS` after #59 archived
throttle): `@agent-cursor-vtpm-22e0` records `report-readiness-honestly`. Branch
from `main@364bffe`. Lab owns the YAML half; no `debian/` edits in the product
sense (a test may *read* `debian/config.yaml`).

Coordinator ACK on the record (2026-10-19) required D-h, a D-c correction, D-f
caching, and strict-YAML upgrade interplay with #60 — incorporated below.

### Evidence read in this repository (2026-10-19, `main@364bffe`)

- `GET /health` in `internal/server/server.go` always encodes
  `map[string]string{"status":"ok"}` and never consults inventory or DevID.
- `openspec/specs/instance-metadata/spec.md` "Answer Health Checks Without A
  Token" requires that fixed body and says health MUST NOT depend on inventory
  or TPM — that contract is what this change replaces.
- `config.Load` and `inventory.LoadYAML` use `yaml.Unmarshal`, which ignores
  unknown fields. `public_ipv4` typos already fail the inventory load
  (fail-fast); unknown keys do not.
- `Server.RefreshVMConfigCache` returns on error before assigning
  `vmConfigCache`, so a failed SIGHUP reload already keeps the previous map.
  Nothing records that failure for `/health` to surface.
- Config keys present in `MDSConfig` whose fields are unread by non-test code
  after identity removal: `jwks_path`, `attestation_ca`, `jwt_ttl`,
  `tpm_device`, `host_tpm_device`, `signing_key_path`. They remain in the
  schema so shipped configs keep decoding; removing them is a separate
  "dead config keys" change.

## Decided by this record's author, for the maintainer to overrule

- **D-a. Enrich `GET /health` in place; do not add a second path.** Guests and
  host probes already use `/health`. A parallel `/ready` would split the
  contract and invite the old endpoint to keep lying. Same path, richer body
  for some callers (see D-h).
- **D-b. Always HTTP `200`; readiness is in `"status"`.** `"ok"` when inventory
  is loaded and, if `enable_tpm_attestation` is true, the DevID CA cert and key
  are readable; otherwise `"degraded"`. Probes that only check the status code
  do not flap.
- **D-c. Detail fields (unbound callers only — D-h).** When detail is served,
  the body MAY contain:
  - `status` (`ok` | `degraded`) — also the only field for bound guests
  - `inventory_loaded` (bool)
  - `vm_count` (non-negative int; `0` when not loaded)
  - `devid_ca_readable` (bool) — **only when `enable_tpm_attestation` is
    true**; when attestation is off the field is **omitted** (not `true`)
  - `last_reload` (RFC3339 UTC of last successful inventory load, or `""`)
  - `last_error` (short class from a closed set, or `""`)
  It MUST NOT contain filesystem paths, MAC addresses, VM ids/names, peer
  addresses, certificate subjects, or free-form `error.Error()` text.
- **D-d. `last_error` classes (closed set).** At least:
  `inventory_load_failed`, `config_invalid`, `devid_ca_unreadable`, `""`.
  The journal still gets the full error.
- **D-e. Strict YAML via `yaml.Decoder` + `KnownFields(true)`** for
  `config.Load` and `inventory.LoadYAML`. Unknown keys fail decode. Start
  fails if config decode fails. Inventory reload failure keeps the previous
  cache and sets readiness accordingly.
- **D-f. Track readiness on the `Server`; cache CA readability ~5 s.**
  Successful `RefreshVMConfigCache` updates counts / `last_reload` / clears
  `last_error`. Failed refresh leaves the cache alone and records
  `last_error`. DevID CA readability is checked for health responses but the
  result is cached for about five seconds because `/health` is throttle-exempt.
- **D-g. Lab verifies the YAML half.** Good inventory → unbound `/health`
  detail `ok`; unknown-key SIGHUP → previous `vm_count`, `degraded`. Bound
  guest sees only `status`. Hogan checks the installed config file against
  strict decode *before* installing the new package (task).
- **D-h. Detail only for callers bound to no VM.** A caller the inventory
  binds to a VM record (a guest) gets only `{"status":…}`. An unbound caller
  (the host on hogan / lab) gets the full detail object. `/health` stays
  reachable by every peer and throttle-exempt. **Open for @dembaca:** whether
  this split is the lasting policy, or detail should move off the guest path
  entirely later.

## Non-Goals

- **Path unit / inotify on `/etc/pve`** — `reload-when-guest-configs-change`.
- **postinst restart on upgrade** — `restart-the-service-on-upgrade` (#60).
- **Removing dead config keys** from `MDSConfig` / shipped YAML (follow-up).
- **Editing `AGENTS.md` or the root `README.md`** (coordinator archives).

## Risks

- **BREAKING for configs/inventories with unknown keys** that today load.
- **Interplay with #60 (restart on upgrade):** a node whose
  `/etc/…/config.yaml` still carries an unknown or removed key will fail to
  start after upgrade once this change ships. Mitigation in this change: (1)
  unit test that strictly decodes `debian/config.yaml` (and root
  `config.yaml` if present as example), (2) hogan task to strict-decode the
  live installed file *before* installing the new package, (3) dead keys
  listed above stay in the schema until a dedicated removal change.
- **Clients that require an exact body** `{"status":"ok"}` — bound guests
  still get a one-field object; unbound callers see more fields.
- **D-h is maintainer-open:** if dembaca prefers no detail on the IMDS
  address at all, a follow-up can move detail to a host-only listener.
