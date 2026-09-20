# Tasks

## 1. Establish that the premise in AGENTS.md is wrong

- [ ] 1.1 Verify `openspec init` needs no root here by confirming the checkout root grants `coding-agent` write access, with `getfacl -p .` and a create/remove test in the repository root
- [ ] 1.2 Verify the run is non-destructive by exporting the branch to a scratch directory, running `openspec init --tools cursor` and `--tools claude` there, and confirming `git status` reports only added command and skill files — in particular that `AGENTS.md`, `.cursor/rules/openspec-workflow.mdc` and `openspec/config.yaml` are unmodified

## 2. Initialize

- [ ] 2.1 Verify the working tree is clean with `git status --porcelain` before running anything that writes, as `AGENTS.md` requires
- [ ] 2.2 Run `openspec init --tools cursor,claude` in the checkout and verify it reports setup complete for both agents
- [ ] 2.3 Verify `git status --porcelain` lists only new files under `.cursor/commands/`, `.cursor/skills/`, `.claude/commands/` and `.claude/skills/`, and that no tracked file is modified

## 3. Confirm the commands the documentation promises

- [ ] 3.1 Verify `.claude/commands/opsx/` contains `propose.md`, `apply.md`, `archive.md`, `explore.md`, `sync.md` and `update.md`, which Claude Code renders as `/opsx:propose` and so on
- [ ] 3.2 Verify `.cursor/commands/` contains the six matching `opsx-*.md` files
- [ ] 3.3 Verify no `verify` workflow is present in either set, confirming the README line has to be corrected rather than the profile changed

## 4. Correct the documentation

- [ ] 4.1 Correct `README.md` line 182 to name the six commands that exist, and verify the file no longer mentions `/opsx:verify`
- [ ] 4.2 Remove the `openspec init` bullet from the maintainer list in `AGENTS.md`, and verify the file no longer claims the task needs root

## 5. Confirm nothing else moved

- [ ] 5.1 Run `openspec validate --all --strict` and verify every change and spec still passes
- [ ] 5.2 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
