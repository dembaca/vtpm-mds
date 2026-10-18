# Tasks

Written on 2026-10-18 against `main` at `407bfca` by `agent-mac-vtpm-mds-3502f6`,
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record.**

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — overrule or accept D-a to D-h in `design.md`, and verify the outcome is recorded there
      The ones most likely to be wanted differently: the field order of the access
      line (D-c), keeping the unmatched-route line (D-d), and no line for a refusal
      (D-f).
- [x] 1.2 **Coordinator** — ACK the record and the deltas, and verify the ACK is on the bus before any code is written
      **Done, 2026-10-18:** ACK on the bus for #49 at `d7d5999`, with one addition
      (errors stay visible, with the list of the lines that change) taken into D-h
      and a scenario.

## 2. Implement

- [x] 2.1 Write the `[DEBUG` lines only when the service was started with `-debug`, and verify with a test that a normal run writes none for a request and a debug run writes the same lines as before
      **Done, 2026-10-18** (`f4998da`): `server.SetDebug` and `debugf` in
      `internal/server/server.go`, called from `main.go`; 18 lines go through
      `debugf`, four (D-h) through `log.Printf`. Tests in
      `internal/server/logging_test.go`: a normal run writes no `[DEBUG`, a debug run
      writes the lines, and errors stay visible without the flag.
- [x] 2.2 Give the access line the peer address and the bound VM, quote the path, and verify with tests for a bound caller, an unbound caller, a path with an encoded line break, and that `X-Forwarded-For` is not the peer
      **Done, 2026-10-18:** `loggingMiddleware` writes `METHOD %q STATUS DURATION PEER
      VM`; the unmatched-route line quotes the path too. Tests: bound caller,
      unbound caller and an unparseable peer, no peer from a header, a path that
      holds a line break.
- [x] 2.3 Log one line for each issued LDevID with serial, VM ID and EK fingerprint, and verify with a test that a successful finish writes exactly one such line, that a failed finish writes none, and that the line holds none of the certificate, the nonce and the session id
      **Done, 2026-10-18:** `logIssuance` in `internal/devid/http.go`, called by
      `HandleFinish` after `Finish`. `internal/devid/issuance_log_test.go` drives
      start and finish through the real handlers: a successful finish logs exactly
      one line with the serial, `vm=100` and the EK fingerprint and no PEM or
      session; a finish with a wrong response logs none and is still refused with
      `400`.
- [x] 2.4 For each new test, run it against the code before this change or mutate the new code, and verify it fails
      **Done, 2026-10-18**, by mutation, each applied to the committed code and
      restored: debug always on (2 tests fail), debug never on (1), the reload
      summary left as debug (1), a load failure gated (1), the path not quoted (2),
      the peer taken from `X-Forwarded-For` (3), the VM always `-` (1), no issuance
      line (1), the PEM in the issuance line (1), and a refused finish logged as an
      issuance (1).
- [x] 2.5 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass
      **Done, 2026-10-18:** `go vet` and `go test` for `.`, `./cmd/devid-enroll`,
      `./imds` and `./internal/...` pass; `openspec validate --all --strict` →
      6 passed; a trial `openspec archive` on a scratch copy applies the deltas
      (+3) and leaves 5 passed.

## 3. Verify on the real systems

- [x] 3.1 **Hogan** — install the git-stamped package, reload, send a few requests from VM 399 and from the host, and verify from the journal that no `[DEBUG` line appears, that each access line names the peer and `VM` (`399` for the guest, `-` for the host), and that the 24-hour share of debug lines is gone (count before and after)
      **Done, 2026-10-18, hogan (coordinator), against
      `0.2.0+git20261018.57cc6cb3c8ac` (head `57cc6cb`), with the maintainer's go in
      hogan's session:** passed. Before the change, 30 minutes of the journal held 427
      lines, 366 of them `[DEBUG`. After it, a start, a probe run from VM 399 and
      three requests from the host wrote 15 lines and no `[DEBUG`. The start and a
      reload showed, without a marker, `Cache refresh complete (…): 3 VMs, 5 MAC
      addresses cached`. A load failure and the other errors of D-h were not
      provoked on hogan (the unit tests cover them). Each access line named the peer
      and the VM: `PUT "/latest/api/token" 200 <duration> <peer> 399` for the bound
      guest, `GET "/latest/meta-data/instance-id" 401 … <host address> -` for the
      host. A request carrying `X-Forwarded-For: 10.9.9.9` did not show it, and a path
      with an encoded line break appeared as one line, `GET
      "/latest/meta-data/x\nFAKE LINE" 401 …`.
- [x] 3.2 **Hogan** — start once with `-debug` and verify the `[DEBUG` lines are back, then go back to the packaged unit
      Changes the running service's arguments, so it needs the maintainer's go in
      the session of whoever runs it.
      **Done, 2026-10-18, hogan:** a second instance started with `-debug` on a
      loopback port wrote the trace lines again (nine in one run), as before the change;
      the packaged unit was not touched, so no change of the running service's
      arguments was needed.
- [x] 3.3 **Hogan** — re-enrol VM 399 and verify one `devid issued` line with its serial, `vm=399` and the EK fingerprint, and that the fingerprint equals the one `Start` bound
      Changes a shared resource, VM 399's DevID, so it needs the maintainer's go in
      the session of whoever runs it.
      **Done, 2026-10-18, hogan, with the maintainer's go:** one enrolment of VM 399,
      with the client of the same build, into a scratch directory, guarded against a
      second run and without touching the SPIRE files (the same before and after; the
      lockout counter `0x0` before and after). The journal held exactly one line
      `devid issued serial=b25d051a… vm=399 ek=19517bd1…`; the serial is the
      certificate's (`CN=399`), the fingerprint is the EK certificate's, and the line
      held no PEM and no key, between the access lines of the start and the finish.
      Whether the fingerprint equals the one the start leg bound was not compared
      separately on hogan; the unit test of the real handlers shows it. The scratch
      directory was removed.
- [ ] 3.4 **Cloud lab** — run the DevID end-to-end script and verify one `devid issued` line per run; say which layout was used
- [ ] 3.5 Say what was not verified: a flooding guest (that is `throttle-guest-requests`), a journal read after a fault with debug off

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists operability, and verify `openspec validate --all --strict` is still clean
