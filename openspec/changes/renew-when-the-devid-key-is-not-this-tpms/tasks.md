# Tasks

Written on 2026-10-25 against `main@0a3d9d3` (v0.3.1) by
`agent-vtpm-hogan-d3e6cbc82`, from bgl-infra TASK 2.3c and the bgl-infra
coordinator's request for a 0.3.2 proposal. Files: `internal/devid/client.go`,
`internal/devid/renew.go`, their tests, `debian/devid-enroll.1` and this
record. Not the host package, not `AGENTS.md`/`README.md`.

**Proposal only: implementation waits for the maintainer's go.**

## 1. Implement

- [ ] 1.1 Check fit before the lifetime short-cut (D-a): missing or empty
      blob, certificate key ≠ `devid.pub.blob` key, `ErrDevIDLoad`; verify
      with unit tests for each reason and for fitting materials
- [ ] 1.2 Reuse the key loaded by the check, or create a new one when it did
      not load (D-b); verify with a simulator test: blobs from another
      simulator instance plus a fresh certificate lead to a new key and a
      request, fitting blobs to no request
- [ ] 1.3 Fail the run on a TPM error other than `ErrDevIDLoad` and leave the
      files unchanged (D-c); verify with a unit test through the key factory
- [ ] 1.4 Log the reason once per renewal (D-d); verify in the tests' log
      output
- [ ] 1.5 Update `debian/devid-enroll.1`; verify with `man -l`
- [ ] 1.6 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`

## 2. Verify on a guest

- [ ] 2.1 Build with `make deb-local`; `dpkg-deb -c`/`-I` and
      `lintian --fail-on error,warning` on `devid-enroll`
- [ ] 2.2 On VM 399 (maintainer's go): with fitting materials a run makes no
      request and leaves the files unchanged; with `devid.pub.blob` moved
      aside it renews with reason `missing blob`
- [ ] 2.3 On a clone with an empty vTPM and inherited materials (bgl-infra,
      as in 2.3c): the first boot renews with reason `key does not load in
      this TPM` before `spire-agent` starts, the new certificate has the
      clone's CN, and the agent attests with a new agent ID

## Pre-review

- [ ] Review by a stronger model before the PR is called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change (after
      `make-guest-renewal-work-with-a-non-root-spire-agent`)
