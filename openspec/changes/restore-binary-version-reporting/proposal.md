# Proposal

## Why

`README.md` and `scripts/deb-local.sh` tell operators to identify a running
build with `vtpm-mds -version` and `devid-enroll -version`, and the `.deb`
installed on the lab host answers both. No commit ever contained those flags:
they were uncommitted work, dropped when the working tree was restored on
2026-09-20. On `main`, both binaries reject `-version` with exit 2, so the
documented way to check which tree is deployed does not work, and the guest
`devid-enroll` package introduced by `split-debian-host-and-guest-packages`
would ship a client that cannot report its own version.

The same lost hunk had `StartLimitIntervalSec` / `StartLimitBurst` in the unit
`[Unit]` section. On `main` they sit in `[Service]`, where systemd ignores
them (`systemd-analyze verify`: *Unknown key 'StartLimitIntervalSec' in
section [Service], ignoring*), so `Restart=on-failure` has no rate limit and a
daemon that cannot bind its listen address restarts forever.

## What Changes

- Both binaries accept `-version`, print `<name> <version>` and exit 0. The
  version is the string injected at build time, so a packaged binary reports
  the Debian package version.
- `debian/vtpm-mds.8` and `debian/devid-enroll.1` document the flag.
- The unit's start-limit directives move to `[Unit]` so systemd applies them.
- No change to the enroll protocol, the metadata endpoints, config defaults,
  or any other flag.

## Capabilities

### New Capabilities

- `operability`: how an operator confirms which build is running and how the
  host service is supervised.

### Modified Capabilities

None.

## Impact

- `main.go`, `cmd/devid-enroll/main.go`: one flag each, checked before any TPM
  or config work so `-version` never touches hardware or files.
- `debian/vtpm-mds.service`: two directives move section.
- `debian/vtpm-mds.8`, `debian/devid-enroll.1`: one option entry each.
- Implemented together with `split-debian-host-and-guest-packages` in
  https://github.com/dembaca/vtpm-mds/pull/10, because the guest package's
  documented verification step (`devid-enroll -version` after `dpkg -i`)
  cannot pass without it. Recording it as its own change keeps the packaging
  change's "CLI flags unchanged" non-goal honest.
