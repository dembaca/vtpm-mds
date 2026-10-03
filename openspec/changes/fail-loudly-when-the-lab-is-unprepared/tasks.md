# Tasks

## 1. Reproduce all three

- [ ] 1.1 Run `scripts/qemu-lab/gen-lab-pki.sh` on a host whose `/etc/swtpm_setup.conf` names a non-default CA config, and verify it prints `WARNING: swtpm-localca issuer missing`, writes no `ek-chain.pem`, prints `Lab PKI ready` and exits 0 — record that as the baseline
- [ ] 1.2 Verify `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v` SKIPs on a lab prepared only by `setup-guest-tpm.sh`, and verify by `ls` that the only socket produced is the `--ctrl` one
- [ ] 1.3 Verify `e2e-devid-guest.sh` reaches `create-guest.sh` before failing when `cloud-localds` is absent, by checking how far its output gets — this is the cost the precondition gate removes
- [x] 1.4 Verify the ordering instance of the same pattern is already fixed: `gen-lab-pki.sh` ran before `setup-guest-tpm.sh`, so the first run had no EK chain and the service started without DevID routes
      Already fixed by the cloud lab agent in commit `a7c0106`, merged here. The
      script now manufactures the vTPM first and aborts when the service log
      says `DevID enrollment disabled`. Verified by reading the merged script;
      the cloud lab guest e2e run on 2026-10-03 passed with that ordering.

## 2. Build the EK chain from the CA the host uses

- [ ] 2.1 Make `gen-lab-pki.sh` read `create_certs_tool_config` from `/etc/swtpm_setup.conf` and `issuercert` from that file, falling back to the default swtpm-localca path, and verify it resolves `/etc/ssl/certs/proxmox_tpm_ca.crt` on a host configured that way
- [ ] 2.2 Verify the resulting `/etc/vtpm-mds/ek-chain.pem` carries the issuing CA by checking `openssl x509 -noout -subject` reports the expected CA common name
- [ ] 2.3 Make the script exit non-zero when no EK chain can be assembled, and verify it does so with a message naming both the config it consulted and the file it could not find
- [ ] 2.4 Verify an EK certificate freshly produced by `swtpm_setup` on that host verifies against the generated chain, with `openssl verify -CAfile`

## 3. Produce the socket the test opens

- [ ] 3.1 Add a `--server type=unixio` command socket to the existing swtpm invocation in `setup-guest-tpm.sh`, keeping the `--ctrl` socket and its current name, and verify both sockets exist after the script runs
- [ ] 3.2 Verify only one swtpm process serves the state directory, so the two sockets cannot race over NVRAM
- [ ] 3.3 Verify `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v` now RUNS and passes on a prepared lab host, and record the issued subject from its log line
- [ ] 3.4 Verify the test still SKIPs cleanly on a machine with no lab, so `go test ./...` keeps working for a developer

## 4. Fail before doing any work

- [ ] 4.1 Add a precondition gate to the top of `e2e-devid-guest.sh` covering every external binary it and its child scripts invoke, and verify the list matches by grepping the child scripts for command invocations
- [ ] 4.2 Verify the gate reports the missing tool by name and points at `scripts/qemu-lab/cloud-install.sh`, by temporarily hiding one tool from `PATH` and running the script
- [ ] 4.3 Verify the gate runs before any side effect: with a tool hidden, confirm no PKI was generated, no bridge created, no image downloaded and no vTPM manufactured
- [ ] 4.4 Make the OVMF firmware path discovered rather than hardcoded, and verify it finds the firmware on a host that keeps it under `/usr/share/pve-edk2-firmware/`
- [ ] 4.5 Verify the `OVMF_CODE` and `OVMF_VARS_TEMPLATE` environment overrides still win when set, and that the gate reports every path it tried when nothing is found

## 5. Confirm nothing else moved

- [ ] 5.1 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
- [ ] 5.2 Verify a full `e2e-devid-guest.sh` run on a prepared lab host still reaches `Guest enroll finished with rc=0` — this needs the cloud lab host, so record it as maintainer-run if it cannot be executed here
