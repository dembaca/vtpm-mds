# Tasks

## 1. Strict YAML

- [ ] 1.1 `config.Load` decodes with unknown fields rejected (`KnownFields`)
      **Verify:** unit test: file with `mds.listen_addres` → error; good file
      still loads defaults overlay. `go test ./internal/config`
- [ ] 1.2 `inventory.LoadYAML` decodes with unknown fields rejected
      **Verify:** unit test: VM entry with unknown key → error; existing
      fixtures still load. `go test ./internal/inventory`

## 2. Readiness on `/health`

- [ ] 2.1 Track last successful reload time, vm count, inventory loaded, and
      last error class on the server; update on `RefreshVMConfigCache`
      success/failure (failed refresh MUST NOT clear the cache)
      **Verify:** unit test around refresh success then failing reload
- [ ] 2.2 `GET /health` returns the JSON object from the instance-metadata
      delta (`status`, `inventory_loaded`, `vm_count`, `devid_ca_readable`,
      `last_reload`, `last_error`); no paths in the body; HTTP 200 for both
      `ok` and `degraded`
      **Verify:** `go test ./internal/server` (and/or `./imds`) covering ready,
      degraded-after-failed-reload, and attestation-disabled →
      `devid_ca_readable` true
- [ ] 2.3 Update any tests that asserted the exact body `{"status":"ok"}`
      **Verify:** `go test ./...`

## 3. Specs and validate

- [ ] 3.1 `openspec validate --all --strict` clean for this change
      **Verify:** CLI output shows the change passed

## 4. Lab (YAML half)

- [ ] 4.1 Cloud lab with `inventory_path`: start on a good lab YAML →
      `GET /health` shows `status=ok`, `inventory_loaded=true`, `vm_count` ≥ 1
      **Verify:** paste the JSON body and commit SHA in the RESULT
- [ ] 4.2 Replace inventory with a file that has an unknown key; SIGHUP →
      reload fails; previous `vm_count` retained; `status=degraded`;
      `last_error` set; bound guest still gets `instance-id`
      **Verify:** health JSON + one guest `instance-id` 200
- [ ] 4.3 Say what was not verified: Proxmox inventory path, hogan, path-unit
      reload, postinst restart

## 5. Archive

- [ ] 5.1 After merge, archive with the coordinator (no `AGENTS.md` / root
      `README.md` edits from this agent)
