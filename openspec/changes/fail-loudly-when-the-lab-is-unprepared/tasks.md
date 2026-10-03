# Tasks

## 1. Reproduce all three

- [ ] 1.1 Run `scripts/qemu-lab/gen-lab-pki.sh` on a host whose `/etc/swtpm_setup.conf` names a non-default CA config, and verify it prints `WARNING: swtpm-localca issuer missing`, writes no `ek-chain.pem`, prints `Lab PKI ready` and exits 0 — record that as the baseline
      Verified here (Claude cloud, default swtpm-localca layout): with `issuercert.pem` moved
      aside, `gen-lab-pki.sh` printed `WARNING: swtpm-localca issuer missing`, then
      `Lab PKI ready under ...` and exited 0. That is the warn-and-continue mechanism.
      NOT verified: the site-CA case itself (`/etc/swtpm_setup.conf` naming a non-default
      config) — this box has the default layout. Left unticked for the maintainer host.
- [x] 1.2 Verify `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v` SKIPs on a lab prepared only by `setup-guest-tpm.sh`, and verify by `ls` that the only socket produced is the `--ctrl` one
      Baseline: after `setup-guest-tpm.sh` only `guest100.tpm.pid` and `guest100.tpm.sock`
      (the `--ctrl` socket) existed; the test printed `SKIP ... no such file or directory`.
- [x] 1.3 Verify `e2e-devid-guest.sh` reaches `create-guest.sh` before failing when `cloud-localds` is absent, by checking how far its output gets — this is the cost the precondition gate removes
      Baseline, `cloud-localds` hidden: the run built the binaries, set up the bridge,
      checked the 597 MB image, manufactured the vTPM, generated the PKI and started the
      daemon before dying in `create-guest.sh` with exit 127.
- [x] 1.4 Verify the ordering instance of the same pattern is already fixed: `gen-lab-pki.sh` ran before `setup-guest-tpm.sh`, so the first run had no EK chain and the service started without DevID routes
      Already fixed by the cloud lab agent in commit `a7c0106`, merged here. The
      script now manufactures the vTPM first and aborts when the service log
      says `DevID enrollment disabled`. Verified by reading the merged script;
      the cloud lab guest e2e run on 2026-10-03 passed with that ordering.

## 2. Build the EK chain from the CA the host uses

- [ ] 2.1 Make `gen-lab-pki.sh` read `create_certs_tool_config` from `/etc/swtpm_setup.conf` and `issuercert` from that file, falling back to the default swtpm-localca path, and verify it resolves `/etc/ssl/certs/proxmox_tpm_ca.crt` on a host configured that way
      Verified: the parser reads `create_certs_tool_config` and `issuercert` from a synthetic
      site layout (comments, odd spacing, trailing comment, decoy commented-out line) and
      falls back to the default path; in the default layout the chain is byte-identical to
      the old `cat issuercert + rootca` (`cmp`).
      NOT verified: `/etc/ssl/certs/proxmox_tpm_ca.crt` on a host configured that way. Left
      unticked for the maintainer host.
- [x] 2.2 Verify the resulting `/etc/vtpm-mds/ek-chain.pem` carries the issuing CA by checking `openssl x509 -noout -subject` reports the expected CA common name
      Synthetic site CA: `openssl x509 -noout -subject` on `/etc/vtpm-mds/ek-chain.pem`
      printed `CN = Synthetic Site TPM CA, O = lab`. Default layout: chain identical to the
      old output. The expected CN for the Proxmox CA was not checked (not on this box).
- [x] 2.3 Make the script exit non-zero when no EK chain can be assembled, and verify it does so with a message naming both the config it consulted and the file it could not find
      Missing issuer (default layout and synthetic site layout) and an unreadable config:
      exit 1 with `looked for`, `swtpm setup` and `CA config` lines naming both.
- [x] 2.4 Verify an EK certificate freshly produced by `swtpm_setup` on that host verifies against the generated chain, with `openssl verify -CAfile`
      Fresh `swtpm_setup --create-ek-cert --write-ek-cert-files`: both `ek-rsa2048.crt` and
      `ek-secp384r1.crt` verified `OK` with `openssl verify -CAfile /etc/vtpm-mds/ek-chain.pem`.
      Default swtpm-localca chain only.

## 3. Produce the socket the test opens

