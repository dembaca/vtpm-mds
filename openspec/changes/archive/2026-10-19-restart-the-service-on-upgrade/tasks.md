# Tasks

Written on 2026-10-19 against `main` at `364bffe` by
`agent-vtpm-hogan-d3e6cbc82`, at the coordinator's assignment after the
maintainer asked to "tackle the operations and packaging". **Code only after
the coordinator's ACK of this record.** Files: `debian/rules`,
`debian/postinst`, `debian/README.Debian`; the line in the root `README.md` is
handed to the coordinator.

## 1. Decide

- [x] 1.1 **Maintainer, optional** — accept or overrule D-a to D-f in `design.md` (D-d, warn or fail on a failed restart, is the one most likely to move), and verify the outcome is recorded there
      **Done, 2026-10-19 11:49:** dembaca accepted D-d "warn" (E3, 👍 on the
      bus); the rest stands as recorded. `design.md` "Open Questions" says so.
- [x] 1.2 **Coordinator** — ACK the record and the delta on the bus before any code is written, and verify the ACK names this change
      **Done, 2026-10-19 10:50:** `agent-mac-vtpm-mds-36505b`, "ACK on the
      record `restart-the-service-on-upgrade` @ `e756a97` … Code: go", draft
      PR #60; D-d stays "warn".

## 2. Reproduce

- [x] 2.1 Read the installed `preinst` and verify it stops the service on `upgrade` while `postinst` starts nothing
      Done on `hogan` 2026-10-19, `/var/lib/dpkg/info/vtpm-mds.preinst`
      (debhelper 13.24.2): `deb-systemd-invoke stop 'vtpm-mds.service'` under
      `[ "$1" = upgrade ]`; `postinst` has only `deb-systemd-helper` calls and
      the hand-written `daemon-reload`. Journal of the 10:34:19 upgrade:
      `Stopping vtpm-mds.service`, nothing started until a manual
      `systemctl start` at 10:35:02.

## 3. Implement

- [x] 3.1 Add `--no-stop-on-upgrade` to `override_dh_installsystemd` in `debian/rules`, keeping `--no-start`, and update its comment; verify with `dpkg-deb -e` on the rebuilt package that `preinst` contains no `stop`
      `e18ac57`, built as `0.2.0+git20261019.e18ac5792907` (clean tree):
      `dpkg-deb -e` yields `conffiles control md5sums postinst postrm prerm` —
      no `preinst` at all, so nothing stops the daemon before unpacking.
- [x] 3.2 Add the guarded `deb-systemd-invoke try-restart vtpm-mds.service` to `debian/postinst` after the `daemon-reload`, on `configure` with a non-empty previous version, warning on stderr and exiting 0 on failure (D-c, D-d, D-e); verify by reading the extracted `postinst` that the order is `#DEBHELPER#` → reload → try-restart
      Extracted `postinst`: debhelper section lines 16–32, `daemon-reload`
      line 47, `deb-systemd-invoke try-restart` line 57 under
      `[ "$1" = "configure" ] && [ -n "$2" ] && [ -d /run/systemd/system ]`,
      warning on stderr on failure. `deb-systemd-invoke` (init-system-helpers
      on hogan) consults `policy-rc.d` for every action and `exec`s
      `systemctl try-restart` for a system unit, so its exit status is
      systemctl's; no direct `systemctl` fallback was needed.
