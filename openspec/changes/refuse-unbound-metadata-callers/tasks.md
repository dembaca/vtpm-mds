# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Add a test that requests `/latest/meta-data/instance-id` from an unbound caller with `X-Forwarded-For: 10.9.9.9` and verify the current response is `200` with the body `i-10-9-9-9` — record that as the baseline
- [ ] 1.2 Add a test that requests `/latest/identity` as a caller bound to VM `100` with `X-Forwarded-For: 10.9.9.9` and verify the current claims report `i-10-9-9-9` rather than `i-100`, confirming that endpoint ignores the inventory entirely

## 2. Add the setting

- [ ] 2.1 Add `RequireVMIdentity` to `internal/config` with the yaml key `require_vm_identity` and a built-in default of true, and verify `DefaultConfig()` reports true
- [ ] 2.2 Verify a configuration file that does not mention `require_vm_identity` loads it as true — this pins the ordering dependency on `fail-fast-on-unusable-configuration` and fails loudly if the defaults merge is ever lost
- [ ] 2.3 Verify an explicit `require_vm_identity: false` loads as false
- [ ] 2.4 Document the setting in `config.yaml` and in `debian/vtpm-mds.8`, describing it as a migration aid, and verify the man page still parses with `groff -man -ww -z`

## 3. Refuse unbound callers

- [ ] 3.1 Wrap exactly the four identity-bearing route registrations in `internal/server/server.go` — `instance-id`, the instance identity document, its signature and `/latest/identity` — with a check that refuses a request whose connection has no bound VM record, and verify no other route is wrapped
- [ ] 3.2 Verify the refusal runs after token validation: an unbound caller with no token gets `401` with `IMDSv2 token required`, and with a valid token gets `404`
- [ ] 3.3 Verify the refused response body is exactly `404 page not found`, identical to a request for an unregistered route
- [ ] 3.4 Verify all four refused paths return `404` for an unbound caller: `/latest/meta-data/instance-id`, `/latest/dynamic/instance-identity/document`, `/latest/dynamic/instance-identity/signature` and `/latest/identity`
- [ ] 3.5 Verify the paths that are deliberately not refused still return `200` to an unbound caller — `/latest/meta-data/`, `local-hostname`, `local-ipv4`, `placement/availability-zone`, `services/domain` — with a table-driven test that pins both sides of the line, so a handler added to the wrong side fails rather than ships
- [ ] 3.6 Verify `PUT /latest/api/token` and `GET /health` are served to an unbound caller
- [ ] 3.7 Verify a bound caller is served every metadata value exactly as before, by running the existing `imds` handler tests unchanged

## 4. Stop headers from naming the caller

- [ ] 4.1 Remove the `X-Forwarded-For` branch from `getClientIP` in `imds/handlers.go` and from its copy in `identity/handlers.go`, and verify `grep -rn 'X-Forwarded-For' imds identity` returns nothing outside tests
- [ ] 4.2 Replace `identity.getInstanceID` with the same inventory-backed derivation the metadata handler uses, and verify the task 1.2 test now reports `i-100`
- [ ] 4.3 Verify the fallback path with `require_vm_identity: false`: an unbound caller from `127.0.0.1` sending `X-Forwarded-For: 10.9.9.9` receives `i-127-0-0-1`
- [ ] 4.4 Verify the `ip` claim of the identity document is the peer address and is the empty string when the peer has no IPv4 address

## 5. Make the refusal diagnosable

- [ ] 5.1 Log one line per refused request naming the caller's peer address, and verify it appears once for a refused metadata read and identifies the address

## 6. Confirm end to end

- [ ] 6.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [ ] 6.2 Run `scripts/qemu-lab/e2e-netns.sh` and verify it still reports `i-100`, since its netns MAC is in the lab inventory — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
- [ ] 6.3 Verify an unbound caller is refused on the running service by requesting the instance id from a netns whose MAC is not in the inventory and confirming `404` plus the log line from task 5.1, and that the same caller still gets `200` on `local-ipv4` — needs root, record who ran it
- [ ] 6.4 Verify DevID enrollment still refuses an unbound caller with `401` and `VM identity required (MAC not in inventory)`, so this change did not flatten the two refusals into one
