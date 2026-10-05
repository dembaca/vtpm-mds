# Tasks

## 1. Reproduce the defect

- [x] 1.1 Add a test with `mds.require_vm_identity` false, no bound VM record and `RemoteAddr` set to `not-an-address`, and verify `GET /latest/meta-data/instance-id` currently returns `200` with the body `i-127-0-0-1` — record that output as the baseline
- [x] 1.2 Repeat 1.1 with `RemoteAddr` set to `[fe80::1]:5000` and verify the current baseline is `i-fe80::1`
- [x] 1.3 Verify the same values reach `instanceId` in the identity document and `sub` in `/latest/identity` for that peer

## 2. Stop inventing an address

- [x] 2.1 Remove `getClientIP` from `imds/handlers.go` and build the fallback id from `LocalIPv4`, and verify with `grep -n 'getClientIP\|127.0.0.1' imds identity` that no non-test fallback literal remains
- [x] 2.2 Make `InstanceID` return the empty string when no record is bound and the peer has no IPv4 address, and verify a unit test covers unparseable, empty, port-only and `[fe80::1]:5000` `RemoteAddr` values alongside `10.0.0.5:1234` and `[::ffff:10.0.0.5]:5000`

## 3. Answer the unavailable case

- [x] 3.1 Make `HandleInstanceID` respond `422` with an empty body and `Content-Type: text/plain` when `InstanceID` is empty, through one helper shared by the three handlers, and verify the task 1.1 and 1.2 tests now expect and get that
- [x] 3.2 Do the same in `HandleInstanceIdentityDocument` and in the `/latest/identity` handler in `identity/handlers.go`, and verify neither emits a document or token with an empty or `i-` subject
- [x] 3.3 Verify `GET /latest/dynamic/instance-identity/signature` still serves `dGVzdC1zaWduYXR1cmU=` to the same caller

## 4. Confirm nothing else moved

- [x] 4.1 Verify a caller from `127.0.0.1` with `require_vm_identity` false still receives `i-127-0-0-1`, and a bound caller still receives `i-<vmid>`
- [x] 4.2 Verify the default configuration still refuses an unbound caller with `404 page not found`, unchanged, and that `local-ipv4`, `local-hostname`, `public-ipv4` and the index are byte-for-byte unchanged
- [x] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [x] 4.4 Run `openspec validate --all --strict` and verify it passes
