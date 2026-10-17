# Tasks

Nothing is ticked: this is a decision record, written on 2026-10-17 against
`main` at `b77b259` by `agent-mac-vtpm-mds-0172be`, on the coordinator's
assignment. No code has been touched. Code follows only after the
coordinator's ACK on this record, on the same branch, in `internal/devid/` and
this change directory.

**Do not re-enroll VM 399** until task 3.3 is done and 3.4 decides it. Its
SPIRE agent attests with material that works.

## 1. Settle the facts and the decision

- [x] 1.1 On hogan, find out why VM 399's SRK works with SPIRE, and verify the finding is recorded in `proposal.md`
      **Done** on 2026-10-17 by `agent-vtpm-hogan-ead994124`, in two passes. The
      first, through tpm2-tools, found that the persistent object at
      `0x81000001` has the same modulus as the empty-`unique` primary, so it is
      SPIRE's H-1 SRK. The second, task 1.5 through go-tpm, showed the client's
      template gives a different parent there. VM 399's pilot material most
      likely came from a tpm2-tools script of 2026-09-14, not from the client.
- [ ] 1.2 **Maintainer** — decide what the client does with a persistent object at `0x81000001` (option 1, 2 or 3 in `design.md`), and verify the answer is recorded there under a dated "Decided" heading
      The delta is written for option 1. If another option is chosen, the
      delta changes with it.
- [ ] 1.3 Check that SPIRE `1.14.0` is the version to design against and that the helper is unchanged in the version the maintainer plans to run next, and verify the finding is recorded in `design.md`
      The record compared tags `v1.14.0`, `v1.15.4` and `main` on 2026-10-17.
      Redo that comparison when the next release is picked.
- [x] 1.4 **Cloud lab** — report the `swtpm` and `libtpms` versions of the lab TPM and compare the templates there, and verify the output is recorded in `proposal.md`
      **Done** on 2026-10-17 by `agent-cursor-vtpm-22e0`: `swtpm 0.7.3`,
      `libtpms0 0.9.3`. Through tpm2-tools the lab, like hogan, does **not**
      tell an empty and a 256-zero `unique` apart; through go-tpm,
      `SRKTemplateRSA` and `SRKTemplateHighRSA` give different primaries on the
      same guest. So tpm2-tools is unsuited to compare these templates, and the
      defect does not depend on which TPM it is.
- [x] 1.5 **hogan** — create transient owner primaries through **go-tpm** from `client.SRKTemplateRSA()` and from the template with an empty `ModulusRaw` on VM 399's TPM, flush them, and verify whether their moduli differ and whether the empty one equals the persistent object, with the output recorded in `proposal.md`
      **Done** on 2026-10-17 by `agent-vtpm-hogan-ead994124`, with a static
      measuring binary: the client template gives `b71a7c39…`, the empty one
      `e36d99c0…`, equal to the persistent object; a control differs. No
      transient handle was left, the persistent objects were unchanged, and the
      binary and temporary files were removed. How the 2026-09-14 material was
      made is circumstantial: a tpm2-tools script, certificate and script
      minutes apart; the certificate is RSA while the visible `tpm2_create`
      lines are ECC.

## 2. Implement, after the coordinator's ACK on this record

- [ ] 2.1 Create the DevID key and the attestation key under a transient owner-hierarchy primary made from the H-1 template, and flush it afterwards, and verify with a unit test that the template has an empty `ModulusRaw` and equals SPIRE's `SRKTemplateHighRSA()`
      `SRKTemplateHighRSA()` is `client.SRKTemplateRSA()` with
      `RSAParameters.ModulusRaw = []byte{}`. Compare the two as values; do not
      hard-code one from the other.
- [ ] 2.2 Remove the use of `cachedPrimary` and `EvictControl` for the SRK, and verify with `git grep` that nothing evicts or persists the SRK handle, and with a swtpm test that a persistent object at `0x81000001` survives an enrollment with the same public area
      The EK keeps `cachedPrimary`; this task must not change it.
- [ ] 2.3 Add a test that loads the enrolled blobs the way SPIRE does, with `CreatePrimaryEx` in the owner hierarchy from the H-1 template followed by `tpm2.Load`, and verify it passes
      For the negative control, load the same blobs under a primary made from a
      template whose `unique` is **certainly** different, 256 bytes of `0x01`,
      and verify that load fails with the integrity error. That is what shows the
      test tests something, and it holds on every TPM. Do **not** use
      "256 zero bytes" as the control when driven through tpm2-tools: both
      `libtpms` versions measured give the same key as the empty one there.
      Name the `swtpm` and `libtpms` versions of the harness in the output.
- [ ] 2.4 Run `go vet ./...` and `go test ./...` and verify both pass, quoting the output
- [ ] 2.5 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on the real artifacts

- [ ] 3.1 **Cloud lab** — enroll a guest with a **fresh vTPM** using the stock client from this branch, with no lab-only patch, and verify SPIRE `1.14.0` attests it and `spire-server agent show` lists the selectors
      This is the observation the lab could not make on 2026-10-17: it needed
      a lab-only binary. Say which layout was used (default `swtpm-localca`,
      `/usr/share/OVMF/`).
- [ ] 3.2 **Cloud lab** — repeat with a persistent object pre-created at `0x81000001` and verify it is untouched and SPIRE still attests
- [ ] 3.3 **hogan** — with a git-stamped package, verify on VM 399's TPM that the persistent objects at `0x81000001`, `0x81010001` and `0x81010016` are unchanged after an enrollment run into a **scratch directory**, not into `/var/lib/spire/agent/`
      The measuring binary of 1.5 can show the defect on hogan, and this run
      shows that nothing is broken. The persistent SRK there already is the H-1
      primary.
      The scratch directory keeps the working pilot material in place. Back up
      `/var/lib/spire/agent/` on the guest before any run that could replace
      it, and say so.
- [ ] 3.4 **Maintainer/hogan** — only after 3.3, decide whether VM 399 is re-enrolled, and verify the decision is recorded here, with a note that its SPIFFE ID changes if it is
      Re-enrolling replaces the certificate and so its fingerprint, which is
      the agent ID. Nothing else about VM 399 changes in this task.

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, and verify `openspec validate --all --strict` is still clean and `devid-enrollment` now states the parent key
