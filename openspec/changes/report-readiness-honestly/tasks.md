# Tasks

## 1. Strict YAML

- [x] 1.1 `config.Load` decodes with unknown fields rejected (`KnownFields`)
      **Verify:** unit test: file with `mds.listen_addres` → error; good file
      still loads defaults overlay. `go test ./internal/config`
- [x] 1.2 `inventory.LoadYAML` decodes with unknown fields rejected
      **Verify:** unit test: VM entry with unknown key → error; existing
      fixtures still load. `go test ./internal/inventory`
- [x] 1.3 Strict-decode the config **shipped** in `debian/config.yaml` (and
      root `config.yaml` if present as the example operators copy)
      **Verify:** unit test in `internal/config` that `Load`s those files
      successfully under KnownFields

## 2. Readiness on `/health`

- [x] 2.1 Track last successful reload time, vm count, inventory loaded, and
      last error class on the server; update on `RefreshVMConfigCache`
      success/failure (failed refresh MUST NOT clear the cache)
      **Verify:** unit test around refresh success then failing reload
- [x] 2.2 `GET /health`: bound guest → only `{"status":…}`; unbound → full
      detail per delta; omit `devid_ca_readable` when attestation is off;
      cache CA readability ~5 s (D-f); HTTP 200 for ok and degraded; no paths
      **Verify:** `go test ./internal/server` covering bound vs unbound, ready,
      degraded-after-failed-reload, attestation-off omits field
- [x] 2.3 Update any tests that asserted the exact body `{"status":"ok"}`
      **Verify:** `go test ./...`

## 3. Specs and validate

- [x] 3.1 `openspec validate --all --strict` clean for this change
      **Verify:** CLI output shows the change passed

## 4. Lab (YAML half)

- [x] 4.1 Cloud lab with `inventory_path`: unbound `GET /health` shows
      `status=ok`, `inventory_loaded=true`, `vm_count` ≥ 1; bound guest
      (guest100) gets only `status`
      **Verify:** paste both JSON bodies and commit SHA in the RESULT
- [x] 4.2 Replace inventory with a file that has an unknown key; SIGHUP →
      reload fails; previous `vm_count` retained; unbound `status=degraded`;
      bound guest still gets `instance-id`
      **Verify:** health JSON + one guest `instance-id` 200
      **Lab note:** unbound degraded + retained `vm_count` measured; bound
      guest `instance-id` during degraded not re-checked (ssh); covered by
      `TestHealthFailedReloadKeepsCount`.
- [x] 4.3 Say what was not verified: Proxmox inventory path, path-unit reload,
      postinst restart, removing dead config keys
      **Lab, 2026-10-19** (build-61): unbound detail ok/degraded as designed;
      bound guest `{"status":"ok"}` only; bad SIGHUP keeps vm_count=1. Guest
      instance-id during degraded not re-checked (ssh); unit test covers keep.

## 5. Hogan (strict config before upgrade)

- [ ] 5.1 On hogan, **before** installing the new package: confirm the live
      `/etc/…/config.yaml` strict-decodes with this change's `config.Load`
      (or an equivalent decode of the same schema). If it fails, stop and
      report — do not install.
      **Verify:** command output + path; then install only if green

## 6. Archive

- [ ] 6.1 After merge, archive with the coordinator (no `AGENTS.md` / root
      `README.md` edits from this agent)
