# Tasks

Written on 2026-10-19 against `main@da0083a` by the coordinator, from the
maintainer's E2.5. Files: `internal/devid/ca.go`, `internal/devid/devid_test.go`,
`debian/devid-enroll.1`, one example line in the root `README.md` and this
change directory. **Not** `cmd/devid-enroll` code, other `debian/` files,
`AGENTS.md`.

## 1. Decide

- [x] 1.1 **Maintainer** — subject is `CN=<VMID>` only, guest attributes dropped
      silently. **Done, 2026-10-19 19:32** (E2.5: "E2.5 akzeptiert").

## 2. Implement

- [x] 2.1 Build the subject in `IssueDevID` from the VM ID alone; verify by
      reading the diff that `sr.PlatformIdentity` is no longer read there.
      `subj := pkix.Name{CommonName: subjectCN}`; `FillFromRDNSequence` is gone.
- [x] 2.2 Replace `TestIssueDevIDPreservesOrganizationAndOU` with a test that a
      platform identity carrying `O`, `OU`, `C` and a foreign `CN` yields a
      subject whose RDN sequence is exactly one attribute, `CN=100`; keep the
      existing CN and empty-identity tests; verify `go test ./internal/devid/`.
      `TestIssueDevIDDropsPlatformIdentityAttributes` (O, OU, C, foreign CN →
      one RDN, `CN=100`, one raw attribute); fails on the old `ca.go`.
- [x] 2.3 Verify the serial is identical for two issuances of the same key with
      different platform identities (new test).
      `TestIssueDevIDSerialIgnoresPlatformIdentity`; fails on the old `ca.go`.
- [x] 2.4 Run `go vet` and `go test` on `./internal/... ./imds .` and
      `openspec validate --all --strict`; verify all green.
      vet clean; test ok for config, devid, inventory, proxmox, server, imds;
      strict 6 passed. A trial `openspec archive` on a scratch copy left the
      new requirement and no trace of the old one, strict 5 passed.
- [x] 2.5 Fable pre-review (2026-10-19): no must-fix. Should-fixes done: the
      `-cn` entry in `debian/devid-enroll.1` and the `subject=` example in
      `README.md` now say the subject is always `CN=<VM ID>`; the `Finish` test
      carries an organization too. The lifetime record it could not find is
      PR #71. Left as is: the new requirement lands at the end of the spec
      after archive; `subjectIsEmpty` in `ca.go` is now always false (later
      cleanup).

## 3. Verify

- [x] 3.1 Lab: enrol a guest whose request carries `O`/`OU` (a test client or a
      patched request) and show with `openssl x509 -noout -subject` that the
      certificate reads `subject=CN=<vmid>`; SPIRE `1.14.0` still attests.
      **Subject half done, 2026-10-19 20:05, cloud lab (`agent-cursor-vtpm-22e0`)
      against `3e102c3`:** a request carrying `O=Example GmbH`, `OU=lab`,
      `CN=other-name` got a certificate whose
      `openssl x509 -noout -subject -nameopt RFC2253` is `CN=100` and nothing
      else (`/mnt/shared/devid-out-70-oo/devid.crt.pem` on guest100). At 20:07
      the same PEM and blobs were installed under `/opt/spire/conf/agent/`,
      `agent-data.json` cleared and the agent restarted: SPIRE `1.14.0` logged
      `Node attestation was successful`, `reattestable=true`, agent ID
      `…/tpm_devid/6c5253955e5386ca87bd53d9b4e3f794d980a85f` (the SHA-1 of that
      PEM's DER).
- [x] 3.2 hogan: after the install, enrol VM 399 into a scratch directory and
      show the subject is `CN=399`; SPIRE's material under
      `/var/lib/spire/agent/` unchanged.
      **Done, 2026-10-19 22:28, hogan (`agent-vtpm-hogan-d3e6cbc82`)** against
      `0.2.0+git20261019.e487c6bf62a6` (`e487c6b` on `main@01fd5f1`), installed
      with the maintainer's go: `try-restart` gap 0.23 s, `DevID CA key from
      systemd credential`; `vtpm-mds-bind` re-applied the same rules. VM 399
      enrolled to `/root/devid-scratch-70`; `openssl x509 -noout -subject
      -nameopt RFC2253` reads `subject=CN=399`; the journal shows `devid issued
      serial=7233ad01… vm=399` with the seeded EK pin matching. The sha256 of
      `/var/lib/spire/agent/devid.*` is the same before and after. lintian
      `--fail-on error,warning` exits 2 on one warning only,
      `debian-changelog-line-too-long`, from the branch name that
      `scripts/deb-local.sh` stamps into the changelog, not from this change.
- [x] 3.3 Say what was not verified. On hogan, VM 399's request carried no
      `O`/`OU` (only the lab run in 3.1 did), and SPIRE on VM 399 was not
      re-attested with the new certificate (SPIRE attestation is shown only in
      the lab). Certificates issued before this change keep their old subject
      until the guest enrols again.

## 4. Archive

- [x] 4.1 After merge, the coordinator archives and checks the order against
      `limit-the-ldevid-lifetime-and-renew-it` (proposal, "Apply order").
      Archived 2026-10-19 on `main@5653949` first; strict 6 passed.
      `limit-the-ldevid-lifetime-and-renew-it` (PR #71) archives second and
      rebuilds its `MODIFIED` block against the new requirement name and text.
