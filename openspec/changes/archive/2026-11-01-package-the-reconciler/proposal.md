# Proposal

## Why

`reconcile-spire-node-aliases-from-proxmox` adds a third binary,
`vtpm-mds-reconcile`, which runs once beside the SPIRE server and never on a
Proxmox node. It ships nowhere: the host package `vtpm-mds` belongs on
hypervisors and the guest package `devid-enroll` in VM images, so neither may
carry it ("Host Package Ships The Daemon Only", "Guest Package Ships The
Client Only"). Until it is packaged an operator builds it from source, writes
a unit by hand and has no versioned artifact to deploy, and its man page
`debian/vtpm-mds-reconcile.8` is installed nowhere.

The maintainer decided on 2026-10-29 (Q-2 of that change, ask `jpwfsysg…`):
a third `.deb` and a static binary, in a follow-up change. This is that
change; the go to write it came on 2026-11-01 (ask `mpj6ah7d…`).

## What Changes

- The Debian source builds a third binary package, `vtpm-mds-reconcile`, at
  the same version as the other two. It installs the binary at
  `/usr/sbin/vtpm-mds-reconcile`, a hardened systemd unit
  `vtpm-mds-reconcile.service` running as its own system user, the man page
  `vtpm-mds-reconcile(8)`, and an example configuration under
  `/usr/share/doc/vtpm-mds-reconcile/examples/`. It installs no
  configuration in `/etc` and depends on neither other package.
- Installing it starts nothing: the unit is enabled but only starts once
  `/etc/vtpm-mds-reconcile/config.yaml` exists. An upgrade restarts a running
  reconciler and leaves a stopped one stopped, like the host package.
- The release workflow builds, checks (xz, lintian) and attaches the third
  `.deb`, and attaches a statically linked
  `vtpm-mds-reconcile_<version>_linux_amd64` binary with its SHA-256, for a
  SPIRE server that is not a Debian host.
- `make deb`, `make deb-clean` and `make deb-local` handle the third package.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `debian-packaging`:
  - REMOVED "Two Binary Packages From One Source", replaced by the ADDED
    "Three Binary Packages From One Source".
  - MODIFIED "Packages Are Independently Installable", "Packages Are
    Compressed With xz On Every Build Host" and "Local Lab Builds Pass
    lintian Whatever The Branch" to cover the third package.
  - ADDED "Three Binary Packages From One Source", "Reconciler Package
    Ships The Reconciler Only", "Installing The Reconciler Package Starts
    Nothing Unconfigured" and "Release Attaches A Static Reconciler Binary".

## Impact

- `debian/control`, `debian/rules`, new `debian/vtpm-mds-reconcile.{service,
  sysusers,manpages,examples,postinst,lintian-overrides,README.Debian}` and
  an example config;
  `.github/workflows/deb.yml`; `Makefile` (`deb`, `deb-clean`);
  `scripts/deb-local.sh`; `README.md` (status row "not yet packaged");
  on archiving, the Purpose paragraph of
  `openspec/specs/debian-packaging/spec.md` by hand (archiving does not
  rewrite it).
- **Prerequisite:** `reconcile-spire-node-aliases-from-proxmox` is merged, so
  `cmd/vtpm-mds-reconcile` and `debian/vtpm-mds-reconcile.8` exist on `main`.
  This record is written against `main` before that; the code waits.
- **Not** in this change: any behaviour of the reconciler itself (flags,
  configuration keys, defaults, logs), the operator's deployment (which host,
  credentials, the SPIRE server's socket group), a container image, other
  architectures than amd64, an apt repository.
