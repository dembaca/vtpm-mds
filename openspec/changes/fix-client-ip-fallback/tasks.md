# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Add a test with `mds.require_vm_identity` false, no bound VM record and `RemoteAddr` set to `not-an-address`, and verify `GET /latest/meta-data/instance-id` currently returns `200` with the body `i-127-0-0-1` — record that output as the baseline
- [ ] 1.2 Verify the same value reaches `instanceId` in the identity document and `sub` in `/latest/identity` for that peer

## 2. Stop inventing an address

- [ ] 2.1 Make `getClientIP` in `imds/handlers.go` report no address instead of returning `127.0.0.1`, and verify with `grep -n '127.0.0.1' imds identity` that no non-test fallback literal remains
- [ ] 2.2 Make `InstanceID` return the empty string when no record is bound and no peer address parses, and verify a unit test covers unparseable, empty and port-only `RemoteAddr` values alongside `10.0.0.5:1234`

## 3. Answer the unavailable case

- [ ] 3.1 Make `HandleInstanceID` respond `404` with the body `404 page not found` when `InstanceID` is empty, reusing the refusal's response rather than a copy, and verify the task 1.1 test now expects and gets that
- [ ] 3.2 Do the same in `HandleInstanceIdentityDocument` and in the `/latest/identity` handler in `identity/handlers.go`, and verify neither emits a document or token with an empty or `i-` subject
- [ ] 3.3 Verify `GET /latest/dynamic/instance-identity/signature` still serves `dGVzdC1zaWduYXR1cmU=` to the same caller

## 4. Confirm nothing else moved

- [ ] 4.1 Verify a caller from `127.0.0.1` with `require_vm_identity` false still receives `i-127-0-0-1`, and a bound caller still receives `i-<vmid>`
- [ ] 4.2 Verify the default configuration still refuses an unbound caller with `404 page not found` and that `local-ipv4`, `local-hostname`, `public-ipv4` and the index are byte-for-byte unchanged
- [ ] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [ ] 4.4 Run `openspec validate --all --strict` and verify it passes
