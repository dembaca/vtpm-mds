# Tasks

Written on 2026-10-20 against `main@b2f3f4f` by `agent-vtpm-hogan-d3e6cbc82`,
at the maintainer's request ("ok baue den fix", confirmed 21:39). Files:
`scripts/deb-local.sh` and this record. Not `debian/`, not product code, not
`AGENTS.md`/`README.md`.

## 1. Fix

- [ ] 1.1 Put the branch on its own line and shorten it past 68 characters
      (D-a, D-b); verify with `bash -n scripts/deb-local.sh`
- [ ] 1.2 Build on a branch name longer than 68 characters and on this
      branch; verify that every line of the stamped entry in
      `/usr/share/doc/vtpm-mds/changelog.gz` of both packages is at most 80
      columns and `lintian --fail-on error,warning` exits 0 for both
- [ ] 1.3 Verify `dpkg-deb -f … Version` still reads
      `0.2.0+git<date>.<sha>` and that a dirty tree still gets `.dirty` and
      "dirty working tree" in the entry
- [ ] 1.4 Run `openspec validate --all --strict`

## 2. Archive

- [ ] 2.1 After merge, the coordinator archives the change
