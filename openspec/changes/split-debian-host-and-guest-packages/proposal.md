# Proposal

## Why

Debian currently builds a single `vtpm-mds` binary package that ships both the
hypervisor daemon and the guest `devid-enroll` client. The package is
host-oriented (systemd unit, IMDS listen address, Proxmox inventory, swtpm
recommendation), so installing it on guests would also enable a daemon they
must not run. Guests therefore get the client by copying a binary (cloud-init
/ shared dir) rather than by a first-class package, which does not scale to
real VM images.

## What Changes

- Produce **two binary packages** from the existing `vtpm-mds` source:
  - `vtpm-mds` — host daemon, aliases, systemd unit, host config and state dirs
  - `devid-enroll` — guest client binary and its man page only
- **BREAKING**: upgrading `vtpm-mds` no longer leaves `/usr/bin/devid-enroll`
  on the host. Operators who need the client install `devid-enroll` separately
  (guest images, not hypervisors).
- Release, CI, `make deb`, and operator docs advertise both `.deb` artifacts.
- The enroll HTTP protocol, guest CLI flags, and daemon behaviour are unchanged.

## Capabilities

### New Capabilities

- `debian-packaging`: how the source is split into installable Debian packages
  for the hypervisor versus guest clients, including what each package may and
  must not contain.

### Modified Capabilities

None. `devid-enrollment` describes the enroll wire protocol, not how the guest
client is delivered.

## Impact

- `debian/control` grows a second `Package:` stanza; `debian/rules` installs
  each binary into its own package directory.
- Maintainer scripts, systemd unit, dirs and conffile stay on `vtpm-mds` only.
- `debian/devid-enroll.1` moves with the client package.
- `Makefile` (`deb`, `deb-clean`), `.github/workflows/deb.yml`, README,
  `debian/README.Debian`, and GitHub Release notes currently assume exactly
  one `vtpm-mds_*_amd64.deb`.
- QEMU lab cloud-init copy of `devid-enroll` can keep working; switching the
  lab to the guest package is optional, not required for this change.
- Ansible / operator fetch patterns that download only `vtpm-mds_*.deb` stay
  valid for hosts; guest image builds need a second download.
