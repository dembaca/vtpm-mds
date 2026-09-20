# Tasks

## 1. Reproduce both defects

- [ ] 1.1 Start the daemon with a config file that sets only `listen_addr` and verify a metadata read with a valid token returns `404` while `GET /health` returns `{"status":"ok"}` — the baseline for the defaults defect
- [ ] 1.2 Start the daemon with `token_ttl: 60` (no unit) and verify `PUT /latest/api/token` returns `200` and the immediately following metadata read returns `401` — the baseline for the TTL defect

## 2. Merge the built-in defaults

- [ ] 2.1 Change `config.Load` to unmarshal onto `DefaultConfig()` instead of a zero value, and verify a file that sets only `listen_addr` yields `EnableEC2Compat` true and `TokenTTL` `60s`
- [ ] 2.2 Verify an explicit `enable_ec2_compat: false` still yields false, so an operator can state a zero value — add this as a test case alongside 2.1
- [ ] 2.3 Replace the assertion in `internal/config/config_test.go` that expects `false` from an omitted `enable_ec2_compat`, and verify `go test ./internal/config/...` passes
- [ ] 2.4 Verify `listen_addr: ""` still fails the load with `listen_addr is required`, and that an omitted `listen_addr` now yields the built-in address

## 3. Reject an unusable token TTL

- [ ] 3.1 Add validation of `token_ttl` to `config.Load`, returning an error naming the setting and the offending value, and verify `Load` fails for `60` and for `sixty` and succeeds for `60s`
- [ ] 3.2 Verify an omitted `token_ttl` passes validation and yields `60s`, since the defaults merge supplies it
- [ ] 3.3 Verify the daemon exits non-zero and never binds its listen address when started with a config whose `token_ttl` is `sixty`, and that the log line names `token_ttl`

## 4. Confirm the observable behaviour

- [ ] 4.1 Repeat task 1.1 with the new binary and verify the metadata read now returns `200`
- [ ] 4.2 Verify `PUT /latest/api/token` followed immediately by a metadata read returns `200` with the shipped `/etc/vtpm-mds/config.yaml`, confirming the packaged configuration is unaffected
- [ ] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
