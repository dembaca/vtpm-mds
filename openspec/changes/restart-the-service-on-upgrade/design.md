# Design

## Context

See proposal.md — Why.

`debian/rules`:

```
override_dh_installsystemd:
	dh_installsystemd --no-start
```

What debhelper 13.24.2 generates from that, read from
`/var/lib/dpkg/info/vtpm-mds.*` on `hogan`:

| Script | systemd action |
|---|---|
| `preinst` | `deb-systemd-invoke stop` when `$1 = upgrade` |
| `postinst` | `deb-systemd-helper unmask/enable/update-state`; no start, no restart; then the hand-written `daemon-reload` |
| `prerm` | hand-written `stop` on `remove`/`deconfigure` only |
| `postrm` | `daemon-reload`, `deb-systemd-helper purge` |

The `--no-start` comment says why it is there: binding `169.254.169.1:80`
fails on build hosts and lab hosts without `vmbr_imds`. That reason is about
**first installs** and stays valid. It never applied to an upgrade of a
daemon that is already running and bound — that host demonstrably has the
bridge.

## Goals / Non-Goals

**Goals:**

- Upgrading a node whose daemon runs ends with the new version running,
  without a manual step, and with a gap of one restart.
- Upgrading a node whose daemon is stopped leaves it stopped.
- A first install starts nothing.
- The spec says what the package does, including what debhelper generates.

**Non-Goals:**

- Zero-downtime upgrades (socket activation, `SO_REUSEPORT` hand-over). One
  restart is a sub-second gap; guests retry metadata reads.
- Restarting on `pve-cluster` or configuration changes; that is
  `reload-when-guest-configs-change`.
- Changing the unit file, the sysusers entry, or the credential handling.
- The guest package `devid-enroll`, which ships no service.

## Decisions

- **D-a. `try-restart` from `postinst`, not debhelper's restart snippet.**
  Dropping `--no-start` and relying on debhelper's default
  `--restart-after-upgrade` would generate
  `if [ -n "$2" ]; then restart; else start; fi`: a first install would
  *start* (the case `--no-start` exists for), and an upgrade would *start* a
  daemon the operator had stopped. `try-restart` restarts only an active unit
  and is a no-op otherwise, which is exactly the policy. Alternative
  considered: keep debhelper's snippet with `--no-start` removed and add
  `--no-enable`-style guards by hand — more generated code to reason about,
  for the same result.
- **D-b. `--no-stop-on-upgrade`.** Without it the generated `preinst` stops
  the daemon before unpacking, so `try-restart` in `postinst` would find it
  inactive and do nothing. With it, the old process keeps serving from its
  already-loaded binary during the unpack, and the restart swaps it for the
  new one. The daemon reads its binary, unit and configuration only at start,
  so an unpack under a running process cannot confuse it.
- **D-c. Order inside `postinst`: `#DEBHELPER#`, then `daemon-reload`, then
  `try-restart`.** The restart must use the unit just installed (the
  `PartOf=`, `LoadCredential=` and sandbox directives change between
  versions), so it follows the reload added by
  `make-host-package-install-deterministic`.
- **D-d. A failed restart warns, it does not fail `dpkg`.** If the new
  version cannot start, the old process is already gone either way; failing
  the `configure` step would leave the package half-configured on top of a
  down service and block unrelated `apt` runs. The script prints one line to
  stderr (`vtpm-mds: restart after upgrade failed; see journalctl -u
  vtpm-mds`) and exits 0. Alternative, for the maintainer to choose instead:
  fail the configure step so Ansible stops the rollout at the first node.
- **D-e. Only where a manager runs, and through `deb-systemd-invoke`.** Same
  `[ -d /run/systemd/system ]` guard as the reload; `deb-systemd-invoke`
  honours `policy-rc.d`, so chroots and image builds that deny service actions
  keep doing so.

## Risks / Trade-offs

- An upgrade of a production node now restarts its IMDS. That is the point,
  but rollouts that previously relied on a later manual start to stagger
  restarts now restart at install time. Mitigation: they could already not
  rely on it, since the old package stopped the daemon anyway.
- A new version that fails to start is now discovered at upgrade time, by the
  warning, rather than at the next manual start. That is an improvement, but
  D-d means `dpkg` still exits 0.
- `--no-stop-on-upgrade` is the debhelper name; its deprecated alias
  `--no-restart-on-upgrade` disappears in compat 14 and is not used.

## Migration

None. The first upgrade **to** this version still runs the new `preinst`,
which no longer stops anything, and the new `postinst`, which restarts — so
the fix applies from the very upgrade that installs it.

## Open Questions

- D-d (warn or fail) is the maintainer's call; the record proceeds with
  "warn" unless overruled.
