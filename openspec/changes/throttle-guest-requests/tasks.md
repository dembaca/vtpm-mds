# Tasks

Written on 2026-10-18 against `main` at `407bfca` by
`agent-cursor-vtpm-22e0`, at the coordinator's assignment after the
maintainer asked for throttling against DoS from a VM and Hogan listed
unbounded token mint / token map as production-readiness point 8.
**Code only after the coordinator's ACK of this record.** Rebase behind
Mac's `quiet-the-log-and-record-issuance` (and `drop-the-connection-map` if
merged with it) before touching `server.go`.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — overrule or accept D-a to D-k in
      `design.md` (defaults in D-c and "no config keys" in D-f are the ones
      most likely to move), and verify the outcome is recorded there
- [ ] 1.2 **Coordinator** — ACK the record and the deltas on the bus before
      any code is written, and verify the ACK names this change

## 2. Implement

- [ ] 2.1 Add `internal/server/limit.go` with per-peer token, request and
      enroll budgets (D-c, D-d), the `429` answer (D-g), `/health` exemption
      (D-e), peer keying (D-a), and `ConnState` accounting for the connection
      cap (D-i), and verify with unit tests for every scenario of the three
      delta specs that do not need a live listener
- [ ] 2.2 Enforce per-peer live-token quota and the global emergency brake
      in `imds/token.go` (D-h: 32 per peer with oldest-of-peer eviction;
      65536 global → `429` + log), pass the minting peer into
      `GenerateToken`, and verify with token-store and handler tests for
      every scenario of "Bound Session Tokens Per Peer"
- [ ] 2.3 Wire one middleware wrap and the `ConnState` hook in
      `internal/server/server.go` (only those additions in that file), after
      rebasing onto Mac's merged log-change, and verify with a server-level
      test that a flood of limited paths returns `429` while `/health`
      returns `200`
- [ ] 2.4 For each new test, run it against the code before this change or
      mutate the new code, and verify it fails
- [ ] 2.5 Run `go vet` and `go test` on the packages touched, and
      `openspec validate --all --strict`, and verify all pass

## 3. Verify on the real systems

- [ ] 3.1 **Cloud lab** — from guest100, flood `PUT /latest/api/token` and
      a limited GET past the defaults, and verify `429` with body
      `Too many requests` and `Retry-After`; from the same guest under
      budget, mint once and read `instance-id`; say which layout was used
- [ ] 3.2 **Cloud lab** — open more than 16 concurrent connections from one
      guest (or a host netns peer) and verify further limited requests get
      `429` while `/health` stays `200`
- [ ] 3.3 **Cloud lab** — run one DevID enroll (start+finish) under budget
      and verify it still succeeds; flood enroll past 6/60s and verify
      `429` without a new certificate
- [ ] 3.4 Say what was not verified: Hogan/VM 399 under the new package,
      multi-guest fairness beyond two peers, and tuning the constants in
      production

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, mention the throttle defaults in
      `AGENTS.md` or `README.md` only if the Impact of a packaging/docs
      touch is already accepted, and verify
      `openspec validate --all --strict` is still clean
