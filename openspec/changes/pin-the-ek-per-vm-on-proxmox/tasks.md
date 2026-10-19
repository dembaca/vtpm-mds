# Tasks

Written on 2026-10-19 against `main` (contains archived
`report-readiness-honestly`) by `agent-cursor-vtpm-22e0`, at the
coordinator's assignment after dembaca's E1 decision. **Code only after the
coordinator's ACK of this record.** Files: `internal/devid/` (+ tests);
optionally tiny helpers elsewhere only if needed for the pin path. **Not**
`debian/` product logic, **not** `internal/server/server.go` (hogan #62),
**not** `AGENTS.md` / root `README.md`.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-f in
      `design.md` (D-a path, D-b write-before-sign, D-c dual error bodies,
      D-d no auto-reset). Verify the outcome is recorded there.
- [ ] 1.2 **Coordinator** — ACK the record and the delta on the bus before any
      code is written, and verify the ACK names this change.

## 2. Implement (after ACK)

- [ ] 2.1 Add pin store helpers (path, exclusive create, read) under
      `internal/devid/`; verify with unit tests for empty / present / bad perms.
- [ ] 2.2 Enforce stored pin in `AuthenticateEnrollCaller` when inventory pin
      is empty; verify `401` body `EK certificate does not match stored pin`
      and that inventory pin still wins.
- [ ] 2.3 On `enroll/finish`, when no pin exists, write the pin **before**
      signing the LDevID; verify unwritable dir → no PEM; happy path creates
      `/var/lib/…/ek-pins/<vmid>`.
- [ ] 2.4 Run `go test ./...`, `go vet ./...`, and
      `openspec validate --all --strict`; verify all green.

## 3. Verify in the lab

- [ ] 3.1 Fresh enroll on lab guest (or swtpm test) creates the pin file;
      verify path + fingerprint match `EKFingerprint`.
- [ ] 3.2 Second enroll with the same TPM succeeds; enroll with a mismatched
      pin file content fails with `stored pin` body.
- [ ] 3.3 Say what was not verified (hogan VM 399, concurrent first finishes,
      TOFU race with a spoofing guest).

## 4. Archive

- [ ] 4.1 After merge, the coordinator archives with
      `openspec archive pin-the-ek-per-vm-on-proxmox` and verifies strict stays
      clean. Note in archive that pin alone does not replace E1(a)/(b).
