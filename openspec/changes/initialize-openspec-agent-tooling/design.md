# Design

## Context

See proposal.md — Why.

`openspec init` is non-destructive in the shape this repository is in. Probing
it against an exported copy of the branch showed it adding only command and
skill files, and reporting `Config: openspec/config.yaml (exists)` rather than
rewriting it. `AGENTS.md` and `.cursor/rules/openspec-workflow.mdc` were
untouched by both `--tools cursor` and `--tools claude`.

The two agents spell the same commands differently, because each follows its
own convention: Cursor takes a flat `.cursor/commands/opsx-propose.md`, Claude
Code takes `.claude/commands/opsx/propose.md` and renders it as `/opsx:propose`.

## Goals / Non-Goals

**Goals:**

- The commands `README.md` documents exist.
- Both agents in use on this checkout get the same workflows.
- The project's own rules stay authoritative over the generated ones.

**Non-Goals:**

- Moving process rules out of `AGENTS.md` into generated files. The generated
  files describe how to drive the CLI; `AGENTS.md` describes what this project
  requires on top of it — never leaving work uncommitted, verifying on real
  artifacts, the lab host facts. Those are not OpenSpec's to own.
- Rewriting `.cursor/rules/openspec-workflow.mdc`.
- Adding the six non-core workflows. See the decision below.
- Initializing for any of the other thirty-odd supported agents.
- Pinning or vendoring the OpenSpec CLI version.

## Decisions

### Initialize for both Cursor and Claude Code

The repository's git author is `cursor-agent` and `.cursor/rules/` is tracked,
so Cursor is in use. The current session is Claude Code, and `README.md`
documents the `/opsx:` spelling that only the Claude Code generator produces.
Initializing for one would leave the other without commands or leave the README
wrong.

`--tools cursor,claude` writes both sets. They are independent directories, so
neither constrains the other, and a third agent can be added later without
touching what is there.

### Take the core profile and correct the README, not the other way round

The core profile ships six workflows: propose, apply, archive, explore, sync
and update. `README.md` promises `verify`, which is one of six further
workflows the CLI offers only through an interactive picker —
`openspec config profile` rejects every preset but `core` non-interactively.

Rather than drive an interactive picker to make a documentation line true, the
line is corrected to the commands that exist. Adding the extra workflows stays
available to a maintainer at a terminal, and costs nothing to defer: the
verification step they wrap is `openspec validate --all --strict`, which this
project already runs directly and names in `AGENTS.md`.

### Commit the generated files

`.gitignore` excludes neither `.claude/` nor `.cursor/`, and
`.cursor/rules/openspec-workflow.mdc` is already tracked. Committing the
generated files means every contributor and every agent session gets the same
workflow definitions, and a change to them shows up in review like any other.
The alternative — expecting each contributor to run `openspec init` themselves
— reintroduces exactly the drift this change is fixing.

### Correct the `AGENTS.md` premise rather than just the task list

The bullet is removed together with the claim that it needs root, because a
wrong reason is what kept the task queued. `fix-fallback-openspec-validator`
corrects the neighbouring wrong claim about node/npm in the same file; the two
are kept as separate changes because they fix different things, and both say so.

## Risks / Trade-offs

- **[Risk] The generated workflow files contradict `AGENTS.md`, and an agent
  follows the wrong one** → Mitigation: they address different layers, and
  `AGENTS.md` step 1 still sends a reader to the change documents first. The
  generated files were read before committing; the planning boundary they
  declare ("this workflow creates planning artifacts only") is stricter than
  `AGENTS.md`, not in conflict with it.
- **[Risk] A future `openspec init` run overwrites the generated files with a
  newer CLI's versions, silently changing how agents behave** → Accepted, and
  visible: they are tracked, so the diff appears in review. That is the reason
  to commit them rather than generate them per contributor.
- **[Risk] `/opsx:verify` stays absent and the next reader looks for it** →
  Mitigation: the README line is corrected in this change, so it no longer
  promises it.
- **[Trade-off] Twenty-four generated files enter the repository for tooling
  that is not the product** → Accepted; `.cursor/rules/openspec-workflow.mdc`
  set that precedent and the project is explicitly spec-driven.

## Migration Plan

None to deploy. A contributor with the checkout open in Cursor or Claude Code
restarts the IDE, or the session, to pick up the new commands — the CLI says so
on completion.

Rollback is deleting the generated directories and restoring the two
documentation lines; nothing depends on them at build or run time.

## Open Questions

None.