- [x] 3.3 Update `debian/README.Debian` where it tells the operator to start the service after an upgrade, and name the matching root `README.md` line to the coordinator; verify with `grep -n 'systemctl start' debian/README.Debian README.md`
      `debian/README.Debian` now says an upgrade restarts a running daemon;
      its two remaining `systemctl start` lines (32, 82) are first-install
      instructions and stay. `README.md:66` ("enabled on install, not
      auto-started") goes to the coordinator, who owns the root README.
- [x] 3.4 Run `lintian --fail-on error,warning` on the built `.deb`s and `openspec validate --all --strict`, and verify both pass
      `lintian --fail-on error,warning` on both `.deb`s: exit 0, no output.
      `openspec validate --all --strict`: 6 passed.

## 4. Verify on hogan

- [x] 4.1 Upgrade a **running** daemon: poll `http://169.254.169.1/health` from the host every 50 ms during `hogan-lab dpkg-install --force-confold`, and verify `systemctl is-active` is `active` afterwards with no manual start, the journal shows the new version, the longest gap in seconds is recorded, and VM 399 reads `instance-id` `200`
      Done on hogan 2026-10-19 with the maintainer's go (10:56):
      `02dadca5c027` → `e18ac5792907`, no manual start. `is-active`
      `active`, `vtpm-mds -version` the new version. Journal: `Stopped`
      10:57:55.114, `Started` .127, `Starting server on 169.254.169.1:80`
      .365, `DevID enrollment routes registered`. Longest `/health` gap
      **0.257 s** (245 polls at 50 ms, 5 failed); the same kind of upgrade
      before this change left the IMDS down 43 s until a manual start. VM 399
      afterwards: token `200`, `instance-id` `200` `i-399`.
- [x] 4.2 Upgrade a **stopped** daemon (needs the maintainer's go: the node's IMDS is down for the test), and verify it is still `inactive` afterwards; then start it and verify VM 399 reads `instance-id` `200`
      Done on hogan 2026-10-19 with the maintainer's go (11:38, in the
      thread of the 10:57 request that named this case). `systemctl stop`
      at 11:42:06; `hogan-lab dpkg-install --force-confold` of
      `0.2.0+git20261019.27d0db6e0a7c` (`27d0db6`, the final code) over
      `…e18ac5792907`: `dpkg` exit 0, no stderr from `postinst`, three
      seconds later `is-active` `inactive` and no journal line after the
      stop. Then `systemctl start`: `Started` 11:42:19.99, `vtpm-mds version
      0.2.0+git20261019.27d0db6e0a7c`, `DevID enrollment routes
      registered`; VM 399 token `200`, `instance-id` `200` `i-399`. The IMDS
      was down 13.5 s, all of it the deliberate stop. hogan now runs
      `27d0db6`.
- [x] 4.3 Verify the first-install case on a host without the IMDS bridge (cloud lab, or a chroot where `policy-rc.d` denies), and say which was used
      **Lab guest** `guest100`, by `agent-cursor-vtpm-22e0` on the cloud lab,
      2026-10-19 11:12: systemd running (`/run/systemd/system` present), no
      `policy-rc.d`, no IMDS listen address, unit never installed;
      `0.2.0+git20261019.88c2405a4e51` from `88c2405`. `dpkg -i` exit 0,
      `is-enabled` enabled, `is-active` inactive, stderr without
      `restart after upgrade failed`. So the `[ -n "$2" ]` branch was
      executed against a running manager and started nothing. (An earlier
      chroot run with `policy-rc.d` 101 is superseded by this one.)
- [x] 4.4 Say what was not verified, in particular the failing-restart scenario if no version that fails to start was installed
      Not verified: the refused-restart scenario (would need a masked or
      broken unit on the production node), and the `DPKG_ROOT` guard (no
      installation into another root was made). 4.1 ran on `e18ac57`, whose
      `postinst` differs from the final `27d0db6` only by the `DPKG_ROOT`
      guards, the warning text and comments; 4.2 ran on `27d0db6`. By
      design, not a gap: a new version that execs and then exits (failed
      bind, bad configuration) is not reported by `dpkg` at all, because
      `Type=simple` makes `try-restart` succeed at fork; it shows in the
      journal and is retried by `Restart=on-failure` (D-d, Fable pre-review
      2026-10-19).

## 5. Pre-review fixes (Fable, 2026-10-19)

- [x] 5.0 Correct D-d, the delta and 4.4 so a failed restart means a restart systemd refuses, not a daemon that exits after fork; add D-f (`dpkg-reconfigure` restarts) and the `DPKG_ROOT` guard (D-e); verify with `sh -n debian/postinst`, `openspec validate --all --strict` and a rebuilt package's extracted `postinst`

## 6. Archive

- [x] 6.1 After merge, the coordinator archives with `openspec archive restart-the-service-on-upgrade` and verifies `openspec validate --all --strict` stays clean
      Done 2026-10-19 from `main` at `e034138` (#60 merged); see the archive
      commit for the delta counts and validation.
