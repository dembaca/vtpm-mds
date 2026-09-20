# Design

## Context

See proposal.md — Why.

`check_requirements` does:

```python
blocks = re.split(r"^### Requirement: ", text, flags=re.M)[1:]
for block in blocks:
    scenarios = re.split(r"^#### Scenario: ", block, flags=re.M)[1:]
    check(bool(scenarios), ...)
```

The document's `##` section headings are never consulted, so the function
cannot know whether a block is being added, modified or removed. It was written
when no change in the repository had removed a requirement, so the case never
came up.

The script's own docstring is honest about its role: "This is a floor, not a
replacement: run the real CLI where it exists." The CLI does exist here, which
makes the floor cheap to correct and cheap to keep.

## Goals / Non-Goals

**Goals:**

- The fallback agrees with `openspec validate --all --strict` on the deltas in
  this repository.
- Each delta operation is checked against the rule that actually applies to it.
- The scenario-preservation rule is enforced, because losing a scenario at
  archive time is the failure with the longest fuse.

**Non-Goals:**

- Reimplementing the CLI. The script stays a floor for a host without node, not
  a second source of truth.
- Validating the prose of a requirement, its length, or its wording.
- Changing the `--strict` behaviour of failing on unticked tasks. That is what
  the flag is for, and a change in flight is expected to fail it until its
  tasks are done.
- Vendoring the CLI, adding a node dependency, or changing how it is installed.

## Decisions

### Parse the `##` sections, then dispatch per operation

The document is split on `^## (ADDED|MODIFIED|REMOVED|RENAMED) Requirements`
first, and requirement blocks are collected per section. Each operation gets
its own rule:

- `ADDED` and `MODIFIED`: at least one `#### Scenario:`, each using
  GIVEN/WHEN/THEN.
- `REMOVED`: a `**Reason**` and a `**Migration**` line, and no scenario
  requirement.
- `RENAMED`: `FROM:` and `TO:`.

A living spec under `openspec/specs/` keeps the existing rule unchanged, since
it has no sections.

Special-casing only `REMOVED` while leaving the flat split in place was
rejected: the same blindness would return the first time a change uses
`RENAMED`.

### Add the scenario-preservation check

For each `MODIFIED` block, if `openspec/specs/<capability>/spec.md` has a
requirement of that name, every `#### Scenario:` name in the living spec must
appear in the modified block. This is the rule the CLI enforces with the
message "archive refuses to drop them", and it is the one a hand-written delta
is most likely to break — a partial MODIFIED silently loses behaviour when the
change is archived.

It is added here rather than in a separate change because it is the same
function, the same parse, and a fallback that misses the most consequential
rule is not much of a floor.

### Correct `AGENTS.md` rather than delete the fallback

The script is still the right answer on a host without node, and the
instruction to run it is still correct there. What is wrong is the assertion
that no lab host can have the CLI. `AGENTS.md` states which is available on
`hogan` and keeps both paths.

## Risks / Trade-offs

- **[Risk] The fallback and the CLI drift again as the CLI gains rules** →
  Accepted and inherent to having a second checker. Mitigation: the script's
  docstring already says the CLI wins where it exists, and `AGENTS.md` now says
  the CLI is what to run here, so the fallback is the exception rather than the
  default path.
- **[Risk] The new scenario-preservation check rejects a delta the CLI accepts,
  or the reverse** → Mitigation: the acceptance criterion is agreement with
  `openspec validate --all --strict` on every change and spec currently in the
  repository, including the six proposed on 2026-09-20, which between them use
  ADDED, MODIFIED and REMOVED.
- **[Trade-off] The script grows a dependency on the living specs, which it
  previously read only to check them** → Accepted; the check cannot be made
  without comparing against them, and the script already walks
  `openspec/specs/`.

## Migration Plan

None. A development-time checker and a contributor document; nothing is
deployed.

## Open Questions

None.
