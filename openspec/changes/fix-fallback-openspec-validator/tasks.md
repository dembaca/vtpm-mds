# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Run `scripts/openspec-validate.py --strict` and verify it reports `has at least one scenario` failures for REMOVED requirements in the six changes proposed on 2026-09-20, while `openspec validate --all --strict` reports all items passing — record both outputs as the baseline
- [ ] 1.2 Verify the CLI is genuinely available on this host by recording `openspec --version` and `node --version`, since `AGENTS.md` asserts it cannot be

## 2. Check each delta operation by its own rule

- [ ] 2.1 Split a delta document on its `## ADDED/MODIFIED/REMOVED/RENAMED Requirements` headings before collecting requirement blocks, and verify a delta using all four sections is parsed into the right buckets
- [ ] 2.2 Require a scenario only for ADDED and MODIFIED requirements, and verify the six proposed changes no longer report spurious scenario failures
- [ ] 2.3 Require `**Reason**` and `**Migration**` on every REMOVED requirement, and verify a REMOVED block missing either one is reported
- [ ] 2.4 Require `FROM:` and `TO:` on every RENAMED entry, and verify a malformed RENAMED entry is reported
- [ ] 2.5 Verify a living spec under `openspec/specs/` is still checked exactly as before, by confirming all five existing capability specs still pass

## 3. Enforce scenario preservation

- [ ] 3.1 For each MODIFIED requirement, compare its scenario names against the same requirement in `openspec/specs/<capability>/spec.md` and report any the delta drops, and verify the check fires on a deliberately truncated copy of a MODIFIED block
- [ ] 3.2 Verify the check stays silent for a MODIFIED requirement that does not exist in the living spec, so a delta that modifies a requirement added by an earlier unarchived change is not falsely reported

## 4. Confirm the two checkers agree

- [ ] 4.1 Run both `openspec validate --all --strict` and `scripts/openspec-validate.py` over the whole repository and verify they agree on every change and spec, with the script's only remaining `--strict` failures being unticked tasks
- [ ] 4.2 Verify the script still exits non-zero when a change has unticked tasks under `--strict`, since that is what the flag is for

## 5. Correct the contributor instructions

- [ ] 5.1 Update `AGENTS.md` step 4 and the "Lab host facts" section to record that the OpenSpec CLI is installed on `hogan` and is the check to run there, keeping the script as the fallback for a host without node, and verify the file no longer claims the CLI cannot be installed on lab hosts
- [ ] 5.2 Correct the same claim in `README.md`, where it appears as a comment above the `scripts/openspec-validate.py` invocation, and verify `grep -rn 'no node/npm' README.md AGENTS.md` returns nothing
