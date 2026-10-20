# Tasks

Written on 2026-10-20 against `main@b2f3f4f` by `agent-vtpm-hogan-d3e6cbc82`,
at the maintainer's request ("ok baue den fix", confirmed 21:39). Files:
`scripts/deb-local.sh` and this record. Not `debian/`, not product code, not
`AGENTS.md`/`README.md`.

## 1. Fix

- [x] 1.1 Put the branch on its own line and shorten it past 68 characters
      (D-a, D-b); verify with `bash -n scripts/deb-local.sh`
      `bash -n` OK (`666a522`).
- [x] 1.2 Build on a branch name longer than 68 characters and on this
      branch; verify that every line of the stamped entry in
      `/usr/share/doc/vtpm-mds/changelog.gz` of both packages is at most 80
      columns and `lintian --fail-on error,warning` exits 0 for both
      Done 2026-10-20 on hogan, `666a522`: on this branch the entry's lines
      are 68/42/50/75 columns; on an 86-character branch the branch line is
      cut to exactly 80 (`…-builds-stays~`); `lintian --fail-on
      error,warning` exit 0 for `vtpm-mds` and `devid-enroll` in every
      build (before the fix: `debian-changelog-line-too-long`, exit 2).
- [x] 1.3 Verify `dpkg-deb -f … Version` still reads
      `0.2.0+git<date>.<sha>` and that a dirty tree still gets `.dirty` and
      "dirty working tree" in the entry
      `0.2.0+git20261020.666a522b45a3`; with an untracked file
      `0.2.0+git20261020.666a522b45a3.dirty` and `* Local lab build from git
      666a522b45a3, dirty working tree.` (62 columns), lintian exit 0.
- [x] 1.4 Run `openspec validate --all --strict`
      Totals: 7 passed, 0 failed (7 items)

## Pre-review

- Fable, 2026-10-20 (`409e09e`): no must-fix. Should-fix: the requirement
  promised every line of the entry, while the script bounds only the lines
  under the `*` item (the only ones lintian measures); wording narrowed.
  Nits: first line 62 columns, not 63; characters are counted under
  `C.UTF-8`, so a non-ASCII branch is not cut mid-codepoint in a C locale.
- [ ] 1.5 After the pre-review fix, rebuild on an 86-character branch under
      `LC_ALL=C`; verify lintian exit 0 and the branch line at 80 columns

## 2. Archive

- [ ] 2.1 After merge, the coordinator archives the change
