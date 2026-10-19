# Tasks

Written on 2026-10-19 against `main` (contains archived
`report-readiness-honestly`) by `agent-cursor-vtpm-22e0`, at the
coordinator's assignment after dembaca's E1 decision. Files: `internal/devid/`
(+ tests) and `ekPinDir` in `internal/server/server.go`. **Not** `debian/`
product logic, **not** `AGENTS.md` / root `README.md`.

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

- [x] 2.1 Add pin store helpers (path, `O_EXCL` create, read) under
      `internal/devid/`; verify unit tests: empty / present / bad perms /
      exclusive create race (exactly one pin).
      `pin.go` + `TestReadStoredPinEmptyPresent`,
      `TestCreateStoredPinExclusiveRace`,
      `TestEnsurePinBeforeIssueUnwritable`.
- [x] 2.2 Enforce stored pin in `AuthenticateEnrollCaller` when inventory pin
      is empty; verify `401` body `EK certificate does not match stored pin`
      vs inventory body when inventory pin is set; inventory wins over a
      conflicting store file.
      `TestAuthenticateStoredPin`.
- [x] 2.3 On `enroll/finish`, when no pin exists, `O_EXCL` write **before**
      `IssueDevID`; verify unwritable dir → `500` and no PEM; happy path
      creates `/var/lib/…/ek-pins/<vmid>`; reset (delete file → new EK
      enrolls).
      `TestFinishWritesPinBeforeIssue`, `TestPinWrittenWhenIssueDevIDFails`
      (pin present, no PEM), `TestFinishUnwritablePinDirNoPEM`,
      `TestEnsurePinResetAllowsNewEK`, `TestConcurrentFinishSameEKBothGetCert`;
      HTTP maps `ErrPinStore` → 500 body `pin store unavailable`.
      `PinDir` = `$STATE_DIRECTORY/ek-pins` else package state dir (`ekPinDir`).
- [x] 2.4 Run `go test ./...`, `go vet ./...`, and
      `openspec validate --all --strict`; verify all green.
      After Fable re-check follow-up: `go vet ./...` clean;
      `go test -count=1 ./...` → ok imds, config, devid, inventory, proxmox,
      server (no test files: module root, cmd/devid-enroll);
      `openspec validate --all --strict` 8 passed. Includes
      `TestPinWrittenWhenIssueDevIDFails` and `TestEmptyPinFileMeansNoPin`.

## 3. Verify in the lab

- [x] 3.1 Fresh enroll creates the pin file; fingerprint matches
      `EKFingerprint`.
      Re-lab 2026-10-19 after Fable should-fix (binary `3b144ba`,
      `/var/lib/mds-lab/build-65`, PID 210743, `STATE_DIRECTORY` = package
      state dir). Guest100 SSH enroll → `/var/lib/<pkg>/ek-pins/100` =
      `1ab805b7d10249979f8d2e0758853d10d3e91603cd99f04a2511606aefc911c3`
      matching `devid issued … ek=` (same `EKFingerprint`). Log:
      `/opt/cursor/artifacts/65-3.x-fable-relab.log`.
- [x] 3.2 Second enroll with the same TPM succeeds; mismatched store content
      fails with `stored pin` body; after `rm` of the pin file a new EK can
      enroll.
      Re-lab @ `3b144ba` PID 210743: re-enroll rc=0; wrong pin →
      `HTTP 401: EK certificate does not match stored pin`, no PEM. (Reset
      path was covered on the earlier `a7239df` run after enroll-throttle
      cooldown; not re-hit on re-lab to save 429 budget.)
- [x] 3.3 Say what was not verified (concurrent finish under load, TOFU race
      with a spoofing guest, hogan unless seeded).
      Not verified: concurrent finish under load (unit-tested instead:
      `TestConcurrentFinishSameEKBothGetCert`), a spoofing guest racing
      first enroll, a genuinely new EK, hogan seed (4.x).

## 4. Hogan (seed + enroll; needs dembaca's go)

