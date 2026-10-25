# Tasks

Written on 2026-10-25 against `main@0a3d9d3` (v0.3.1) by
`agent-vtpm-hogan-d3e6cbc82`, from bgl-infra TASK 2.3c and the bgl-infra
coordinator's request for a 0.3.2 proposal. Files: `internal/devid/client.go`,
`internal/devid/renew.go`, their tests, `debian/devid-enroll.1` and this
record. Not the host package, not `AGENTS.md`/`README.md`.

**Proposal only: implementation waits for the maintainer's go.**

## 1. Implement

- [ ] 1.1 Add `ErrDevIDForeign` to `LoadDevID`: a blob that does not decode,
      or a format-1 `TPM2_Load` error with `TPM_RC_INTEGRITY`,
      `TPM_RC_BINDING` or `TPM_RC_SENSITIVE`, wrapped with `%w`; every other
      error stays distinct (D-c); verify with unit tests per response code,
      through a `ReadWriter` wrapper that injects a response to `TPM2_Load`
- [ ] 1.2 Check fit before the lifetime short-cut (D-a): missing or
      unparseable certificate, missing or empty blob, certificate key not
      equal to the `devid.pub.blob` key, `ErrDevIDForeign`; flush the loaded
      key on the not-due path; verify with unit tests for each reason and for
      fitting materials, through a key-factory seam
      (`ClientOptions.Factory`), since `ClientEnrollOrRenew` has no test today
- [ ] 1.3 Hand the loaded key to a `CreateSigningRequestWithDevID` variant that
      records it before the EK and AK steps; create a new key for foreign
      blobs; remove the unreachable `ErrDevIDLoad` retry (D-b); verify with a
      simulator test: blobs from another simulator instance plus a fresh
      certificate give a new key and a request, fitting blobs no request, and
      no command fails for object memory with three slots
- [ ] 1.3a Read `instance-id` with an IMDSv2 token (PUT, then GET), three
      tries over ~10 s, only when a parseable certificate is present, and
      renew when the certificate's `CN` differs; fail the run when it cannot
      be read (D-e); verify with unit tests against an `httptest` server that
      serves both requests: same VM, other VM, `404`, no `i-`, token refused,
      no certificate (no metadata request)
- [ ] 1.4 Fail the run on any other TPM error and leave the files unchanged
      (D-c); verify with a unit test where `TPM2_Load` answers
      `TPM_RC_RETRY`: non-zero exit, no request, files unchanged
- [ ] 1.5 Log the reason once per renewal (D-d); verify in the tests' log
      output
- [ ] 1.6 Update `debian/devid-enroll.1` (including the `CN` check, that
      every run with a certificate reads `instance-id`, and the dependency on
      the host's `enable_ec2_compat`) and add the 0.3.2 entry to
      `debian/changelog` (certificates from before the `CN`-only subject
      renew once); verify with `man -l`
- [ ] 1.7 Run `go vet ./...`, `go test ./...` and
      `openspec validate --all --strict`

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
- [ ] Review of the implementation by a stronger model before the PR is
      called ready

## 3. Archive

- [ ] 3.1 After merge, the coordinator archives the change (after
      `make-guest-renewal-work-with-a-non-root-spire-agent`)
