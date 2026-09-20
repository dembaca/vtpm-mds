# Proposal

## Why

`openspec init` has never been run in this checkout, so the slash commands the
project documents do not exist. `README.md` line 182 tells a reader to use
`/opsx:propose`, `/opsx:apply`, `/opsx:verify` and `/opsx:archive` "in an
OpenSpec-aware agent", and none of them is defined anywhere in the repository.

The consequence is not cosmetic. The workflow definitions those commands carry
are what make an agent follow the repository's own process — write the proposal
before the spec delta, treat the delta as the contract, stop at the end of
planning instead of starting to implement. Without them every contributor
reconstructs the process from `AGENTS.md` prose, which is how the last two
sessions worked and why `AGENTS.md` has to repeat the workflow at such length.

`AGENTS.md` lists this as work that needs the maintainer because it needs root
on the lab host. It does not: the checkout's root directory carries an ACL
granting `coding-agent` write access, and `openspec init` writes only inside
the checkout. That premise is wrong in the same way as the claim that lab hosts
have no node/npm, which `fix-fallback-openspec-validator` corrects.

## What Changes

- `openspec init --tools cursor,claude` is run, adding command and skill
  definitions for both agents: `.cursor/commands/` and `.cursor/skills/`,
  `.claude/commands/opsx/` and `.claude/skills/`.
- Cursor gets `/opsx-propose` and friends; Claude Code gets `/opsx:propose` and
  friends, which is the spelling `README.md` already documents.
- The `verify` workflow is added to the six of the core profile, so all four
  commands `README.md` names exist. It is not in the core profile, but it is
  installable without an interactive picker by appending it to the `workflows`
  list in the OpenSpec global config and switching that config to the `custom`
  profile.
- **`README.md` line 182 is corrected** to name all seven installed commands and
  to say how the five that are not installed can be added.
- Nothing already in the repository is overwritten. `AGENTS.md`,
  `.cursor/rules/openspec-workflow.mdc` and `openspec/config.yaml` are left as
  they are, so the project's own rules keep precedence and the generated files
  own only the command and skill definitions, exactly as `AGENTS.md` asks.
- `AGENTS.md` loses the bullet listing this as a maintainer task needing root.
- No change to the daemon, the client, the packaging, the specs, or any change
  already in flight.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change adds agent tooling and corrects a documentation line; no
behaviour of the service changes, so `.openspec.yaml` sets `skip_specs: true`.

## Impact

- New: `.cursor/commands/` (7 files), `.cursor/skills/` (7 directories),
  `.claude/commands/opsx/` (7 files), `.claude/skills/` (7 directories).
- `README.md`: one line.
- `AGENTS.md`: the maintainer bullet about `openspec init`.
- `.gitignore` does not exclude `.claude/` or `.cursor/`, so all generated
  files are committed and every contributor gets the same commands.
- Anyone opening this checkout in Cursor or Claude Code.
