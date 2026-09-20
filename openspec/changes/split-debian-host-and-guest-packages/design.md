# Design

## Context

See proposal.md for why the split is needed. Today `debian/control` declares a
single `Package: vtpm-mds`. `debian/rules` installs both Go binaries into
`debian/vtpm-mds/` (`/usr/sbin/vtpm-mds` and `/usr/bin/devid-enroll`), and
`debian/vtpm-mds.manpages` ships both man pages. CI (`deb.yml`) and `make deb`
assert exactly one `vtpm-mds_*_amd64.deb`.

This is a packaging split of an already-built pair of binaries. No Go API,
enroll wire protocol, or systemd unit behaviour changes.

## Goals / Non-Goals

**Goals:**

- One source package, two binary packages, same changelog version.
- Guests can `dpkg -i devid-enroll_*.deb` without enabling `vtpm-mds.service`.
- File-move from the pre-split `vtpm-mds` is a clean dpkg upgrade.

**Non-Goals:**

- Changing enroll protocol, CLI flags, or daemon listen/config defaults.
- Publishing an apt repository; GitHub Release remains the distribution path.
- Switching the QEMU lab from cloud-init binary copy to the guest `.deb`.
- Splitting into two source packages or adding a third `-doc` package.
- Making `vtpm-mds` Recommend or Depend on `devid-enroll`.

## Decisions

### One source, two binary packages

Keep a single `Source: vtpm-mds` and add a second `Package: devid-enroll`
stanza in `debian/control`. Both stay `Architecture: any` (static Go binaries).

Alternative considered: two source packages. Rejected — they share one Go
module, one changelog, and one CI job.

Alternative considered: keep one package and tell operators it is "light
enough" for guests. Rejected — install would enable a host systemd unit that
binds `169.254.169.1:80` (`After=pve-cluster.service`, `Recommends: swtpm`).

### No inter-package Depends

`vtpm-mds` does not Depend/Recommend `devid-enroll` (hosts do not run the
client). `devid-enroll` does not Depend on `vtpm-mds` (the daemon lives on the
hypervisor; the guest talks to IMDS over the network).

`devid-enroll` Depends stay `${misc:Depends}` only (`CGO_ENABLED=0`). Do not
copy `Recommends: swtpm` onto the guest package; the guest uses `/dev/tpm0`
from the vTPM device, not the host `swtpm` package.

### File move via Breaks/Replaces

`/usr/bin/devid-enroll` moves off `vtpm-mds`. On `devid-enroll`:

```
Breaks: vtpm-mds (<< <first-split-version>)
Replaces: vtpm-mds (<< <first-split-version>)
```

That is the Debian file-move pattern so unpack order is defined and the new
package can take the path. Upgrading `vtpm-mds` alone is supposed to drop the
client (hosts should not keep it).

### Install layout in debian/rules

Keep the existing `override_dh_auto_build` (both binaries). Change
`override_dh_auto_install` to:

- `debian/vtpm-mds/usr/sbin/vtpm-mds`
- `debian/devid-enroll/usr/bin/devid-enroll`

Split manpages: `debian/vtpm-mds.manpages` keeps `vtpm-mds.8`; add
`debian/devid-enroll.manpages` for `devid-enroll.1`. Maintainer scripts,
`vtpm-mds.service`, `vtpm-mds.dirs`, `vtpm-mds.install` (config.yaml), and
`dh_installsystemd --no-start` stay host-only. `debian/docs` can stay on the
source/host package; the guest package does not need ARCHITECTURE.md.

Update the `vtpm-mds` Description so it no longer claims to ship
`devid-enroll`.

### CI and `make deb`

`make deb` already copies `$(DEB_STAGE)/*.deb`. Stop asserting a single file.
`deb.yml` must lint, upload, and attach both artifacts; release notes must
name host vs guest. `deb-clean` must remove `debian/devid-enroll` as well as
`debian/vtpm-mds`.

## Risks / Trade-offs

- **[Risk] Operators who installed the combined package on a host lose
  `/usr/bin/devid-enroll` on upgrade** → Mitigation: changelog, README, and
  `debian/README.Debian` state that the client is a separate guest package.
  Hosts should not have been using it.
- **[Risk] CI/Ansible still downloads only `vtpm-mds_*.deb`** → Mitigation:
  keep that pattern valid for hypervisors; document `devid-enroll_*.deb` for
  image builds. Fail CI if either artifact is missing.
- **[Risk] Unpack conflict if Breaks/Replaces version is wrong** → Mitigation:
  set the version to the first split changelog version (`<<` that version),
  and verify with `dpkg -i` of old then new packages in the test plan.

## Migration Plan

1. Land the split in `debian/` and bump `debian/changelog`.
2. Hosts: `dpkg -i vtpm-mds_<new>_<arch>.deb` (or apt upgrade). Client binary
   disappears; daemon/unit unchanged.
3. Guest images: install `devid-enroll_<new>_<arch>.deb` instead of copying
   the binary. Lab may keep the copy path.
4. Rollback: install the previous combined `vtpm-mds` `.deb`. The split
   `devid-enroll` package should be removed first if both were installed, to
   avoid the reverse file-move conflict.

## Open Questions

None. Lab-vs-package delivery for guests is deferred on purpose (non-goal).
