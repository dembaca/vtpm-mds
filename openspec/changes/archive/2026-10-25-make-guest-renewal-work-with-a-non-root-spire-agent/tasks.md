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

- [x] 2.1 Build with `make deb-local`; verify `dpkg-deb -c`/`-I` and
      `lintian --fail-on error,warning` on `devid-enroll`
      `devid-enroll_0.3.0+git20261024.ceede3c83dc8_amd64.deb`: same 13 paths
      as 0.3.0, members `control.tar.xz`/`data.tar.xz`, lintian exit 0; the
      extracted binary reports that version, `-tpm` defaults to
      `/dev/tpmrm0` and it carries `--no-block`.
- [x] 2.2 Install the build in VM 399 (with the maintainer's go), move
      `devid.crt.pem` aside and start `devid-enroll.service`; verify the unit
      ends `success` within seconds, the three files are `spire:spire 0600`,
      the agent logs `Node attestation was successful`, and
      `tpm2_getcap handles-transient` lists nothing
      Done 2026-10-24 19:45 on `0.3.0+git20261024.ceede3c83dc8` (the
      maintainer's go on ask `dfg7mwar…`, sha256 checked in the guest):
      `systemctl start devid-enroll.service` returned after 4.5 s,
      `Result=success` (0.3.0: 120 s, `timeout`); `devid.crt.pem`,
      `.priv.blob`, `.pub.blob` owned by `spire:spire` (UID 999), `0600`
      (0.3.0: `root:root`); hogan logged `devid issued … vm=399` with both
      legs `200`; the agent logged `Node attestation was successful` at
      19:45:24; `tpm2_getcap -T device:/dev/tpm0 handles-transient` empty.
- [x] 2.3 Kill a run mid-enrollment (`systemctl kill -s KILL`); verify no
      transient object is left and the next run enrolls
      Certificate moved aside, `systemctl start --no-block`, `SIGKILL` after
      0.6 s: `Result=signal`, no enroll request reached hogan, so the run was
      killed while creating keys, before the first leg; no certificate
      written; raw-device `handles-transient` empty (0.3.0 left two). The
      next run: `success` in 4 s, `devid issued`, attestation at 19:45:53,
      no transients. Not shown: a kill between the two legs.

## Pre-review

- Fable, 2026-10-24 (`678b246`): no must-fix. Should-fix: the ownership
  test could not tell the replaced file's owner from the directory's (both
  the test user) — now through `lstatPath`/`statPath` seams with distinct
  owners; a guest already left with root-owned files by 0.3.0 keeps them —
  migration note in the proposal and the man page. Nits taken: shorter `-tpm`
  help, wrapped spec line; the 0.3.1 changelog entry belongs to the release.
  These edits change only tests, help text and docs, not the code VM 399 ran.

## 3. Archive

- [x] 3.1 After merge, the coordinator archives the change
      Archived 2026-10-25 by `agent-vtpm-hogan-d3e6cbc82` at the coordinator's
      request (bgl-infra TASK 0.3.2r), before
      `renew-when-the-devid-key-is-not-this-tpms`. Released as 0.3.1; VM 399
      runs the release package since 2026-10-25 07:53 (bgl-infra 2.3b′:
      forced renew `success` in ~4 s, one clean agent restart, attested).
