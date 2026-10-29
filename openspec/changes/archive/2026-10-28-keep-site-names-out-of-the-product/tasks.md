# Tasks

Written on 2026-10-26 against `main@e54344d` by the coordinator, at the
maintainer's request (thread `yz8nmijh7i8ifmjqhraqzk13zy`). Files: those in
the proposal's Impact. Not `internal/devid/clusterpin_test.go` or
`internal/devid/pin.go`, which `read-cluster-ek-pins-from-pmxcfs` (PR #90)
neutralised itself.

## 1. Rename

- [x] 1.1 Verify with vet and tests after replacing the site names in tests
      under `imds/` and `internal/` with D-a's names
      `go vet ./...` clean, `go test ./...` all seven packages `ok` (Mac
      worktree). The VM is `350` rather than a lower ID because several
      tests depend on it sorting between `300` and `500`
      (`TestGuestNICsReadsEveryModelWithIndexAndBridge`, the render order of
      `tap<vmid>i<n>`).
- [x] 1.2 Verify with the grep of 1.5 after replacing them in comments in
      `internal/proxmox/vmconfig.go` and the simulator tests, in
      `debian/devid-enroll.1`, and in the prose of `scripts/deb-local.sh`,
      `scripts/openspec-validate.py` and `scripts/qemu-lab/README.md`
- [x] 1.3 Verify with the grep of 1.5 after rewriting the README's E2E cheat
      sheet with placeholders (`<vmid>`, `<guest>`, and `<DevID CA>` for the
      CA's subject)
- [x] 1.4 Write the spec deltas per D-b; verify with
      `openspec validate --all --strict` on the lab host
      Each requirement block extracted from `openspec/specs/` with the
      substitutions of D-a and nothing else. On the lab host (OpenSpec
      1.13.1, worktree of `ccf1d6a`): `Totals: 6 passed, 0 failed (6 items)`,
      only INFO notes on long requirements; `go test ./...` all `ok`. On the
      Mac, OpenSpec 1.14.3 reports the change valid but fails `--all` on
      `main` as well, because it warns on long requirement texts.
- [x] 1.5 Verify the grep is empty outside D-c's exceptions and the two files
      PR #90 owns:
      `git grep -nIiwE 'hogan|holger|bgl|bgl-cluster|DUS[0-9]?|keller(01)?|vtpm-pilot|399' -- ':!openspec/changes' ':!debian/changelog' ':!openspec/specs' ':!internal/devid/clusterpin_test.go' ':!internal/devid/pin.go'`
      and
      `git grep -niE 'bc:24:11:(75:da:c0|06:1d:b2)' -- ':!openspec/changes'`
      Both print nothing. `openspec/specs/` keeps the old names until the
      archive applies the deltas.
- [x] 1.6 Verify with the grep of 1.5 after reducing `AGENTS.md` to rules and
      still-valid pitfalls (D-d)
      `AGENTS.md` is 147 lines instead of 810; the removed lines 110–810 went
      to the operator as a file for their infrastructure repository.

## Pre-review

- [x] Verify with a review by a stronger model before the PR is called ready
      **Fable, 2026-10-26, of `51cfb21`:** no must-fix; spec deltas are pure
      renames (scenario counts unchanged), tests faithful, vet/test green.
      Should-fix taken: `gofmt` on `fingerprint_test.go`; `ARCHITECTURE.md`
      named in the requirement. Should-fix 3 (the `qm guest exec` ASCII rule
      is new in `AGENTS.md`) goes to the maintainer with the merge ask. Nits
      taken: D-a rows for `bgl` as region and the third MAC; task order; the
      README's journal grep and the ignored `-cn`; two still-valid pitfalls
      back in `AGENTS.md` (`e2e-netns.sh` needs the YAML inventory; `301`/`307`
      by Go version). Not taken: `169.254.169.10` in the README (an address,
      not a site name).

- [x] 1.7 After rebasing on `main@972c635` (PR #90 merged), verify with the
      grep of 1.5 without the two PR #90 exclusions, `go vet ./...`,
      `go test ./...`, and a word diff of each new block against
      `openspec/specs/`
      Found two more requirements with `399` (`devid-enrollment` renewal,
      `operability` access line) and PR #90's delta; restated per D-b. The
      grep prints nothing; vet clean, all packages `ok` (Mac); the word diffs
      show only `399` → `350` and, in PR #90's delta, `390` → `340`.

## 2. Archive

- [x] 2.1 After merge, the coordinator archives the change; verify with
      `openspec validate --all --strict` and the grep of 1.5 without the
      `openspec/specs` exclusion. Until `read-cluster-ek-pins-from-pmxcfs` is
      archived too, the only expected hits are in
      `openspec/specs/devid-enrollment/spec.md` under `Refuse An EK Pinned To
      Another VM`
      Archived with `openspec archive` (OpenSpec 1.14.3, Mac) on
      `main@a966b58`: `+ 1, ~ 6`; the spec diff is the ADDED requirement and
      the D-a renames only. The grep prints exactly the eight expected lines
      (`devid-enrollment` 1087–1119) and the MAC grep nothing. On the lab
      host (OpenSpec 1.13.1, `2fa99a8`): `Totals: 6 passed, 0 failed
      (6 items)`, only INFO notes on long requirements. The Mac's 1.14.3
      fails every spec on long requirement texts, `main` included.
