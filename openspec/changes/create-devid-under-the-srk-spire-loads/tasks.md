# Tasks

Nothing is ticked: this is a decision record, written on 2026-10-17 against
`main` at `b77b259` by `agent-mac-vtpm-mds-0172be`, on the coordinator's
assignment. No code has been touched. Code follows only after the
coordinator's ACK on this record, on the same branch, in `internal/devid/` and
this change directory.

**Do not re-enroll VM 399** until task 1.1 is answered. Its SPIRE agent
attests with material that works.

## 1. Settle the facts and the decision

- [ ] 1.1 On hogan, read the public area of the persistent object at `0x81000001` on VM 399's vTPM (`tpm2_readpublic -c 0x81000001`, in the guest) and verify whether its `unique` is empty, and record the output in `design.md`
      This tests the hypothesis in `proposal.md`: that VM 399 attests because
      its SRK was made with an empty `unique` and `cachedPrimary` reused it.
      Read-only. If the `unique` is **not** empty, stop and report: the account
      of why VM 399 works is then wrong or incomplete, and the design needs
      rethinking before any code.
- [ ] 1.2 **Maintainer** — decide what the client does with a persistent object at `0x81000001` (option 1, 2 or 3 in `design.md`), and verify the answer is recorded there under a dated "Decided" heading
      The delta is written for option 1. If another option is chosen, the
      delta changes with it.
- [ ] 1.3 Check that SPIRE `1.14.0` is the version to design against and that the helper is unchanged in the version the maintainer plans to run next, and verify the finding is recorded in `design.md`
      The record compared tags `v1.14.0`, `v1.15.4` and `main` on 2026-10-17.
      Redo that comparison when the next release is picked.

## 2. Implement, after the coordinator's ACK on this record

- [ ] 2.1 Create the DevID key and the attestation key under a transient owner-hierarchy primary made from the H-1 template, and flush it afterwards, and verify with a unit test that the template has an empty `ModulusRaw` and equals SPIRE's `SRKTemplateHighRSA()`
      `SRKTemplateHighRSA()` is `client.SRKTemplateRSA()` with
      `RSAParameters.ModulusRaw = []byte{}`. Compare the two as values; do not
      hard-code one from the other.
- [ ] 2.2 Remove the use of `cachedPrimary` and `EvictControl` for the SRK, and verify with `git grep` that nothing evicts or persists the SRK handle, and with a swtpm test that a persistent object at `0x81000001` survives an enrollment with the same public area
      The EK keeps `cachedPrimary`; this task must not change it.
- [ ] 2.3 Add a test that loads the enrolled blobs the way SPIRE does, with `CreatePrimaryEx` in the owner hierarchy from the H-1 template followed by `tpm2.Load`, and verify it passes, and that the same load under the 256-zero-byte template fails with the integrity error
      The failing half is what shows the test tests something. Run it against
      swtpm in `TestEnrollAgainstSwtpm`'s harness.
- [ ] 2.4 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the output
- [ ] 2.5 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on the real artifacts

- [ ] 3.1 **Cloud lab** — enroll a guest with a **fresh vTPM** using the stock client from this branch, with no lab-only patch, and verify SPIRE `1.14.0` attests it and `spire-server agent show` lists the selectors
      This is the observation the lab could not make on 2026-10-17: it needed
      a lab-only binary. Say which layout was used (default `swtpm-localca`,
      `/usr/share/OVMF/`).
- [ ] 3.2 **Cloud lab** — repeat with a persistent object pre-created at `0x81000001` and verify it is untouched and SPIRE still attests
- [ ] 3.3 **hogan** — with a git-stamped package, verify on VM 399's TPM that the persistent objects at `0x81000001`, `0x81010001` and `0x81010016` are unchanged after an enrollment run into a **scratch directory**, not into `/var/lib/spire/agent/`
      The scratch directory keeps the working pilot material in place. Back up
      `/var/lib/spire/agent/` on the guest before any run that could replace
      it, and say so.
- [ ] 3.4 **Maintainer/hogan** — only after 3.3, decide whether VM 399 is re-enrolled, and verify the decision is recorded here, with a note that its SPIFFE ID changes if it is
      Re-enrolling replaces the certificate and so its fingerprint, which is
      the agent ID. Nothing else about VM 399 changes in this task.

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, and verify `openspec validate --all --strict` is still clean and `devid-enrollment` now states the parent key
