# Proposal

## Why

`GET /health` answers `{"status":"ok"}` whenever the HTTP listener is up, even
when the inventory never loaded, the DevID CA cannot be read, or the last
reload failed. Operators and probes cannot tell readiness from "the port is
open". At the same time, YAML for the service config and the lab inventory is
decoded with a permissive unmarshal: an unknown key or a typo is silently
ignored, so a mistyped field looks like a healthy default.

## What Changes

- **`GET /health` reports readiness detail** that is safe for any caller on the
  IMDS bridge: whether inventory is loaded, how many VMs it holds, whether the
  DevID CA material is readable when attestation is enabled, when the inventory
  last reloaded successfully, and a short last-error class when the last load
  or reload failed. It does not expose paths, MACs, VM names, peer addresses,
  or raw error strings.
- **A non-ready service still answers `200`**, with `"status":"degraded"` (or
  `"ok"` when ready), so existing probes that only check the status code keep
  working while the body tells the truth.
- **BREAKING** Service config YAML and YAML inventory are decoded with unknown
  fields rejected. A typo or unknown key fails start (config) or fails the load
  / reload (inventory); a failed reload keeps the previously loaded inventory.
- Lab verification covers the YAML inventory half; Proxmox-path reload is out
  of this change (`reload-when-guest-configs-change`).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `instance-metadata`: the health-check requirement changes from a fixed
  `{"status":"ok"}` body to a readiness object with the fields above.
- `operability`: new requirements for strict YAML decoding of the service
  config and of the YAML inventory, and for keeping the previous inventory on
  a failed reload.

## Impact

- **Code, after the coordinator's ACK:** `internal/config/` (strict decode),
  `internal/inventory/` (strict decode for YAML), `/health` handler and
  readiness state in `internal/server/`, unit tests. No `debian/`, no unit
  file, no `AGENTS.md` / root `README.md`.
- **Callers:** any client that parsed `/health` as exactly `{"status":"ok"}`
  must accept additional JSON fields; the HTTP status stays `200`.
- **Operators:** a config or inventory file that today carries unknown keys
  will fail to load after this change — that is the point.
- **Not in scope:** path-unit / inotify reload of `/etc/pve` (separate
  assignment), postinst restart on upgrade, spoofing, CA operation decisions.
