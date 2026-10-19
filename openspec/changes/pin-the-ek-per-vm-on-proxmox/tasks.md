# Tasks

Written on 2026-10-19 against `main` (contains archived
`report-readiness-honestly`) by `agent-cursor-vtpm-22e0`, at the
coordinator's assignment after dembaca's E1 decision. Files: `internal/devid/`
(+ tests). **Not** `debian/` product logic, **not** `internal/server/server.go`
(hogan #62), **not** `AGENTS.md` / root `README.md`.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-h in
      `design.md`. Verify the outcome is recorded there.
- [x] 1.2 **Coordinator** — ACK the record and the delta on the bus before any
      code is written, and verify the ACK names this change.
      **Done, 2026-10-19:** `agent-mac-…-fc45cd`, "ACK on the record
      `pin-the-ek-per-vm-on-proxmox` @ `5e788f8` … TOFU (A) is right", with
      four record additions (D-g seed, firm pin-before-sign, both backends,
      tests); **code go** after those are in the record.

## 2. Implement

- [ ] 2.1 Add pin store helpers (path, `O_EXCL` create, read) under
      `internal/devid/`; verify unit tests: empty / present / bad perms /
      exclusive create race (exactly one pin).
- [ ] 2.2 Enforce stored pin in `AuthenticateEnrollCaller` when inventory pin
      is empty; verify `401` body `EK certificate does not match stored pin`
      vs inventory body when inventory pin is set; inventory wins over a
      conflicting store file.
- [ ] 2.3 On `enroll/finish`, when no pin exists, `O_EXCL` write **before**
      `IssueDevID`; verify unwritable dir → `500` and no PEM; happy path
      creates `/var/lib/…/ek-pins/<vmid>`; reset (delete file → new EK
      enrolls).
- [ ] 2.4 Run `go test ./...`, `go vet ./...`, and
      `openspec validate --all --strict`; verify all green.

## 3. Verify in the lab

- [ ] 3.1 Fresh enroll creates the pin file; fingerprint matches
      `EKFingerprint`.
- [ ] 3.2 Second enroll with the same TPM succeeds; mismatched store content
      fails with `stored pin` body; after `rm` of the pin file a new EK can
      enroll.
- [ ] 3.3 Say what was not verified (concurrent finish under load, TOFU race
      with a spoofing guest, hogan unless seeded).

## 4. Hogan (seed + enroll; needs dembaca's go)

- [ ] 4.1 **Seed path (D-g):** from the host journal, extract
      `devid issued … vm=<id> ek=<sha256>` into
      `/var/lib/<package>/ek-pins/<vmid>` for VM 399 (and any other VMs in
      scope). Example shape (adjust unit/path to the installed package name):
      ```
      journalctl -u <package>.service --no-pager \
        | sed -n 's/.*devid issued serial=[^ ]* vm=\([^ ]*\) ek=\([0-9a-f]*\).*/\1 \2/p' \
        | while read -r vmid ek; do
            printf '%s\n' "$ek" | sudo tee "/var/lib/<package>/ek-pins/$vmid" >/dev/null
          done
      ```
      Verify the file for `399` matches the last issued `ek=` for that VM.
      **Who:** hogan-d3e6 after dembaca approves host writes.
- [ ] 4.2 After the new package is installed: enroll from VM 399 succeeds with
      the seeded pin; a wrong pin file content fails with `stored pin`.

## 5. Archive

- [ ] 5.1 After merge, the coordinator archives with
      `openspec archive pin-the-ek-per-vm-on-proxmox` and verifies strict stays
      clean. Note that pin alone does not replace E1(a)/(b).
