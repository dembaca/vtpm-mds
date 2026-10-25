# Tasks

Written on 2026-10-25 against `main@0a3d9d3` (v0.3.1) by
`agent-vtpm-hogan-d3e6cbc82`, from the maintainer's decision "A" on ask
`gox8yerp…` (bgl-infra TASK 2.3c). Files: `internal/devid/auth.go`,
`internal/devid/pin.go`, their tests, `debian/vtpm-mds.8`, `debian/changelog`
and this record; the README pin row is the coordinator's.

**Proposal only: implementation waits for the maintainer's go.**

## 1. Implement

- [ ] 1.1 Look up the caller's EK fingerprint in the pin store and the
      inventory, excluding the caller's own VM ID (D-b); verify with unit
      tests on a constructed store: other VM pinned, only own pin, empty file,
      file with an invalid name, inventory `ek_sha256` of another VM
- [ ] 1.2 Refuse both legs with `401` and the D-c body and log line, before
      session and request checks (D-a, D-c); verify with handler tests that
      no challenge is issued, no pin file is created and the log line names
      both VM IDs and no fingerprint
- [ ] 1.3 Answer `500` `pin store unavailable` when the directory cannot be
      listed (D-d); verify with a unit test on an unreadable directory
- [ ] 1.4 Document the operator steps for a VM restored under a new ID and
      for a refused clone (empty `tpmstate`, proposal Impact) in
      `debian/vtpm-mds.8` (D-e); verify with `man -l`
- [ ] 1.5 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`

## 2. Verify on hogan

- [ ] 2.1 Build with `make deb-local` from a tree rebased on `main`;
      `dpkg-deb -c`/`-I`, `lintian --fail-on error,warning`,
      `systemd-analyze verify`
- [ ] 2.2 Install on hogan (maintainer's go); VM 399 enrolls into a scratch
      directory unchanged (own pin), SPIRE's material untouched
- [ ] 2.3 A clone of 399 that kept its vTPM state (bgl-infra, own ask) is
      refused on both legs with the D-c body and log line, and no pin file
      for the clone appears
- [ ] 2.4 Time one leg with the store as it is on hogan

## Pre-review

- [ ] Review by a stronger model before the PR is called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change
