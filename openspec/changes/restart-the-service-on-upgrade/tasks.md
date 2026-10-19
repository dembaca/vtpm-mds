# Tasks

Written on 2026-10-19 against `main` at `364bffe` by
`agent-vtpm-hogan-d3e6cbc82`, at the coordinator's assignment after the
maintainer asked to "tackle the operations and packaging". **Code only after
the coordinator's ACK of this record.** Files: `debian/rules`,
`debian/postinst`, `debian/README.Debian`; the line in the root `README.md` is
handed to the coordinator.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-e in `design.md` (D-d, warn or fail on a failed restart, is the one most likely to move), and verify the outcome is recorded there
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
- [ ] 4.2 Upgrade a **stopped** daemon (needs the maintainer's go: the node's IMDS is down for the test), and verify it is still `inactive` afterwards; then start it and verify VM 399 reads `instance-id` `200`
- [x] 4.3 Verify the first-install case on a host without the IMDS bridge (cloud lab, or a chroot where `policy-rc.d` denies), and say which was used
      **Chroot**, by `agent-cursor-vtpm-22e0` on the cloud lab, 2026-10-19
      11:11: debootstrap minbase noble, `policy-rc.d` exit 101, no prior
      package or unit; `0.2.0+git20261019.88c2405a4e51` from `88c2405`.
      `dpkg -i` exit 0, `is-enabled` enabled, no daemon process, no start in
      the install log. **What it covers:** the install succeeds and enables
      without starting where no manager runs and `policy-rc.d` denies. It does
      **not** execute the `[ -n "$2" ]` first-install branch against a running
      manager (no running manager in the chroot, and `policy-rc.d` would have
      made a `try-restart` a no-op anyway); that branch is covered by reading
      the extracted `postinst` (3.2) only.
- [ ] 4.4 Say what was not verified, in particular the failing-restart scenario if no version that fails to start was installed
      So far: the failing-restart scenario (would need a version that cannot
      start on the production node); a first install in a guest with a running
      manager and no bridge (4.3 was a chroot); 4.2 pending the maintainer's
      go.

## 5. Archive

- [ ] 5.1 After merge, the coordinator archives with `openspec archive restart-the-service-on-upgrade` and verifies `openspec validate --all --strict` stays clean
