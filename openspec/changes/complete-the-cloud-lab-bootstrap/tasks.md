# Tasks

## 1. Reproduce

- [x] 1.1 In a shell with `USER` unset, run `scripts/qemu-lab/setup-host.sh` and verify it aborts with `USER: unbound variable` before creating the bridge — the baseline
      Baseline, Claude cloud 2026-10-03: `setup-host.sh: line 58: USER: unbound variable`, exit 1.
- [x] 1.2 Remove `iputils-ping`, `swtpm-tools` and `openssh-client`, run `cloud-install.sh` and verify it does not reinstall them because `qemu-system-x86_64` and `cloud-localds` are present
      Baseline: with the three packages removed, `cloud-install.sh` printed `vtpm-mds cloud install complete`, and `ping`, `swtpm_setup`, `ssh-keygen` were still missing.

## 2. Default USER

- [x] 2.1 Default `USER` to `id -un` in `setup-host.sh` and `start-guest.sh`, and verify `setup-host.sh` completes with `USER` unset and creates `br-imds` with both addresses
      `env -u USER setup-host.sh` printed `MDS lab host ready`, exit 0, `br-imds` carries both addresses.
- [x] 2.2 Verify that with `USER=somebody` exported, the value the scripts use is `somebody`, by tracing the `usermod`/`chown` arguments with `bash -x`
      Stubbed `sudo`: `USER=somebody` gave `chown -R somebody:somebody ...`; `USER` unset gave `root:root`.
- [x] 2.3 Verify `start-guest.sh` reaches the `ip tuntap` line with `USER` unset, and that the tap is owned by the invoking user
      `start-guest.sh` ran inside `e2e-devid-guest.sh` with `USER` unset; `ip -d link` shows `tun type tap ... user root`.

## 3. Complete the bootstrap

- [x] 3.1 Replace the two-tool guard in `cloud-install.sh` with a check over the full command set, and extend the package list with `iputils-ping`, `swtpm`, `swtpm-tools`, `gnutls-bin`, `openssh-client`, and verify with `bash -n` that the script still parses
      Done in `cloud-install.sh`: `LAB_TOOLS` array, install when any is missing, package list extended.
- [x] 3.2 Verify the command set against the scripts by grepping `scripts/qemu-lab/*.sh` for every external command and comparing with the list
      Grep of `scripts/qemu-lab/*.sh` found no external command outside the list except `ssh` (same package as `ssh-keygen`), `sysctl` (base system) and `strings` (error path only, `|| true`); `bridge` matched only comments and `type bridge`.
- [x] 3.3 Verify `cloud-install.sh` on a host with the three packages from 1.2 removed installs them, and `ping`, `swtpm_setup`, `ssh-keygen` resolve afterwards
      With the packages removed, the run printed `Installing lab packages (missing: swtpm_setup swtpm_localca ping ssh-keygen)`; afterwards all four and `certtool` resolve.
- [x] 3.4 Verify `cloud-install.sh` on a fully provisioned host runs no `apt-get` — run it twice and check the second run's output contains no apt activity
      Second run: 0 lines matching `Installing lab|apt|Get:|Reading package`.

## 4. Documentation

- [x] 4.1 State in `scripts/qemu-lab/README.md` that the scripts support the Cursor and the Claude cloud environment, and what each assumes (prepared image vs. bare Ubuntu 24.04, `USER` set or not, no `/dev/kvm`), and verify the table against the observed Claude cloud run
      README table added; wording for the Cursor column is taken from the existing README notes, not observed.

## 5. Prove both environments

- [x] 5.1 Run `cloud-install.sh`, `cloud-start.sh` and `e2e-netns.sh` with `USER` unset and verify `E2E netns IMDS smoke test PASSED` — the Claude cloud path
      `env -u USER`: `cloud-start.sh` exit 0, `E2E netns IMDS smoke test PASSED`, `instance-id=i-100`.
- [x] 5.2 Run the same with `USER=root` exported and verify the same result — the path the Cursor environment takes
      `USER=root`: `E2E netns IMDS smoke test PASSED`, `instance-id=i-100`.
- [x] 5.3 Run `e2e-devid-guest.sh` and verify `GUEST DEVID E2E PASSED`
      `env -u USER e2e-devid-guest.sh`: `Guest enroll finished with rc=0`, `subject=CN = 100`, `GUEST DEVID E2E PASSED` (TCG, no `/dev/kvm`).
- [ ] 5.4 Verify the Cursor environment itself is unaffected; it cannot be run here, so record it as maintainer-run
      Not run: needs the Cursor environment. Maintainer-run; the change is written so that a fully provisioned host takes no new path (no apt run, `USER` unchanged).
- [x] 5.5 Run `bash -n` on every changed script, `go vet ./...` and `go test ./...`, and `openspec validate --all --strict`, and verify all pass
      `bash -n` on all three scripts clean; `go vet ./...` clean; `go test ./...` all packages `ok`; `openspec validate --all --strict`: 10 passed, 0 failed.
