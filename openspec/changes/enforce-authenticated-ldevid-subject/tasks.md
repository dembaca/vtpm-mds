# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Add a test that enrolls as VM `100` with a platform identity carrying common name `other-name`, and verify the issued certificate's subject is currently `CN=other-name` — record that as the baseline the fix has to invert

## 2. Make the authenticated VM ID the common name

- [ ] 2.1 Remove `subjectCNFromRequest` from `internal/devid/enroll.go` and drop the now-duplicated `subjectCN` parameter from `Enroller.Start` and the `SubjectCN` field from the session, and verify `go build ./...` succeeds and `HandleStart` no longer passes the VM ID twice
- [ ] 2.2 Make `CA.IssueDevID` set the subject common name from the session's VM ID, replacing any common name in the platform identity RDN sequence, and verify the task 1.1 test now expects and gets `CN=100`
- [ ] 2.3 Verify a request with no platform identity still yields `CN=100`, so the previous fallback path is unchanged
- [ ] 2.4 Verify a platform identity carrying organization `Example GmbH` and organizational unit `lab` produces a subject with both preserved and `CN=100`

## 3. Confirm the derived values

- [ ] 3.1 Verify the TCG SAN extension is not marked critical on a certificate issued to an authenticated caller, since the subject is never empty
- [ ] 3.2 Verify the certificate shape is otherwise unchanged — `notAfter` `9999-12-31T23:59:59Z`, key usage digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, TCG SAN with HardwareModuleName and PermanentIdentifier
- [ ] 3.3 Verify the serial is still deterministic: enroll the same DevID key twice as the same VM and verify both certificates carry the same serial number

## 4. Confirm end to end

- [ ] 4.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [ ] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify the issued certificate's subject is the VM ID passed to `Start`, not the `guest100` platform name the test requests — record explicitly whether the test ran or skipped
- [ ] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` and verify the guest's `devid.crt.pem` reads `CN=<vm id>` with `openssl x509 -noout -subject` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
