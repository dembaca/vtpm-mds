# Design

## Context

Assigned by the coordinator on 2026-10-19 (`ASSIGNMENTS` after #59 archived
throttle): `@agent-cursor-vtpm-22e0` records `report-readiness-honestly`. Branch
from `main@364bffe`. Lab owns the YAML half; no `debian/`.

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

## Decided by this record's author, for the maintainer to overrule

- **D-a. Enrich `GET /health` in place; do not add a second path.** Guests and
  host probes already use `/health`. A parallel `/ready` would split the
  contract and invite the old endpoint to keep lying. Same path, richer body.
- **D-b. Always HTTP `200`; readiness is in `"status"`.** `"ok"` when inventory
  is loaded and (if `enable_tpm_attestation`) the DevID CA cert (and key, via
  credential or configured path) is readable; otherwise `"degraded"`. Probes
  that only check the status code do not flap; honest detail is in the JSON.
- **D-c. Guest-safe fields only.** The body MAY contain:
  - `status` (`ok` | `degraded`)
  - `inventory_loaded` (bool)
  - `vm_count` (non-negative int; `0` when not loaded)
  - `devid_ca_readable` (bool; `false` when attestation is disabled counts as
    not applicable → report `true` so it does not force degraded, or omit —
    **decision: when attestation is disabled, set `devid_ca_readable` to
    `true` and do not require CA files**)
  - `last_reload` (RFC3339 UTC string of the last successful inventory load,
    or `""` if none yet)
  - `last_error` (short class string from a fixed set, or `""`)
  It MUST NOT contain filesystem paths, MAC addresses, VM ids/names, peer
  addresses, certificate subjects, or free-form `error.Error()` text.
- **D-d. `last_error` classes (closed set).** At least:
  `inventory_load_failed`, `config_invalid`, `devid_ca_unreadable`, `""`.
  Implementation may add further classes in the same style; unknown classes
  MUST NOT embed operator paths. The journal still gets the full error.
- **D-e. Strict YAML via `yaml.Decoder` + `KnownFields(true)`** for
  `config.Load` and `inventory.LoadYAML`. Unknown keys and mistyped field names
  fail the decode. Start fails if config decode fails. Inventory load/reload
  fails if inventory decode fails; reload keeps the previous cache (already
  true) and sets readiness `last_error` / `degraded` as appropriate.
- **D-f. Track readiness on the `Server`.** Successful `RefreshVMConfigCache`
  updates `vm_count`, `inventory_loaded`, `last_reload`, clears `last_error`.
  Failed refresh leaves the cache alone and records `last_error`. DevID CA
  readability is checked when building the health response (stat/open the
  effective cert/key paths), not cached across hours without re-check.
- **D-g. Lab verifies the YAML half.** Cloud lab with `inventory_path`: good
  file → `/health` shows `ok` and `vm_count`; broken unknown-key file on
  SIGHUP → reload rejected, previous `vm_count` unchanged, `last_error` set,
  `status` degraded. Config unknown-key → process refuses to start (unit test
  and/or lab). Hogan/Proxmox half is not required here.

## Non-Goals

- **Path unit / inotify on `/etc/pve`** — `reload-when-guest-configs-change`.
- **postinst restart on upgrade** — `restart-the-service-on-upgrade`.
- **Changing who may call `/health`** (still any peer; still throttle-exempt).
- **Exposing per-VM or per-peer diagnostics** on `/health`.
- **Editing `AGENTS.md` or the root `README.md`** (coordinator archives).

## Risks

- **BREAKING for configs/inventories with unknown keys** that today load. That
  is intended; operators fix the file.
- **Clients that require an exact body** `{"status":"ok"}\n` break on extra
  fields. JSON object extension is the migration; document in the delta.
- **`devid_ca_readable` false while enrollment is intentionally off** is
  avoided by D-c (attestation disabled → treat as readable/not required).
