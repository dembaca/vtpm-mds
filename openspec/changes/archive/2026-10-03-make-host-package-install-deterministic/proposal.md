# Proposal

## Why

After `dpkg -i`, two things are supposed to be certain: which tree you
installed, and that the host is running it. Neither currently holds.

**The documented install command is ambiguous.** `README.md` says
`sudo dpkg -i dist/vtpm-mds_*.deb`. As soon as `dist/` holds more than one
build — which `make deb` and `make deb-local` make routine, since they write
different versions side by side — that glob expands to several files and dpkg
is handed all of them. Worse, glibc collation ignores punctuation at the
primary level, so `vtpm-mds_0.2.0_amd64.deb` sorts *before*
`vtpm-mds_0.2.0+git20260920.c2ad576144f2_amd64.deb`: the stale release package
comes first. The same shape in a verification step is actively misleading —
`dpkg-deb -f dist/vtpm-mds_*.deb Version` reads only the first archive and
treats the second filename as a field name, so it reported `Version: 0.2.0`
for a freshly built git-stamped package. Observed on `hogan` on 2026-10-02.

**The installed unit is not the unit systemd runs.** `debian/rules` calls
`dh_installsystemd --no-start`, deliberately, because binding
`169.254.169.1:80` fails on hosts without the IMDS bridge. But debhelper puts
`systemctl daemon-reload` in its *start/restart* snippet, which `--no-start`
suppresses. The generated `postinst` therefore only unmasks and enables the
unit. Until someone runs `daemon-reload` by hand, systemd keeps serving the
previous unit definition and says so:

    Warning: The unit file, source configuration file or drop-ins of
    vtpm-mds.service changed on disk. Run 'systemctl daemon-reload'.

That is how a verification run on 2026-10-02 came to measure a stale unit, and
it would silently defeat any future change that touches `vtpm-mds.service` —
including the `StartLimit*` fix this project already shipped once.

Together these undermine the rule in `AGENTS.md` that `dpkg -l vtpm-mds` and
`vtpm-mds -version` identify exactly which tree is running.

## What Changes

- The host package's `postinst` reloads the systemd manager after the unit is
  installed, so the unit on disk is the unit systemd knows. The package still
  does not start or restart the daemon — that non-goal is unchanged.
- `README.md` names the package explicitly instead of globbing, and shows how
  to pick the freshly built one. The same correction applies to the guest
  package and to the remote install line.
- No change to what either package contains, to the unit's directives, to the
  `--no-start` policy, or to any daemon behaviour.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `debian-packaging`: gains a requirement covering what installing the host
  package leaves behind in systemd. The capability today says the unit is
  installed but nothing about the manager being told, which is why a package
  that leaves a stale unit loaded contradicts no requirement.

## Impact

- `debian/rules`: an `override_dh_installsystemd` that keeps `--no-start` and
  adds the reload, or an explicit snippet in `debian/postinst`.
- `debian/postinst`: the generated maintainer script gains the reload.
- `README.md`: three install lines.
- Operators and any automation that installs the host package. After this
  change a `dpkg -i` that replaces the unit takes effect at the next start
  without a manual step.
- Verification runs for every other change in this repository: a result
  measured against a stale unit is not worth recording, and this removes the
  trap that produces one.
