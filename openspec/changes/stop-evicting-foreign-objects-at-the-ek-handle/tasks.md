# Tasks

Written on 2026-10-17 against `main` at `4b5ed44`, by `agent-mac-vtpm-mds-687cc7`
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record and the maintainer's decision in `design.md`.**

## 1. Decide

- [ ] 1.1 **Maintainer** — decide what the client does with a persistent object at `0x81010001` (`design.md`, option 1 recommended), and verify the decision is written under a dated "Decided" heading in `design.md`
- [ ] 1.2 Verify on hogan that the persistent object at `0x81010001` on VM 399 is, or is not, the key in its EK certificate at NV `0x01c00002`, and that a transient default-template EK equals it, with persistent handles unchanged and no transient handle left
      Readonly, as the measuring binary of the SRK change was. Say which.

## 2. Implement

- [ ] 2.1 Make `CreateEK` a transient primary in the endorsement hierarchy from `client.DefaultEKTemplateRSA()`, never persistent, and verify with a simulator test that a persistent object at `0x81010001` (a storage key, and an EK-template key with another `unique`) is byte-identical afterwards and that the EK in the signing request equals the transient EK
- [ ] 2.2 Flush the EK after reading its public area, derive it again in `Activate` and flush it after use, and verify with a simulator test that the whole flow (create, signing request, activation) never fails for object memory and leaves no transient handle
- [ ] 2.3 Remove `cachedPrimary` and the stale comment in `RequestResources.Flush`, and verify `go build ./...`, `go vet ./...` and `go test ./...` pass, quoting the result
- [ ] 2.4 Verify the new tests fail on `main`'s `keys.go` (the foreign object is replaced; the foreign key is used), by running them against the old file, so they are shown to catch the defect
- [ ] 2.5 Run `openspec validate --all --strict` and `scripts/openspec-validate.py --strict` and verify both pass

## 3. Verify on real artifacts

- [ ] 3.1 **Cloud lab** — a stock client from this branch, fresh vTPM, with a foreign persistent object pre-created at `0x81010001`: verify enrollment succeeds, SPIRE `1.14.0` attests, and the object's public area is byte-identical before and after
      Say which layout was used (`swtpm-localca`, `/usr/share/OVMF/`).
- [ ] 3.2 **hogan** — with a git-stamped package, verify on VM 399's TPM, into a scratch directory, that the persistent objects at `0x81000001`, `0x81010001` and `0x81010016` are unchanged and the enrollment succeeds
      Back up `/var/lib/spire/agent/` first, and do not re-enroll VM 399.

## 4. Archive

- [ ] 4.1 Archive with `openspec archive` and verify `openspec validate --all --strict` is still clean and `devid-enrollment` states how the EK is derived