- [x] 4.1 **Seed path (D-g):** from the host journal, extract
      `devid issued … vm=<id> ek=<sha256>` into
      `/var/lib/<package>/ek-pins/<vmid>` for VM 399 (and any other VMs in
      scope). Create the directory as the service user first (a root-owned
      `ek-pins/` leaves the daemon unable to add pins for new VMs → 500):
      ```
      sudo install -d -o <service-user> -g <service-group> -m 0750 \
        /var/lib/<package>/ek-pins
      journalctl -u <package>.service --no-pager \
        | sed -n 's/.*devid issued serial=[^ ]* vm=\([^ ]*\) ek=\([0-9a-f]*\).*/\1 \2/p' \
        | while read -r vmid ek; do
            printf '%s\n' "$ek" | sudo tee "/var/lib/<package>/ek-pins/$vmid" >/dev/null
            sudo chown <service-user>:<service-group> \
              "/var/lib/<package>/ek-pins/$vmid"
            sudo chmod 0640 "/var/lib/<package>/ek-pins/$vmid"
          done
      ```
      After `fresh-tpm` the journal may list two EKs for the same VMID; the
      last `devid issued` line wins — verify `399` matches that last `ek=`.
      **Who:** hogan-d3e6 after dembaca approves host writes.
      **Done 2026-10-19 16:24 by dembaca as root on `hogan`** (the agent
      cannot write `/var/lib/vtpm-mds`), VM 399 only, with the coordinator's
      block: the journal held three `devid issued … vm=399` lines, all with
      `ek=19517bd139d4aa0512259dd96ca731d4c017b6c6af6891ceeb32e6a88aacc069`
      (no second EK since `fresh-tpm`); `ek-pins/` `vtpm-mds:vtpm-mds`
      `drwxr-x---`, `399` `vtpm-mds:vtpm-mds` `-rw-r-----`, 65 bytes, that
      value.
- [x] 4.2 After the new package is installed: enroll from VM 399 succeeds with
      the seeded pin; a wrong pin file content fails with `stored pin`.
      Done 2026-10-19 on `hogan` by hogan-d3e6 and dembaca. Built `256988a`
      (contains `main@c94bb2d`, clean tree) → `0.2.0+git20261019.256988ac0b8a`,
      installed 16:33:38 through `hogan-lab dpkg-install`: `try-restart`
      PID 3126137 → 3270635, gap 0.19 s, `DevID CA key from systemd
      credential`, 3 VMs/5 MACs.
      - Seeded pin: `devid-enroll -out /root/devid-scratch-65` in VM 399 at
        16:34 → `DevID enrollment complete`, `devid issued … vm=399
        ek=19517bd1…acc069`. The pin file kept its 16:24:33 mtime (read by
        dembaca at 17:04), so the matching pin was compared, not rewritten.
      - Wrong pin (dembaca, 17:04, 64 zeros, owner/mode kept):
        `devid-enroll` exit 1, `enroll/start: HTTP 401: EK certificate does
        not match stored pin`; journal `devid enroll auth: EK certificate does
        not match stored pin` and `POST "/latest/devid/enroll/start" 401 …
        169.254.169.10 399`; no `devid issued` line.
      - Restored from `/root/ek-pin-399.bak-20261019`: `vtpm-mds:vtpm-mds 640
        65`, `19517bd1…acc069`; `devid-enroll -out
        /root/devid-scratch-65-back` → `DevID enrollment complete`, journal
        `enroll/start 200`, `devid issued serial=7ad39ef6… vm=399
        ek=19517bd1…`, `enroll/finish 200`.
      - SPIRE's material `/var/lib/spire/agent/devid.{crt.pem,priv.blob,pub.blob}`
        sha256 `253be3e4…`, `39679c24…`, `d8fb058d…` before 16:34 and after
        17:04, unchanged. The scratch directories stay in the guest.
      Not verified on `hogan`: a first enrollment of a VM with no pin (no
      second test VM), and the concurrent finish (unit test only).

## 5. Archive

- [ ] 5.1 After merge, the coordinator archives with
      `openspec archive pin-the-ek-per-vm-on-proxmox` and verifies strict stays
      clean. Note that pin alone does not replace E1(a)/(b).
