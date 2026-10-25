# Tasks

Written on 2026-10-25 against `main@0a3d9d3` (v0.3.1) by
`agent-vtpm-hogan-d3e6cbc82`, from bgl-infra TASK 2.3c and the bgl-infra
coordinator's request for a 0.3.2 proposal. Files: `internal/devid/client.go`,
`internal/devid/renew.go`, their tests, `debian/devid-enroll.1` and this
record. Not the host package, not `AGENTS.md`/`README.md`.

Implementation approved by the maintainer; section 1 implemented on branch
`renew-when-the-devid-key-is-not-this-tpms`. The simulator tests are behind
the `tpmsim` build tag, as `srk_sim_test.go` and `ek_sim_test.go` are; run them
with `go test -tags tpmsim ./internal/devid/`.

## 1. Implement

- [x] 1.1 Add `ErrDevIDForeign` to `LoadDevID`: a blob that does not decode,
      or a format-1 `TPM2_Load` error with `TPM_RC_INTEGRITY`,
      `TPM_RC_BINDING` or `TPM_RC_SENSITIVE`, wrapped with `%w`; every other
      error stays distinct (D-c); verify with unit tests per response code,
      through a `ReadWriter` wrapper that injects a response to `TPM2_Load`
      `TestLoadDevIDForeignByResponseCode` (tpmsim, wrapper `loadAnswer`):
      `TPM_RC_INTEGRITY`, `_BINDING`, `_SENSITIVE` are `ErrDevIDForeign`;
      `_SIZE`, `_HANDLE`, `_FAILURE`, `_RETRY`, `_OBJECT_MEMORY` are
      `ErrDevIDLoad` and not foreign; an undecodable public blob is foreign
      with no TPM command; a load under another parent is foreign by the
      simulator's own answer (`parameter 1, error code 0x1f`).
      `TestIsForeignLoadError` (untagged) covers values, pointers and a
      wrapped error. go-tpm's legacy package returns these errors as values;
      both are matched. **Deviation from D-c's wording:** `TPM_RC_SENSITIVE`
      is a format-0 error (`RC_VER1 + 0x055`, `tpm2.Error`), not a format-1
      parameter error, so it is matched as `tpm2.Error{RCSensitive}`.
      `isTPMRetry` was not changed; go-tpm's `tpmutil` retries
      `TPM_RC_RETRY` itself (11 backoffs, then an unpack error), which is
      why the injected retry takes about 2 s.
- [x] 1.2 Check fit before the lifetime short-cut (D-a): missing or
      unparseable certificate, missing or empty blob, certificate key not
      equal to the `devid.pub.blob` key, `ErrDevIDForeign`; flush the loaded
      key on the not-due path; verify with unit tests for each reason and for
      fitting materials, through a key-factory seam
      (`ClientOptions.Factory`), since `ClientEnrollOrRenew` has no test today
      `TestRenewReasonsWithLoadableBlobs` (tpmsim, `ClientOptions.Factory`,
      a fake service with a real `Enroller`): fitting materials make no
      request, leave the files byte-identical and nothing loaded; `no
      certificate`, `unreadable certificate`, `due`, `certificate is for
      another VM`, `certificate does not match key` reuse the loaded key and
      keep its blobs; `missing blob` (absent and empty) makes a new key; a
      certificate with both another CN and another key logs the earlier
      reason. No transient handle is left after any run.
- [x] 1.3 Hand the loaded key to a `CreateSigningRequestWithDevID` variant that
      records it before the EK and AK steps; create a new key for foreign
      blobs; remove the unreachable `ErrDevIDLoad` retry (D-b); verify with a
      simulator test: blobs from another simulator instance plus a fresh
      certificate give a new key and a request, fitting blobs no request, and
      no command fails for object memory with three slots
      `TestRenewWithBlobsFromAnotherTPM` (tpmsim): blobs from a first
      simulator instance plus a fresh `CN=390` certificate over them, on a
      second instance, give one request with a new key, new blobs and a
      certificate over the new key, reason `key does not load in this TPM`;
      the next run makes no request and changes nothing. The run completes on
      the simulator's three object slots, nothing left loaded.
      `CreateSigningRequestWithKey` takes the loaded `KeyInfo` and records it
      before the EK and AK steps; `CreateSigningRequestWithDevID` stays as a
      wrapper over it. The `ErrDevIDLoad` retry in `ClientEnrollOrRenew` is
      gone.
- [x] 1.3a Read `instance-id` with an IMDSv2 token (PUT, then GET), three
      tries over ~10 s, only when a parseable certificate is present, and
      renew when the certificate's `CN` differs; fail the run when it cannot
      be read (D-e); verify with unit tests against an `httptest` server that
      serves both requests: same VM, other VM, `404`, no `i-`, token refused,
      no certificate (no metadata request)
      `TestReadVMID`: same VM, `404` (three tries, two pauses through the
      `instanceIDSleep` seam), no `i-`, `i-` alone, token refused;
      `TestUnreadableInstanceIDChangesNothing`: `404`, no `i-`, token refused
      fail the run with no TPM command and the files unchanged;
      `TestNoCertificateMakesNoMetadataRequest`: no metadata request. Other
      VM: `certificate is for another VM` in
      `TestRenewReasonsWithLoadableBlobs`. The helper sends
      `X-aws-ec2-metadata-token-ttl-seconds: 60` (the service ignores it) and
      `X-aws-ec2-metadata-token`, at most 1.5 s per try (token and read
      together), 3 s between tries, no proxy; `TestReadVMIDTryTimesOut`
      checks the per-try limit.
