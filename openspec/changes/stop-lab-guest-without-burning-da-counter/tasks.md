# Tasks

## 1. Reproduce the teardown bump

- [ ] 1.1 On a fresh guest vTPM, run enroll to completion, read `lockoutCounter` live in the guest (SSH + `tpm2_getcap` or a tiny helper), and verify it is `0` — record the output
- [ ] 1.2 Stop with today's `stop-guest.sh`, reopen the guest TPM state on a command socket with `startup-clear`, and verify `lockoutCounter` is `1` — record the output
- [ ] 1.3 On a fresh guest vTPM, enroll, `poweroff` via SSH, SIGTERM the guest swtpm after QEMU exits, reopen the state, and verify `lockoutCounter` is still `0` — record the output

## 2. Make stop-guest graceful

- [ ] 2.1 Change `scripts/qemu-lab/stop-guest.sh` to issue QMP `system_powerdown` (using the existing QMP socket from `start-guest.sh`), wait for the QEMU pid to exit with a bounded timeout, and only then SIGTERM/SIGKILL — verify `bash -n scripts/qemu-lab/stop-guest.sh`
- [ ] 2.2 After QEMU has exited, stop the guest swtpm via its pidfile with SIGTERM (and remove the ctrl socket), and verify a subsequent `setup-guest-tpm.sh` on the same state directory does not remanufacture when `tpm2-00.permall` still exists
- [ ] 2.3 Repeat task 1.1 then stop with the new script, reopen the state, and verify `lockoutCounter` remains `0`

## 3. Confirm reuse and docs

- [ ] 3.1 Run two full `e2e-devid-guest.sh` cycles on the **same** guest vTPM state (do not move `vms/guest100/tpm` aside between them), stopping with the new script each time, and verify both finish with `GUEST DEVID E2E PASSED` and that `lockoutCounter` after the second stop is still `0`
- [ ] 3.2 Update the DA-lockout paragraph in `scripts/qemu-lab/README.md` to state that hard qemu kill while swtpm is attached was the cause, enrollment was not, and verify `grep -n 'DA lockout\|lockoutCounter\|stop-guest' scripts/qemu-lab/README.md` shows no claim that enrollment increments the counter
- [ ] 3.3 Run `go vet ./...` and `go test ./...` and verify both pass (no product code expected to change), reporting the output
- [ ] 3.4 Run `openspec validate --all --strict` and verify it passes

## 4. Record environment limits

- [ ] 4.1 State in the PR or task notes that verification used the cloud lab's default `swtpm-localca` and `/usr/share/OVMF/`, and that the QEMU lab was **not** run on hogan
