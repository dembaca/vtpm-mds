# Tasks

## 1. Reproduce both defects

- [x] 1.1 Start the daemon with a config file that sets only `listen_addr` and verify a metadata read with a valid token returns `404` while `GET /health` returns `{"status":"ok"}` — the baseline for the defaults defect. Observed on the pre-fix binary: `GET /health` → `200 {"status":"ok"}`; `PUT /latest/api/token` → `200` with a token; `GET /latest/meta-data/instance-id` with that token → `404 page not found`.
- [x] 1.2 Start the daemon with `token_ttl: 60` (no unit) and verify `PUT /latest/api/token` returns `200` and the immediately following metadata read returns `401` — the baseline for the TTL defect. Observed on the pre-fix binary (with `enable_ec2_compat: true` set explicitly, to isolate this defect from 1.1's): `PUT /latest/api/token` → `200` with a token; the immediately following `GET /latest/meta-data/instance-id` with that token → `401 IMDSv2 token required`.

## 2. Merge the built-in defaults

- [x] 2.1 Change `config.Load` to unmarshal onto `DefaultConfig()` instead of a zero value, and verify a file that sets only `listen_addr` yields `EnableEC2Compat` true and `TokenTTL` `60s`. Verified by `TestLoadMergesDefaultsForOmittedFields` (`internal/config/config_test.go`) — PASS.
- [x] 2.2 Verify an explicit `enable_ec2_compat: false` still yields false, so an operator can state a zero value — add this as a test case alongside 2.1. Verified by `TestLoadValidFile` — PASS. (Note: this existing test already stated `enable_ec2_compat: false` explicitly, not by omission — see 2.3.)
- [x] 2.3 Replace the assertion in `internal/config/config_test.go` that expects `false` from an omitted `enable_ec2_compat`, and verify `go test ./internal/config/...` passes. On inspection the pre-existing `TestLoadValidFile` stated `enable_ec2_compat: false` explicitly rather than omitting it, so it did not actually encode the omission defect; kept it (now commented to say so) as the explicit-false case for 2.2, and added `TestLoadMergesDefaultsForOmittedFields` as the omission case the proposal describes. `go test ./internal/config/...` → PASS (see full output below).
- [x] 2.4 Verify `listen_addr: ""` still fails the load with `listen_addr is required`, and that an omitted `listen_addr` now yields the built-in address. Verified by `TestLoadEmptyListenAddrIsRejected` and `TestLoadOmittedListenAddrTakesDefault` — both PASS.

## 3. Reject an unusable token TTL

- [x] 3.1 Add validation of `token_ttl` to `config.Load`, returning an error naming the setting and the offending value, and verify `Load` fails for `60` and for `sixty` and succeeds for `60s`. Verified by `TestLoadRejectsUnparseableTokenTTL` (subtests `60` and `sixty`) and `TestLoadAcceptsValidTokenTTL` — all PASS.
- [x] 3.2 Verify an omitted `token_ttl` passes validation and yields `60s`, since the defaults merge supplies it. Verified by `TestLoadOmittedTokenTTLPassesValidationAndTakesDefault` — PASS.
- [x] 3.3 Verify the daemon exits non-zero and never binds its listen address when started with a config whose `token_ttl` is `sixty`, and that the log line names `token_ttl`. Verified by running the rebuilt binary against a config with `token_ttl: "sixty"`: process exited with status 1, log line `Failed to load config: invalid token_ttl "sixty": time: invalid duration "sixty"`, and `ss -ltn` showed nothing listening on the configured port. See full transcript below.

## 4. Confirm the observable behaviour

- [x] 4.1 Repeat task 1.1 with the new binary and verify the metadata read now returns `200`. Verified: `GET /health` → `200`, `PUT /latest/api/token` → `200`, `GET /latest/meta-data/instance-id` with that token → `200 i-127-0-0-1` (previously `404`).
- [x] 4.2 Verify `PUT /latest/api/token` followed immediately by a metadata read returns `200` with the shipped `/etc/vtpm-mds/config.yaml`, confirming the packaged configuration is unaffected. `/etc/vtpm-mds/config.yaml` on this host, and `debian/config.yaml` in the tree, are byte-for-byte identical in `mds:` content and state every setting explicitly (including `token_ttl: "60s"` and `enable_ec2_compat: true`). Could not bind to the real `169.254.169.1:80` — that address is already held by the live systemd `vtpm-mds` service and binding it needs no elevated privilege but the address itself is taken; there is no passwordless sudo to stop/restart that unit either. Verified instead with a copy of the shipped file with only `listen_addr` swapped to a free local port, every other setting unchanged: `PUT /latest/api/token` → `200`, immediately followed by `GET /latest/meta-data/instance-id` → `200 i-127-0-0-1`.
- [x] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output. See output below.
