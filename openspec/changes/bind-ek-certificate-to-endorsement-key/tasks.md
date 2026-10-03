# Tasks

## 1. Reproduce the defect

- [x] 1.1 Add a test that builds a signing request carrying one TPM's endorsement key while presenting a different but CA-chained EK certificate in the header and in the request, and verify that against the current code `enroll/start` succeeds — record that as the baseline the fix has to invert
  - Verification (`go test ./internal/devid/ -run TestEnrollStartAcceptsMismatchedEKBaseline -v -count=1`): PASS before the fix — enroll/start returned 200 with a session_id when the EK cert public key disagreed with the signing request endorsement key. Expectation inverted to `TestEnrollStartRejectsMismatchedEK` after the fix.
- [x] 1.2 Verify the counterpart passes today too: a signing request whose endorsement key matches its EK certificate enrolls successfully, so the test distinguishes the attack from legitimate use
  - Verification (`go test ./internal/devid/ -run TestEnrollStartAcceptsMatchingEK -v -count=1`): PASS — matching EK gets 200.

## 2. Make the binding checkable

- [x] 2.1 Split `VerifyEKCertificate` in `internal/devid/verify.go` into a chain-only function and one that also binds the certificate to an endorsement key public area, removing the `_ = pub` discard, and verify `go build ./...` succeeds with no call site passing a nil key to the binding form
  - Verification: `go build ./...` OK. Split into `VerifyEKCertificateChain` + `VerifyEKCertificateBound`; `_ = pub` removed.
- [x] 2.2 Implement the comparison through `crypto.PublicKey.Equal` and verify unit tests cover a matching RSA key, a different RSA key, and a nil endorsement key public area
  - Verification (`go test ./internal/devid/ -run TestVerifyEKCertificateBoundKeys -v -count=1`): PASS.
- [x] 2.3 Verify `grep -rn 'optional future' internal/devid` returns nothing, so the comment that documented the gap is gone with it
  - Verification: `grep -rn 'optional future' internal/devid` → no matches.

## 3. Enforce it on enroll/start

- [x] 3.1 Pass the signing request's endorsement key into `AuthenticateEnrollCaller` from `HandleStart` and perform the binding check there, and verify the task 1.1 test now expects and gets `401`
  - Verification (`go test ./internal/devid/ -run TestEnrollStartRejectsMismatchedEK -v -count=1`): PASS — 401.
- [x] 3.2 Verify the response body is exactly `EK certificate does not match the endorsement key in the signing request` and that the failure is logged as `devid enroll auth:`
  - Verification: same test asserts exact body and `devid enroll auth:` log prefix — PASS.
- [x] 3.3 Verify no enroll session is created by the rejected request, by asserting the session store is empty after it
  - Verification: same test asserts empty session store — PASS.
- [x] 3.4 Verify a signing request that carries an endorsement certificate but no endorsement key public area is rejected with the same `401`, and not with `create challenge: missing EK public`
  - Verification (`go test ./internal/devid/ -run TestEnrollStartRejectsMissingEndorsementKey -v -count=1`): PASS.
- [x] 3.5 Verify `enroll/finish` still verifies the chain only and that finishing with a different trusted EK certificate is still rejected with `EK certificate does not match enroll session`
  - Verification (`go test ./internal/devid/ -run TestEnrollFinishStillChainOnlyAndSessionBound -v -count=1`): PASS.

## 4. Confirm legitimate enrollment is unchanged

- [x] 4.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
  - Verification: `go vet ./...` → VET_OK. `go test ./... -count=1` → all packages ok (`internal/devid` 7.339s).
- [x] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify a DevID certificate is still issued — if the swtpm socket is not present the test skips, so record explicitly whether it ran or skipped rather than implying it passed
  - Verification: RAN on the cloud lab host, 2026-10-03. Command socket at
    `/var/lib/mds-lab/run/swtpm.sock` (swtpm `--server type=unixio` +
    `--flags not-need-init,startup-clear`). Output:
        --- PASS: TestEnrollAgainstSwtpm (0.55s)
        DevID cert issued (1371 bytes PEM) subject=CN=100
- [x] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` end to end from a git-stamped package and verify the guest still obtains `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
  - Verification: RAN on the cloud lab host, 2026-10-03, as root via
    `sudo ./scripts/qemu-lab/e2e-devid-guest.sh` (TCG). Guest oneshot used
    `-cn guest100`; enroll/start and enroll/finish both returned 200.
    Materials present under `/var/lib/mds-lab/vms/guest100/shared/devid-out/`:
    `devid.crt.pem`, `devid.priv.blob`, `devid.pub.blob`. Log ended with
    `GUEST DEVID E2E PASSED`. Note: the e2e script builds `bin/vtpm-mds` from
    this tree rather than installing a `.deb`; the binary under test is this
    commit.

## 5. Keep the capability description honest

- [ ] 5.1 When archiving, update the `## Purpose` of `openspec/specs/devid-enrollment/spec.md` so the EK factor is described as a certificate that chains to the configured EK CA **and** certifies the endorsement key being enrolled, and verify the archived spec no longer claims the factor is chain building alone
