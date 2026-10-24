# Tasks

Written on 2026-10-24 against `main@3a12af3` by
`agent-vtpm-hogan-d3e6cbc82`, from the forced renewal on VM 399 (bgl-infra
TASK 2.3a, the maintainer's go on ask `3zn433ru…`) and the bgl-infra
coordinator's follow-up task "OpenSpec-Change D1–D3 → PR → Release v0.3.1".
Files: `internal/devid/client.go`, `internal/devid/renew.go`, their tests,
`cmd/devid-enroll/main.go`, `debian/devid-enroll.1` and this record. Not the
host package, not `AGENTS.md`/`README.md`.

## 1. Fix

- [x] 1.1 Keep owner and group on replace, from the replaced file or else the
      output directory, only as root; a failing `chown` fails the write
      (D-a); verify with a unit test through a seam for `geteuid` and
      `chown`
      `keepOwner` in `client.go`; `TestWriteFileAtomicKeepsOwnerAsRoot`
      (replaced file's owner, then the directory's for a new file),
      `…LeavesOwnerWhenNotRoot` (no chown) and `…FailsWhenOwnerCannotBeKept`
      (EPERM fails the write, old content kept, no temporary left) pass.
- [x] 1.2 Queue the agent restart with `--no-block` (D-b); verify with a
      unit test of the command line
      `TestTryRestartSPIREAgentDoesNotWait` passes (`systemctl --no-block
      try-restart spire-agent.service`).
- [x] 1.3 Default `-tpm` to `/dev/tpmrm0` and update the man page (D-c);
      verify `devid-enroll -h` and `man -l debian/devid-enroll.1`
      `-h` shows `(default "/dev/tpmrm0")`; the man page renders the new
      default, `--no-block` and the ownership paragraph.
- [x] 1.4 Run `go vet ./...` and `go test ./...`
      `go vet` clean; `go test ./...` ok for every package (`internal/devid`
      103 s with the simulator tests).
- [x] 1.5 Run `openspec validate --all --strict`
      Totals: 6 passed, 0 failed (6 items).

## 2. Verify on VM 399

- [ ] 2.1 Build with `make deb-local`; verify `dpkg-deb -c`/`-I` and
      `lintian --fail-on error,warning` on `devid-enroll`
- [ ] 2.2 Install the build in VM 399 (with the maintainer's go), move
      `devid.crt.pem` aside and start `devid-enroll.service`; verify the unit
      ends `success` within seconds, the three files are `spire:spire 0600`,
      the agent logs `Node attestation was successful`, and
      `tpm2_getcap handles-transient` lists nothing
- [ ] 2.3 Kill a run mid-enrollment (`systemctl kill -s KILL`); verify no
      transient object is left and the next run enrolls

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change
