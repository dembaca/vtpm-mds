# Proposal

## Why

Upgrading the host package takes the IMDS down and leaves it down.

On `hogan` on 2026-10-19, `hogan-lab dpkg-install --force-confold` of
`0.2.0+git20261018.02dadca5c027` over `…38bd29d4400c` stopped `vtpm-mds` at
10:34:19 (`Stopping vtpm-mds.service`, `Server stopped gracefully`) and nothing
started it again. Every guest on the node had no metadata and no DevID
enrollment until a manual `systemctl start` at 10:35:02 — 43 seconds, and only
because someone was watching. On a production node an unattended upgrade
(Ansible, `unattended-upgrades`) would leave it down until the next reboot or
the next person to notice.

The stop does not come from `debian/prerm`, which stops only on `remove` and
`deconfigure`. It comes from a `preinst` that `dh_installsystemd` generates:

```
# Automatically added by dh_installsystemd/13.24.2
if [ -z "$DPKG_ROOT" ] && [ "$1" = upgrade ] && [ -d /run/systemd/system ] ; then
	deb-systemd-invoke stop 'vtpm-mds.service' >/dev/null || true
fi
```

`debian/rules` passes `--no-start`, which suppresses the start/restart half of
debhelper's upgrade handling but not the stop half. The result is the one
combination nobody chose: stopped on every upgrade, never started.

It also contradicts the living spec. `debian-packaging`, "Installing The Host
Package Leaves systemd In Step", says the package "SHALL NOT start, restart or
stop the daemon on installation". The package does stop it; no test or
validator looks at the generated `preinst`, so the contradiction went unseen.

## What Changes

- An upgrade no longer stops the daemon before unpacking. If it was running,
  it is restarted once, after the new binary and unit are in place and the
  manager has been reloaded, so the IMDS gap is one restart rather than an
  open-ended outage.
- A daemon that was **not** running before the upgrade stays stopped: the
  package restarts, it never starts. A first installation still starts
  nothing, so build hosts and lab hosts without the IMDS bridge are unaffected.
- A restart that systemd refuses does not fail the installation, but it is
  reported on stderr. A new version that starts and then exits (a failed
  bind, say) is not seen by the installation: the unit is `Type=simple`, so
  the restart succeeds once the process is forked. That failure shows in the
  journal, where `Restart=on-failure` retries it, as it does today.
- No change to the unit's directives, to `--no-start` for first installs, to
  removal, or to any daemon behaviour.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `debian-packaging`: "Installing The Host Package Leaves systemd In Step"
  replaces "SHALL NOT start, restart or stop" with the upgrade behaviour above,
  and gains scenarios for a running and a stopped daemon at upgrade time.

## Impact

- `debian/rules`: `override_dh_installsystemd` gains `--no-stop-on-upgrade`,
  so debhelper no longer generates the stopping `preinst`.
- `debian/postinst`: on `configure` with a previous version, after the
  existing `daemon-reload`, `deb-systemd-invoke try-restart vtpm-mds.service`,
  guarded on a running manager.
- `README.md` and `debian/README.Debian`, where they tell the operator to
  start the service after `dpkg -i`: only the coordinator edits the root
  `README.md`; this change names the line it needs changed.
- Operators: an upgrade of a running node now restarts it. Anyone relying on
  "the package never touches the running daemon" — which was not true — loses
  nothing they had.
- Verification of every later change on `hogan`: the post-install
  `systemctl start` becomes unnecessary.
