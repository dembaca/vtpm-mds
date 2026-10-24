# Proposal

## Why

The release packages of `v0.3.0` (built by `.github/workflows/deb.yml` on
`ubuntu-24.04`) carry `control.tar.zst` and `data.tar.zst`, because
Ubuntu's `dpkg-deb` compresses with zstd by default. Debian's lintian, the
one this project verifies with (AGENTS.md, "Verify on real artifacts"),
rejects both members with `malformed-deb-archive`, so a released package
fails the check every local build passes. The CI's own lintian step stays
green because Ubuntu's lintian accepts zstd. dpkg on the Proxmox nodes reads
the packages, so nothing failed to install; the gap is that the artifact we
publish is not the artifact we verify. Found on 2026-10-24 while checking
the `v0.3.0` assets; the maintainer chose to fix it (ask, answer A).

## What Changes

- `debian/rules` passes `-Zxz` to `dpkg-deb` through `dh_builddeb`, so
  both packages use xz whatever the build host's default is.
- The CI job asserts that each built `.deb` has `control.tar.xz` and
  `data.tar.xz` members, so a zstd package cannot be released again
  unnoticed.

## Impact

- `debian/rules` and `.github/workflows/deb.yml`. No product code, no
  change to package contents, file lists, maintainer scripts or versions.
- Spec: `debian-packaging` gains a requirement on the archive format.
- Already published releases (`v0.3.0`) stay as they are; the next release
  is the first in xz.
