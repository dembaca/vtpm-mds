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
      **Second half done on the real stack, first half is not reachable there.**
      2026-10-16, installed daemon `0.2.0+git20261005.5a60c877cd9c` on `hogan`:
      from VM 399, `GET /latest/meta-data/instance-id` → `200 'i-399'`, so a
      bound caller still receives `i-<vmid>`.
      The `127.0.0.1` half **cannot** be produced here and is covered by unit
      tests only: the daemon listens on `169.254.169.1:80` alone, so a
      connection from the host arrives with peer `169.254.169.1`, never
      `127.0.0.1`; and `mds.require_vm_identity` is absent from
      `/etc/vtpm-mds/config.yaml`, which this user cannot write. Saying so
      beats implying the opt-out path was exercised against the package.
- [x] 4.2 Verify the default configuration still refuses an unbound caller with `404 page not found`, unchanged, and that `local-ipv4`, `local-hostname`, `public-ipv4` and the index are byte-for-byte unchanged
      **Done on the real stack**, 2026-10-16, installed
      `0.2.0+git20261005.5a60c877cd9c`, `/etc/vtpm-mds/config.yaml` carrying no
      `require_vm_identity` key so the built-in default of true applies.
      Unbound caller — the host itself, which has no ARP entry for its own
      address: `PUT /latest/api/token` → `200` (the token endpoint is not
      identity-bearing), then `GET /latest/meta-data/instance-id` → `404`,
      `text/plain; charset=utf-8`, body `404 page not found`.
      Bound caller, from VM 399 with a valid token:
      `local-ipv4` → `200 '169.254.169.10'`;
      `local-hostname` → `200 '169.254.169.254'` (the address the caller used,
      as the spec says, not the guest's own hostname);
      `public-ipv4` → `404` with an **empty** body — still the unimplemented
      `getPublicIP`, which is queue item 2 and not this change;
      the index → `200 'instance-id\nlocal-hostname\nlocal-ipv4\npublic-ipv4\nplacement/\nservices/\n'`.
      Also re-checked for task 3.3 on the same run:
      `GET /latest/dynamic/instance-identity/signature` → `200
      'dGVzdC1zaWduYXR1cmU='`, and the document still carries
      `"instanceId":"i-399"`.
- [x] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
      **Done** 2026-10-16 on `hogan`, Go 1.24.4. `go vet ./...` exits 0 with no
      output. `go test ./...`: `ok imds 0.026s`, `ok internal/config 0.008s`,
      `ok internal/devid 29.682s`, `ok internal/inventory 0.008s`,
      `ok internal/server 0.111s`; `no test files` for the root package,
      `cmd/devid-enroll` and `internal/proxmox`. No failures.
- [x] 4.4 Run `openspec validate --all --strict` and verify it passes
      **Done** 2026-10-16 with the authoritative CLI at `/usr/local/bin/openspec`:
      `Totals: 8 passed, 0 failed (8 items)` — six living specs plus this change
      and `remove-the-unverifiable-identity-endpoints`. Only `[INFO]`
      "requirement text is very long" notes, which are advisory.
