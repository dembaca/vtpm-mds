# Tasks

Written on 2026-10-18 against `main` at `407bfca` by `agent-mac-vtpm-mds-3502f6`,
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record.**

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — overrule or accept D-a to D-h in `design.md`, and verify the outcome is recorded there
      The ones most likely to be wanted differently: the field order of the access
      line (D-c), keeping the unmatched-route line (D-d), and no line for a refusal
      (D-f).
- [ ] 1.2 **Coordinator** — ACK the record and the deltas, and verify the ACK is on the bus before any code is written

## 2. Implement

- [ ] 2.1 Write the `[DEBUG` lines only when the service was started with `-debug`, and verify with a test that a normal run writes none for a request and a debug run writes the same lines as before
- [ ] 2.2 Give the access line the peer address and the bound VM, quote the path, and verify with tests for a bound caller, an unbound caller, a path with an encoded line break, and that `X-Forwarded-For` is not the peer
- [ ] 2.3 Log one line for each issued LDevID with serial, VM ID and EK fingerprint, and verify with a test that a successful finish writes exactly one such line, that a failed finish writes none, and that the line holds none of the certificate, the nonce and the session id
- [ ] 2.4 For each new test, run it against the code before this change or mutate the new code, and verify it fails
- [ ] 2.5 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass

## 3. Verify on the real systems

- [ ] 3.1 **Hogan** — install the git-stamped package, reload, send a few requests from VM 399 and from the host, and verify from the journal that no `[DEBUG` line appears, that each access line names the peer and `VM` (`399` for the guest, `-` for the host), and that the 24-hour share of debug lines is gone (count before and after)
- [ ] 3.2 **Hogan** — start once with `-debug` and verify the `[DEBUG` lines are back, then go back to the packaged unit
      Changes the running service's arguments, so it needs the maintainer's go in
      the session of whoever runs it.
- [ ] 3.3 **Hogan** — re-enrol VM 399 and verify one `devid issued` line with its serial, `vm=399` and the EK fingerprint, and that the fingerprint equals the one `Start` bound
      Changes a shared resource, VM 399's DevID, so it needs the maintainer's go in
      the session of whoever runs it.
- [ ] 3.4 **Cloud lab** — run the DevID end-to-end script and verify one `devid issued` line per run; say which layout was used
- [ ] 3.5 Say what was not verified: a flooding guest (that is `throttle-guest-requests`), a journal read after a fault with debug off

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists operability, and verify `openspec validate --all --strict` is still clean
