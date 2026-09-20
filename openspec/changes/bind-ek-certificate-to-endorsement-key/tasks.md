# Tasks

## 1. Reproduce the defect

- [x] 1.1 Add a test that builds a signing request carrying one TPM's endorsement key while presenting a different but CA-chained EK certificate in the header and in the request, and verify that against the current code `enroll/start` succeeds — record that as the baseline the fix has to invert
- [x] 1.2 Verify the counterpart passes today too: a signing request whose endorsement key matches its EK certificate enrolls successfully, so the test distinguishes the attack from legitimate use

## 2. Make the binding checkable

- [x] 2.1 Split `VerifyEKCertificate` in `internal/devid/verify.go` into a chain-only function and one that also binds the certificate to an endorsement key public area, removing the `_ = pub` discard, and verify `go build ./...` succeeds with no call site passing a nil key to the binding form
- [x] 2.2 Implement the comparison through `crypto.PublicKey.Equal` and verify unit tests cover a matching RSA key, a different RSA key, and a nil endorsement key public area
- [x] 2.3 Verify `grep -rn 'optional future' internal/devid` returns nothing, so the comment that documented the gap is gone with it

## 3. Enforce it on enroll/start

- [x] 3.1 Pass the signing request's endorsement key into `AuthenticateEnrollCaller` from `HandleStart` and perform the binding check there, and verify the task 1.1 test now expects and gets `401`
- [x] 3.2 Verify the response body is exactly `EK certificate does not match the endorsement key in the signing request` and that the failure is logged as `devid enroll auth:`
- [x] 3.3 Verify no enroll session is created by the rejected request, by asserting the session store is empty after it
- [x] 3.4 Verify a signing request that carries an endorsement certificate but no endorsement key public area is rejected with the same `401`, and not with `create challenge: missing EK public`
- [x] 3.5 Verify `enroll/finish` still verifies the chain only and that finishing with a different trusted EK certificate is still rejected with `EK certificate does not match enroll session`

## 4. Confirm legitimate enrollment is unchanged

- [x] 4.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [ ] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify a DevID certificate is still issued — if the swtpm socket is not present the test skips, so record explicitly whether it ran or skipped rather than implying it passed
      Not verified: `go test ./internal/devid/ -run TestEnrollAgainstSwtpm -v` SKIPPED —
      `/var/lib/mds-lab/run/swtpm.sock` is absent here, so no DevID certificate was
      issued and the swtpm path is unverified.
- [ ] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` end to end from a git-stamped package and verify the guest still obtains `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
      Not run: needs root on the lab host; maintainer-run.

## 5. Keep the capability description honest

- [ ] 5.1 When archiving, update the `## Purpose` of `openspec/specs/devid-enrollment/spec.md` so the EK factor is described as a certificate that chains to the configured EK CA **and** certifies the endorsement key being enrolled, and verify the archived spec no longer claims the factor is chain building alone
      Not run: archive-time task; the coordinator archives this change.

