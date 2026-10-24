# Design

## Context

See proposal.md — Why. lintian's `debian-changelog-line-too-long` fires for
any changelog line longer than 80 characters. The stamped first line is
`  * Local lab build from git ` (29) + 12-character SHA + ` (` + branch +
`, dirty working tree` + `).`, so any branch longer than about 35 characters
breaks it.

## Goals / Non-Goals

**Goals:** every line of the stamped entry fits 80 columns for any branch
name; the entry still says which commit, which branch and whether the tree
was dirty.

**Non-Goals:** the version string; `make deb` and released changelogs; other
lintian tags.

## Decisions

- **D-a. Branch on its own line.** `  * Local lab build from git <sha>[,
  dirty working tree].` then `    Branch: <branch>`. The first line is at
  most 62 columns.
- **D-b. Shorten what still does not fit.** `    Branch: ` is 12 columns, so
  a branch longer than 68 characters is cut to its first 67 and ends in `~`.
  The SHA identifies the tree; the branch is a hint for humans.
- **D-c. No new dependency.** Plain bash parameter expansion.

## Risks / Trade-offs

- A cut branch name is ambiguous in the changelog; the SHA is not.
