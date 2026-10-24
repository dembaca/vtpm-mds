# Proposal

## Why

`scripts/deb-local.sh` stamps a local build's changelog entry with one line:

    * Local lab build from git <sha> (<branch>[, dirty working tree]).

The branch name goes into that line unshortened. With a long branch, the line
exceeds lintian's 80 columns, and `lintian --fail-on error,warning` — the
check AGENTS.md requires on every built `.deb` — fails with
`debian-changelog-line-too-long` on both packages. On 2026-10-19/20 that
happened for every build of `issue-the-ldevid-subject-from-the-vm-id-only`
(89 columns), `limit-the-ldevid-lifetime-and-renew-it` and
`issue-ldevids-from-an-intermediate-per-cluster`, so each of those packages
was reported as failing lintian for a reason unrelated to the change being
built. The maintainer asked for the fix on 2026-10-20.

## What Changes

- The stamped entry puts the commit on the first line and the branch on its
  own line, and shortens a branch name that would still exceed the limit, so
  every line of the entry stays within 80 columns whatever the branch is
  called.
- The version string, the `.dirty` marker and the "Not for publication" line
  are unchanged; `dpkg -l vtpm-mds` and `vtpm-mds -version` identify a build
  exactly as before.

## Impact

- `scripts/deb-local.sh` only. No product code, no `debian/` file, no change
  to released builds (`make deb`).
- Spec: `debian-packaging` gains a requirement for local builds.
