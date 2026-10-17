# Tasks

## 1. Reproduce the teardown bump

- [x] 1.1 On a fresh guest vTPM, run enroll to completion, read `lockoutCounter` live in the guest (SSH + `tpm2_getcap` or a tiny helper), and verify it is `0` — record the output
  - Verified 2026-10-16 cloud lab: after `GUEST DEVID E2E PASSED`, `sudo tpm2_getcap properties-variable` → `TPM2_PT_LOCKOUT_COUNTER: 0x0`, `inLockout: 0`, `TPM2_PT_MAX_AUTH_FAIL: 0x3` (`/opt/cursor/artifacts/lockout-1.1-live.log`).
- [x] 1.2 Stop with today's `stop-guest.sh`, reopen the guest TPM state on a command socket with `startup-clear`, and verify `lockoutCounter` is `1` — record the output
  - Reproduced the old kill path (SIGTERM/SIGKILL QEMU while guest swtpm still attached): persisted state → `TPM2_PT_LOCKOUT_COUNTER: 0x1` (`lockout-1.2-hardkill.log`).
- [x] 1.3 On a fresh guest vTPM, enroll, `poweroff` via SSH, SIGTERM the guest swtpm after QEMU exits, reopen the state, and verify `lockoutCounter` is still `0` — record the output
  - Fresh TPM, enroll rc=0, live counter `0x0`, SSH `poweroff`, then probe → `TPM2_PT_LOCKOUT_COUNTER: 0x0`, `orderly: 1` (`lockout-1.3-graceful.log`).

## 2. Make stop-guest graceful

- [x] 2.1 Change `scripts/qemu-lab/stop-guest.sh` to issue QMP `system_powerdown` (using the existing QMP socket from `start-guest.sh`), wait for the QEMU pid to exit with a bounded timeout, and only then SIGTERM/SIGKILL — verify `bash -n scripts/qemu-lab/stop-guest.sh`
  - `bash -n` OK. Runtime log: `Issued QMP system_powerdown; waiting up to 60s`.
- [x] 2.2 After QEMU has exited, stop the guest swtpm via its pidfile with SIGTERM (and remove the ctrl socket), and verify a subsequent `setup-guest-tpm.sh` on the same state directory does not remanufacture when `tpm2-00.permall` still exists
  - `setup-guest-tpm-reuse.log`: no `Manufacturing vTPM state` line.
- [x] 2.3 Repeat task 1.1 then stop with the new script, reopen the state, and verify `lockoutCounter` remains `0`
  - After new `stop-guest.sh`: `TPM2_PT_LOCKOUT_COUNTER: 0x0` (`lockout-2.3-new-stop.log`).

## 3. Confirm reuse and docs

- [x] 3.1 Run two full `e2e-devid-guest.sh` cycles on the **same** guest vTPM state (do not move `vms/guest100/tpm` aside between them), stopping with the new script each time, and verify both finish with `GUEST DEVID E2E PASSED` and that `lockoutCounter` after the second stop is still `0`
  - Cycles A and B both `GUEST DEVID E2E PASSED`, neither remanufactured; after B: `TPM2_PT_LOCKOUT_COUNTER: 0x0` (`e2e-devid-3.1a.log`, `e2e-devid-3.1b.log`, `lockout-3.1b.log`).
- [x] 3.2 Update the DA-lockout paragraph in `scripts/qemu-lab/README.md` to state that hard qemu kill while swtpm is attached was the cause, enrollment was not, and verify `grep -n 'DA lockout\|lockoutCounter\|stop-guest' scripts/qemu-lab/README.md` shows no claim that enrollment increments the counter
  - README updated; grep shows hard-kill cause / graceful stop, no enrollment-bumps claim.
- [x] 3.3 Run `go vet ./...` and `go test ./...` and verify both pass (no product code expected to change), reporting the output
  - `go vet ./...` clean; `go test ./...` all ok (including `internal/devid`).
- [x] 3.4 Run `openspec validate --all --strict` and verify it passes
  - Totals: 7 passed, 0 failed.

## 4. Record environment limits

- [x] 4.1 State in the PR or task notes that verification used the cloud lab's default `swtpm-localca` and `/usr/share/OVMF/`, and that the QEMU lab was **not** run on hogan
  - Cloud lab: default `swtpm-localca`, `/usr/share/OVMF/`; QEMU lab not run on hogan. Proxmox counter-evidence (VM 399, enrollment leaves `LOCKOUT_COUNTER: 0x0`) recorded in `proposal.md`.
