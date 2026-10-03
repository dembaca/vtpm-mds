# Tasks

## 1. Reproduce the defect

- [x] 1.1 Add a test that enrolls as VM `100` with a platform identity carrying common name `other-name`, and verify the issued certificate's subject is currently `CN=other-name` — record that as the baseline the fix has to invert
  - Verification: `go test ./internal/devid/ -run TestGuestSuppliedCommonNameIsUsed -v` → PASS; issued CN=`other-name` (pre-fix baseline). Test renamed to `TestGuestSuppliedCommonNameIsIgnored` after the fix.

## 2. Make the authenticated VM ID the common name

- [x] 2.1 Remove `subjectCNFromRequest` from `internal/devid/enroll.go` and drop the now-duplicated `subjectCN` parameter from `Enroller.Start` and the `SubjectCN` field from the session, and verify `go build ./...` succeeds and `HandleStart` no longer passes the VM ID twice
  - Verification: `go build ./...` succeeds; `rg subjectCNFromRequest|SubjectCN internal/devid/` empty; `HandleStart` calls `e.Start(requestData, sig, vmid, ekFP)`
- [x] 2.2 Make `CA.IssueDevID` set the subject common name from the session's VM ID, replacing any common name in the platform identity RDN sequence, and verify the task 1.1 test now expects and gets `CN=100`
  - Verification: `go test ./internal/devid/ -run TestGuestSuppliedCommonNameIsIgnored -v` → PASS; CN=`100`
- [x] 2.3 Verify a request with no platform identity still yields `CN=100`, so the previous fallback path is unchanged
  - Verification: `go test ./internal/devid/ -run TestIssueDevIDVMIDWhenNoPlatformIdentity -v` → PASS
- [x] 2.4 Verify a platform identity carrying organization `Example GmbH` and organizational unit `lab` produces a subject with both preserved and `CN=100`
  - Verification: `go test ./internal/devid/ -run TestIssueDevIDPreservesOrganizationAndOU -v` → PASS

## 3. Confirm the derived values

- [x] 3.1 Verify the TCG SAN extension is not marked critical on a certificate issued to an authenticated caller, since the subject is never empty
  - Verification: `go test ./internal/devid/ -run TestIssueDevIDSANNotCritical -v` → PASS
- [x] 3.2 Verify the certificate shape is otherwise unchanged — `notAfter` `9999-12-31T23:59:59Z`, key usage digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, TCG SAN with HardwareModuleName and PermanentIdentifier
  - Verification: `go test ./internal/devid/ -run TestIssueDevIDCertificateShape -v` → PASS
- [x] 3.3 Verify the serial is still deterministic: enroll the same DevID key twice as the same VM and verify both certificates carry the same serial number
  - Verification: `go test ./internal/devid/ -run TestIssueDevIDSerialDeterministic -v` → PASS

## 4. Confirm end to end

- [x] 4.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
  - Verification: `go vet ./...` exit 0; `go test ./... -count=1` → all packages ok (internal/devid ~6.6s)
- [x] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify the issued certificate's subject is the VM ID passed to `Start`, not the `guest100` platform name the test requests — record explicitly whether the test ran or skipped
  - Verification: RAN on the cloud lab host, 2026-10-03. Output:
        --- PASS: TestEnrollAgainstSwtpm (0.55s)
        DevID cert issued (1371 bytes PEM) subject=CN=100
    The signing request asks for platform CN `guest100`; `Start` is called with
    VM id `100`; the issued subject is `CN=100`.
- [x] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` and verify the guest's `devid.crt.pem` reads `CN=<vm id>` with `openssl x509 -noout -subject` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
  - Verification: RAN on the cloud lab host, 2026-10-03, as root via
    `sudo ./scripts/qemu-lab/e2e-devid-guest.sh` (TCG). Guest oneshot passes
    `-cn guest100`. Observed:
        subject=CN = 100
        issuer=CN = vtpm-mds Lab DevID CA, O = vtpm-mds
        GUEST DEVID E2E PASSED
    `openssl x509 -noout -subject` on
    `/var/lib/mds-lab/vms/guest100/shared/devid-out/devid.crt.pem` reports
    `CN = 100` (the inventory VM id), not `guest100`.
