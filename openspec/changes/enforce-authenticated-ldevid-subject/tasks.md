# Tasks

## 1. Reproduce the defect

- [x] 1.1 Add a test that enrolls as VM `100` with a platform identity carrying common name `other-name`, and verify the issued certificate's subject is currently `CN=other-name` — record that as the baseline the fix has to invert

## 2. Make the authenticated VM ID the common name

- [x] 2.1 Remove `subjectCNFromRequest` from `internal/devid/enroll.go` and drop the now-duplicated `subjectCN` parameter from `Enroller.Start` and the `SubjectCN` field from the session, and verify `go build ./...` succeeds and `HandleStart` no longer passes the VM ID twice
- [x] 2.2 Make `CA.IssueDevID` set the subject common name from the session's VM ID, replacing any common name in the platform identity RDN sequence, and verify the task 1.1 test now expects and gets `CN=100`
- [x] 2.3 Verify a request with no platform identity still yields `CN=100`, so the previous fallback path is unchanged
- [x] 2.4 Verify a platform identity carrying organization `Example GmbH` and organizational unit `lab` produces a subject with both preserved and `CN=100`

## 3. Confirm the derived values

- [x] 3.1 Verify the TCG SAN extension is not marked critical on a certificate issued to an authenticated caller, since the subject is never empty
- [x] 3.2 Verify the certificate shape is otherwise unchanged — `notAfter` `9999-12-31T23:59:59Z`, key usage digitalSignature, CA false, extended key usage `2.23.133.11.1.2`, TCG SAN with HardwareModuleName and PermanentIdentifier
- [x] 3.3 Verify the serial is still deterministic: enroll the same DevID key twice as the same VM and verify both certificates carry the same serial number

## 4. Confirm end to end

- [x] 4.1 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
- [x] 4.2 Run `TestEnrollAgainstSwtpm` against the lab software TPM and verify the issued certificate's subject is the VM ID passed to `Start`, not the `guest100` platform name the test requests — record explicitly whether the test ran or skipped
      RAN, did not skip. Run by Andreas Dembach as root on `hogan`, 2026-10-02:
        --- PASS: TestEnrollAgainstSwtpm (0.48s)
        DevID cert issued (1371 bytes PEM), subject "CN=100"
      The test requests the platform common name `guest100` and passes the VM id
      `100` to `Start`; the issued subject is `CN=100`, so the guest-supplied
      name was ignored against a real TPM.
      Environment note, because the repository does not produce it: the command
      socket had to be created by hand (`swtpm socket --server
      type=unixio,path=/var/lib/mds-lab/run/swtpm.sock …`), and
      `/etc/vtpm-mds/ek-chain.pem` had to be assembled from
      `/etc/ssl/certs/proxmox_tpm_ca.crt`, because `gen-lab-pki.sh` looks only
      under `/var/lib/swtpm-localca/` while this host issues EK certificates
      through the site CA named in `/etc/swtpm_setup.conf`. The TPM, the EK
      certificate and the enroll path itself were real.
- [ ] 4.3 Run `scripts/qemu-lab/e2e-devid-guest.sh` and verify the guest's `devid.crt.pem` reads `CN=<vm id>` with `openssl x509 -noout -subject` — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
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
