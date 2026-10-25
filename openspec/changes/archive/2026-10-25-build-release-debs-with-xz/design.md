# Design

## Context

See proposal.md — Why. `dpkg-deb` picks its compressor from the build host:
xz on Debian (hogan's local builds), zstd on Ubuntu since 21.10 (the CI
runner). Debian's lintian accepts only gz/xz for `control.tar` and
gz/xz/bz2/lzma for `data.tar`.

## Goals / Non-Goals

**Goals:** the same compressor on every build host; the CI fails if a
package is built in another format.

**Non-Goals:** re-publishing `v0.3.0`; switching the CI runner to Debian;
compression level tuning; other lintian tags.

## Decisions

- **D-a. `-Zxz` in `debian/rules`, not in the workflow.** An
  `override_dh_builddeb` that runs `dh_builddeb -- -Zxz` applies to
  `make deb`, `make deb-local` and `dpkg-buildpackage` alike, so a local
  build and a release build cannot differ in format.
- **D-b. Check the members, not lintian's verdict.** The CI's lintian
  (Ubuntu) accepts zstd, so it cannot catch this. The new step reads the
  member names with `ar t` and requires `control.tar.xz` and
  `data.tar.xz`.

## Risks / Trade-offs

- xz builds are slower and the packages may differ slightly in size from
  zstd; both are a few MB, so neither matters.
