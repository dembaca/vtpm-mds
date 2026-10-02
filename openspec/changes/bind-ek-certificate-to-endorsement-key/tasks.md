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
- [x] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify a DevID certificate is still issued — if the swtpm socket is not present the test skips, so record explicitly whether it ran or skipped rather than implying it passed
      RAN, did not skip. Run by Andreas Dembach as root on `hogan`, 2026-10-02,
      against a freshly manufactured swtpm whose EK certificate was issued by
      `CN=BGL Proxmox TPM CA`:
        --- PASS: TestEnrollAgainstSwtpm (0.48s)
        DevID cert issued (1371 bytes PEM), subject "CN=100"
      A certificate was issued, so the EK binding holds against a real TPM: the
      test verifies the EK certificate against the endorsement key with
      `VerifyEKCertificateBinding` and then runs both enroll legs.
      Environment note, because the repository does not produce it: the command
      socket had to be created by hand (`swtpm socket --server
      type=unixio,path=/var/lib/mds-lab/run/swtpm.sock …`), and
      `/etc/vtpm-mds/ek-chain.pem` had to be assembled from
      `/etc/ssl/certs/proxmox_tpm_ca.crt`, because `gen-lab-pki.sh` looks only
      under `/var/lib/swtpm-localca/` while this host issues EK certificates
      through the site CA named in `/etc/swtpm_setup.conf`. The TPM, the EK
      certificate and the enroll path itself were real.
- [ ] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` end to end from a git-stamped package and verify the guest still obtains `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
      Attempted 2026-10-02 on `hogan` by Andreas Dembach and abandoned — not a
      defect of this change. `scripts/qemu-lab/` is the QEMU cloud lab: its
      `setup-host.sh` creates `br-imds` and claims 169.254.169.1/16 and
      169.254.169.254/16, which `hogan` already owns as /32 on the production
      `vmbr_imds`. Two connected routes for 169.254.0.0/16 result, and
      `ip route get 169.254.169.10` resolved via `vmbr_imds`, so the guest's
      SYN arrived on `br-imds` while the reply left through the other bridge:
      `devid-enroll.log` in the guest shows its IMDS NIC up at
      169.254.169.10/16 and then nothing but `curl: (28) Connection timed out`.
      The enrollment code was never reached. `br-imds` is runtime-only, so the
      conflict was removed by deleting it; `vmbr_imds` and the production IMDS
      were verified intact afterwards. Run this task on the cloud lab host
      instead. Also needed there, and worth fixing first: `cloud-localds` and
      an OVMF firmware path, neither of which the e2e script checks for.

## 5. Keep the capability description honest

- [ ] 5.1 When archiving, update the `## Purpose` of `openspec/specs/devid-enrollment/spec.md` so the EK factor is described as a certificate that chains to the configured EK CA **and** certifies the endorsement key being enrolled, and verify the archived spec no longer claims the factor is chain building alone
      Not run: archive-time task; the coordinator archives this change.

