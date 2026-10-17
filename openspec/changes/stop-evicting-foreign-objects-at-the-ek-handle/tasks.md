# Tasks

Written on 2026-10-17 against `main` at `4b5ed44`, by `agent-mac-vtpm-mds-687cc7`
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record and the maintainer's decision in `design.md`.**

## 1. Decide

- [x] 1.1 **Maintainer** — decide what the client does with a persistent object at `0x81010001` (`design.md`, option 1 recommended), and verify the decision is written under a dated "Decided" heading in `design.md`
      **Done, 2026-10-17:** option 1, "1.1: 1" on the agent bus; recorded under "Decided" in `design.md`.
- [x] 1.2 Verify on hogan that the persistent object at `0x81010001` on VM 399 is, or is not, the key in its EK certificate at NV `0x01c00002`, and that a transient default-template EK equals it, with persistent handles unchanged and no transient handle left
      Readonly, as the measuring binary of the SRK change was. Say which.
      **Done, 2026-10-17, coordinator on hogan** (VM 399, `swtpm 0.8.0+pve3`,
      `libtpms 0.9.7`, site-CA layout), read-only and transient: the EK derived
      from `client.DefaultEKTemplateRSA()` through go-tpm in the endorsement
      hierarchy, the persistent object at `0x81010001` and the public key in the
      EK certificate at NV `0x01c00002` (`CN=399`) are **the same key**
      (`952f9c28...f23a23fb`), and `MatchesTemplate` is true. Afterwards no
      transient handle was left, the persistent objects at `0x81000001`,
      `0x81010001` and `0x81010016` were unchanged and the measuring program was
      deleted. On hogan a derived EK therefore holds the binding. A foreign
      object at `0x81010001` does not affect a derived EK, which depends only on
      the seed and the template; the binding would fail only for an option that
      uses the persistent object.

## 2. Implement

- [x] 2.1 Make `CreateEK` a transient primary in the endorsement hierarchy from `client.DefaultEKTemplateRSA()`, never persistent, and verify with a simulator test that a persistent object at `0x81010001` (a storage key, and an EK-template key with another `unique`) is byte-identical afterwards and that the EK in the signing request equals the transient EK
      **Done:** `CreateEK` is `createPrimaryKey(rw, tpm2.HandleEndorsement, f.EKTemplate)`.
      `TestForeignObjectAtEKHandleIsLeftAlone` (two cases: a storage key, and an
      EK-template key with another `unique`) and `TestDerivedEKEqualsPersistentEK`
      pass on the simulator (`-tags tpmsim`).
- [x] 2.2 Flush the EK after reading its public area, derive it again in `Activate` and flush it after use, and verify with a simulator test that the whole flow (create, signing request, activation) never fails for object memory and leaves no transient handle
      **Done:** `CreateSigningRequest` flushes the EK after reading its public
      area; `RequestResources.Activate` derives it again and flushes it.
      `TestEnrollmentFlowFitsThreeObjectSlots` (EK, AK, DevID under a transient
      parent, a real `CreateChallenge`, `Activate`, `Flush`) passes: two
      transient objects before and after activation, none after `Flush`. This
      test guards the flush logic of the new design; it also passes on the old
      `CreateEK`, whose persistent EK took no slot, so only the probe in
      `design.md` (the fourth `CreatePrimary` fails) shows why the EK is not held.
- [x] 2.3 Remove `cachedPrimary` and the stale comment in `RequestResources.Flush`, and verify `go build ./...`, `go vet ./...` and `go test ./...` pass, quoting the result
      **Done:** `cachedPrimary` is removed (no caller left), the comment in
      `Flush` is replaced. `go vet` clean and `go test -count=1` `ok` for
      `cmd/devid-enroll`, `imds`, `internal/...` and the root, with the untracked
      `cmd/prox-mds` leftover left out; `go test -tags tpmsim ./internal/devid/`
      `ok`.
