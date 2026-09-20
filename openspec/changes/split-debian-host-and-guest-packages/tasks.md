# Tasks

## 1. Split the Debian binary packages

- [x] 1.1 Add a `Package: devid-enroll` stanza to `debian/control` (`Architecture: any`, `Depends: ${misc:Depends}` only, no `Recommends: swtpm`) and verify `dpkg-parsechangelog` still reports a single source version
- [x] 1.2 Rewrite the `vtpm-mds` Description so it no longer claims to ship `devid-enroll`, and add a guest-oriented Description on `devid-enroll`; verify neither `Depends`/`Recommends` line references the other package
- [x] 1.3 Change `override_dh_auto_install` so `vtpm-mds` goes to `debian/vtpm-mds/usr/sbin/` and `devid-enroll` to `debian/devid-enroll/usr/bin/`; verify `debian/rules` no longer installs the client into `debian/vtpm-mds`
- [x] 1.4 Move `devid-enroll.1` out of `debian/vtpm-mds.manpages` into `debian/devid-enroll.manpages` and verify the host manpages file lists only `vtpm-mds.8`

## 2. File-move metadata and changelog

- [x] 2.1 On `devid-enroll`, set `Breaks`/`Replaces: vtpm-mds (<< <first-split-version>)` using the version that will appear in the new changelog entry, and verify the relation is strictly less-than that version
- [x] 2.2 Add a `debian/changelog` entry describing the split (host vs guest packages) and verify `dpkg-parsechangelog -S Version` matches the Breaks/Replaces threshold

## 3. Build, CI, and cleanup

- [x] 3.1 Update `Makefile` `deb-clean` to remove `debian/devid-enroll` as well as `debian/vtpm-mds`, and verify `make deb-clean` does not leave a `debian/devid-enroll` tree
- [x] 3.2 Change `.github/workflows/deb.yml` so it fails unless both `vtpm-mds_*_amd64.deb` and `devid-enroll_*_amd64.deb` exist with the same changelog version; verify the metadata step no longer asserts exactly one `.deb`
- [x] 3.3 Lint and upload both artifacts, and attach both to the GitHub Release with notes that name `vtpm-mds` as the host package and `devid-enroll` as the guest package; verify the release notes mention both filenames

## 4. Operator docs

- [x] 4.1 Update `README.md` and `debian/README.Debian` to document two packages, host vs guest install, and that upgrading `vtpm-mds` drops `/usr/bin/devid-enroll`; verify `grep -n devid-enroll README.md debian/README.Debian` describes a separate guest `.deb`
- [x] 4.2 Update Ansible/download examples so hosts still fetch `vtpm-mds_*.deb` and guest image builds fetch `devid-enroll_*.deb`; verify no remaining "exactly one .deb" claim in README or the workflow comments

## 5. Verify package contents

- [x] 5.1 Build with `make deb` and verify `dist/` contains both `.deb` files with identical versions (`dpkg-deb -f … Version`)
- [x] 5.2 Inspect `vtpm-mds` contents with `dpkg-deb -c` and verify it includes `/usr/sbin/vtpm-mds`, aliases, the systemd unit, and `/etc/vtpm-mds`, and does not include `/usr/bin/devid-enroll`
- [x] 5.3 Inspect `devid-enroll` contents with `dpkg-deb -c` and verify it includes `/usr/bin/devid-enroll` plus its man page, and does not include `/usr/sbin/vtpm-mds`, a systemd unit, `/etc/vtpm-mds`, or `/var/lib/vtpm-mds`
- [x] 5.4 Run `dpkg-deb -I` on both packages and verify neither Depends/Recommends the other, and that `devid-enroll` has Breaks/Replaces on old `vtpm-mds`