- [ ] 3.1 Add a `--server type=unixio` command socket to the existing swtpm invocation in `setup-guest-tpm.sh`, keeping the `--ctrl` socket and its current name, and verify both sockets exist after the script runs
      NOT done as written, and cannot be: adding `--server` to the guest's swtpm makes QEMU
      fail at start with `tpm-emulator: Failed to send CMD_SET_DATAFD`. Isolated on copies of
      the state: `--ctrl` alone and `--ctrl` + `startup-clear` run; `--ctrl` + `--server` fails.
      So design.md's decision "One swtpm instance, two sockets" does not hold.
      Done instead: `setup-guest-tpm.sh` keeps the guest instance as it was and starts a second
      swtpm with its own state directory (`vms/guest100-hosttest/tpm`), `--server` socket at
      `/var/lib/mds-lab/run/swtpm.sock`, `--flags not-need-init,startup-clear`. Both sockets
      exist after the script. Left unticked until the coordinator accepts this and updates
      design.md (outside this agent's files).
- [ ] 3.2 Verify only one swtpm process serves the state directory, so the two sockets cannot race over NVRAM
      As written (one process over one state directory) this does not apply; see 3.1. What
      holds: two swtpm processes, each over its own state directory — `ps` shows exactly one
      per directory — so there is no NVRAM race. Left unticked with 3.1.
- [x] 3.3 Verify `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v` now RUNS and passes on a prepared lab host, and record the issued subject from its log line
      `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v -count=1` RUNS and passes:
      `DevID cert issued (1371 bytes PEM) subject=CN=100`. Six consecutive runs passed.
      It needs `startup-clear`: without it the socket answers `TPM_RC_INITIALIZE` to every
      command and the test skips with `device is not a TPM 2.0`.
- [x] 3.4 Verify the test still SKIPs cleanly on a machine with no lab, so `go test ./...` keeps working for a developer
      Skips with no socket and with a stale socket file (nobody listening: `device is not a
      TPM 2.0`); `go test ./internal/devid/` passes without a lab.

## 4. Fail before doing any work

- [x] 4.1 Add a precondition gate to the top of `e2e-devid-guest.sh` covering every external binary it and its child scripts invoke, and verify the list matches by grepping the child scripts for command invocations
      Gate in `e2e-devid-guest.sh` via `common.sh`. Grep of the e2e chain found no directly
      invoked tool outside it except `strings` (error path only, `|| true`). `certtool` and
      `swtpm_localca` are in the gate because `swtpm_setup` needs them.
- [x] 4.2 Verify the gate reports the missing tool by name and points at `scripts/qemu-lab/cloud-install.sh`, by temporarily hiding one tool from `PATH` and running the script
      `cloud-localds` hidden: `ERROR: missing tool(s): cloud-localds` + `Run
      scripts/qemu-lab/cloud-install.sh ...`, exit 1, in 6 ms. Hidden by moving the binary,
      not by `PATH`.
- [x] 4.3 Verify the gate runs before any side effect: with a tool hidden, confirm no PKI was generated, no bridge created, no image downloaded and no vTPM manufactured
      With the tool hidden: no file under `/etc/vtpm-mds`, `/var/lib/mds-lab`,
      `/var/lib/swtpm-localca` or `bin/` newer than a marker, swtpm pids and `ip -br addr`
      unchanged.
- [ ] 4.4 Make the OVMF firmware path discovered rather than hardcoded, and verify it finds the firmware on a host that keeps it under `/usr/share/pve-edk2-firmware/`
      Verified: discovery picks the first readable code+vars pair in `/usr/share/OVMF` (this
      box), and finds a pair in a directory standing in for `pve-edk2-firmware` (synthetic
      path, `OVMF_CODE_4M.fd`/`OVMF_VARS_4M.fd` names).
      NOT verified: `/usr/share/pve-edk2-firmware/` on a real Proxmox host, whose actual file
      names were not seen. Left unticked for the maintainer host.
- [x] 4.5 Verify the `OVMF_CODE` and `OVMF_VARS_TEMPLATE` environment overrides still win when set, and that the gate reports every path it tried when nothing is found
      Both overrides win; with only `OVMF_CODE` set the vars template is discovered; with
      nothing found the error lists all eight paths tried; an override naming a missing file
      is reported. Exercised against synthetic directories.

## 5. Confirm nothing else moved

- [x] 5.1 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
      `bash -n` on every script in `scripts/qemu-lab/`, `go vet ./...`, `go test -count=1 ./...`
      all clean; `openspec validate --all --strict`: 10 passed, 0 failed. Only files under
      `scripts/qemu-lab/` and this `tasks.md` changed.
- [x] 5.2 Verify a full `e2e-devid-guest.sh` run on a prepared lab host still reaches `Guest enroll finished with rc=0` — this needs the cloud lab host, so record it as maintainer-run if it cannot be executed here
      On a fresh guest vTPM: `Guest enroll finished with rc=0`, `subject=CN = 100`,
      `GUEST DEVID E2E PASSED` (TCG). Two earlier attempts are why 3.1 changed: the first,
      with `--server` on the guest instance, died at QEMU start; a later one failed with
      `Certify: ... DA lockout mode` on a reused vTPM whose counter read 3 of 3.
      Not caused by this change: after ONE successful run on a fresh vTPM the counter reads
      1 of 3, so reusing a guest vTPM across runs eventually locks it. Cause not understood;
      it needs its own change. Documented in the README.