- [x] 1.4 Fail the run on any other TPM error and leave the files unchanged
      (D-c); verify with a unit test where `TPM2_Load` answers
      `TPM_RC_RETRY`: non-zero exit, no request, files unchanged
      `TestRenewTPMWarningChangesNothing` (tpmsim): `TPM_RC_RETRY` and
      `TPM_RC_OBJECT_MEMORY`, with a fresh and with a due certificate: error,
      not foreign, no request, files unchanged, no renewal logged. A blob that
      exists but cannot be read also fails the run now instead of counting
      as missing.
- [x] 1.5 Log the reason once per renewal (D-d); verify in the tests' log
      output
      `LogRenewReason` writes `devid-enroll: renewing <dir>: <reason>`, with
      the cause in parentheses for `unreadable certificate`, and for foreign
      blobs whatever the reason, ending `; new key)`
      (`TestRenewForAnotherVMWithBlobsFromAnotherTPM`); it replaces `LogRenewCorruptCert`. The reason tests
      assert exactly one such line with the expected reason, and none for
      fitting materials or a failed run.
- [ ] 1.6 Update `debian/devid-enroll.1` (including the `CN` check, that
      every run with a certificate reads `instance-id`, and the dependency on
      the host's `enable_ec2_compat`) and add the 0.3.2 entry to
      `debian/changelog`; verify with `man -l`
      Man page done: `man -l debian/devid-enroll.1` renders the fit checks,
      the seven reasons, the `instance-id` read on every run with a
      certificate, its three tries, the `enable_ec2_compat` dependency and
      the TPM-warning rule, and that a foreign key is logged as `new key`
      whatever the reason. **Open:** the 0.3.2 `debian/changelog` entry is
      left to the release branch. (The CN has been the VM ID since
      2026-10-03, so the CN check does not renew earlier certificates.)
- [x] 1.7 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`
      `go vet ./...` clean, also with `-tags tpmsim`; `go test ./...` ok for
      every package (`internal/devid` 97 s); `go test -tags tpmsim
      ./internal/devid/` ok (111 s); `openspec validate --all --strict`:
      Totals: 8 passed, 0 failed (8 items).

## 2. Verify on a guest

- [ ] 2.1 Build with `make deb-local`; `dpkg-deb -c`/`-I` and
      `lintian --fail-on error,warning` on `devid-enroll`
- [ ] 2.2 On VM 399 (maintainer's go), against a **scratch copy** of the
      materials via `-out`, so SPIRE's key and agent ID stay untouched: with
      fitting materials a run makes no request and leaves the files
      unchanged; with `devid.pub.blob` removed from the copy it renews with
      reason `missing blob`
- [ ] 2.3 On a clone with an empty vTPM and inherited materials (bgl-infra,
      as in 2.3c): the first boot renews with reason `certificate is for another
      VM` (it comes before the load in D-c) and a **new** key, because the
      inherited blobs do not load, before `spire-agent` starts, the new certificate has the
      clone's CN, and the agent attests with a new agent ID
- [ ] 2.4 On a clone that **kept** the source's vTPM state (bgl-infra): the
      first boot renews with reason `certificate is for another VM`; with
      `refuse-an-ek-pinned-to-another-vm` installed on that node the
      enrollment is refused and the files stay unchanged

## Pre-review

- Fable, 2026-10-25, of the proposal (`cea40ab`). Must-fix: `ErrDevIDLoad`
  wraps every load error, so a TPM warning would have read as a foreign key —
  D-c now defines foreign by response code with its own sentinel; task 2.2
  would have replaced 399's key — it now runs on a scratch copy. Should-fix
  taken: slot sequence and flushing in D-b, the twice-derived parent, all
  reasons in D-d and "cannot be parsed" in the spec, named test seams, the
  dead retry. Nits taken: keys compared as keys, changelog and man-page
  items. Its note on `isTPMRetry` is recorded as outside scope.
- Fable, 2026-10-25, of D-e (`70bb7c6`). Must-fix: `instance-id` always
  needs an IMDSv2 token — the helper and its test now do PUT and GET; task
  2.3 expected the wrong reason for the clone — it now expects `certificate
  is for another VM` with a new key, and D-c says the load still runs after
  an earlier reason. Should-fix taken: the `enable_ec2_compat` dependency,
  the read only with a parseable certificate, three tries before failing.
  Nits taken: `CN` in D-a's list; the changelog notes the one-time renewal.
- Fable, 2026-10-25, of the implementation (`25e7626`): no must-fix;
  should-fix taken: foreign cause logged with any reason, retry timing;
  nits taken: no proxy, man-page wording, 1.6 evidence.
- [x] Review of the implementation by a stronger model before the PR is
      called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change (after
      `make-guest-renewal-work-with-a-non-root-spire-agent`)