- [x] 2.4 Verify the new tests fail on `main`'s `keys.go` (the foreign object is replaced; the foreign key is used), by running them against the old file, so they are shown to catch the defect
      **Done:** with `keys.go` put back to `HEAD` (the old `CreateEK` and
      `cachedPrimary`) both cases of `TestForeignObjectAtEKHandleIsLeftAlone`
      fail: `the object at 0x81010001 changed` for the storage key, `CreateEK
      returned the persistent handle, not a transient EK` for the EK-template key
      with another `unique`. With the new `keys.go` they pass. The other two new
      tests do not fail on the old code, as said under 2.2.
- [x] 2.5 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on real artifacts

      **Done:** `openspec validate --all --strict` → 6 passed, 0 failed. The
      fallback validator's only problem is `all tasks ticked (--strict)`: 3.1,
      3.2 and 4.1 are open until the lab and hogan have run.
- [x] 3.1 **Cloud lab** — a stock client from this branch, fresh vTPM, with a foreign persistent object pre-created at `0x81010001`: verify enrollment succeeds, SPIRE `1.14.0` attests, and the object's public area is byte-identical before and after
      Say which layout was used (`swtpm-localca`, `/usr/share/OVMF/`).
      **Done, 2026-10-17, `agent-cursor-vtpm-22e0`, against `77cfeaa`:** passed.
      Layout `swtpm-localca` and `/usr/share/OVMF/OVMF_CODE_4M.fd`, `swtpm 0.7.3`,
      `libtpms0 0.9.3`, SPIRE `1.14.0`, stock `devid-enroll` from this commit, no
      lab patch. Setup: a fresh guest vTPM, the manufactured EK at `0x81010001`
      evicted, a **foreign storage primary** (`userwithauth|noda|restricted|decrypt`)
      made persistent there, the EK certificate at NV `0x01c00002` unchanged.
      Enrollment exited 0, with `enroll/start` and `enroll/finish` both `200`, so
      **`ActivateCredential` with the newly derived transient EK worked**. **No
      object-memory error on swtpm**: the flush design keeps the peak at three or
      fewer. The foreign object's public area and name were **byte-identical**
      before and after (public-area SHA-256 `617512e7...95c5635`), and afterwards
      only `0x81010001` and `0x81010016` were persistent. SPIRE `tpm_devid`
      attested the agent (`.../tpm_devid/4307f45a...`, `subject:cn:100`, agent
      healthy).
- [x] 3.2 **hogan** — with a git-stamped package, verify on VM 399's TPM, into a scratch directory, that the persistent objects at `0x81000001`, `0x81010001` and `0x81010016` are unchanged and the enrollment succeeds
      Back up `/var/lib/spire/agent/` first, and do not re-enroll VM 399.
      **Done, 2026-10-17, coordinator on hogan, against `77cfeaa`:** passed.
      `make deb-local` gave `devid-enroll_0.2.0+git20261017.77cfeaa52496`,
      lintian clean; `go vet` and `go test -count=1 ./...` green without
      `tpmsim`. The binary was extracted from the `.deb` and run in the guest,
      not installed; the daemon stays `...5efb34ba374a`. `devid-enroll -tpm
      /dev/tpmrm0 -out /tmp/enroll42.d/out` exited 0. The daemon journal shows
      `enroll/start 200` and `enroll/finish 200`, so **`ActivateCredential` with
      the newly derived EK worked** and the EK binding held. Certificate
      `CN=399`. `readpublic` hashes of `0x81000001`, `0x81010001` and
      `0x81010016` and the lockout counter (`0x0`) were identical before and
      after, no transient handle was left, and there were **no object-slot
      problems** on `libtpms 0.9.7`. Loading the new blobs as SPIRE does: OK
      under the H-1 template, `0x1f` under the old 256 x `0x00` template and the
      256 x `0x01` control. The SPIRE material and the running `spire-agent` were
      untouched, and the scratch directory was deleted.
      **Not shown on hogan:** a foreign object at `0x81010001`. The real, matching
      EK sits there, and placing another object would endanger the pilot; 3.1
      shows it in the lab.

## 4. Archive

- [ ] 4.1 Archive with `openspec archive` and verify `openspec validate --all --strict` is still clean and `devid-enrollment` states how the EK is derived
