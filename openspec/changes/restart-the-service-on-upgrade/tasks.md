# Tasks

Written on 2026-10-19 against `main` at `364bffe` by
`agent-vtpm-hogan-d3e6cbc82`, at the coordinator's assignment after the
maintainer asked to "tackle the operations and packaging". **Code only after
the coordinator's ACK of this record.** Files: `debian/rules`,
`debian/postinst`, `debian/README.Debian`; the line in the root `README.md` is
handed to the coordinator.

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — accept or overrule D-a to D-e in `design.md` (D-d, warn or fail on a failed restart, is the one most likely to move), and verify the outcome is recorded there
- [ ] 1.2 **Coordinator** — ACK the record and the delta on the bus before any code is written, and verify the ACK names this change

## 2. Reproduce

- [x] 2.1 Read the installed `preinst` and verify it stops the service on `upgrade` while `postinst` starts nothing
      Done on `hogan` 2026-10-19, `/var/lib/dpkg/info/vtpm-mds.preinst`
      (debhelper 13.24.2): `deb-systemd-invoke stop 'vtpm-mds.service'` under
      `[ "$1" = upgrade ]`; `postinst` has only `deb-systemd-helper` calls and
      the hand-written `daemon-reload`. Journal of the 10:34:19 upgrade:
      `Stopping vtpm-mds.service`, nothing started until a manual
      `systemctl start` at 10:35:02.

## 3. Implement

- [ ] 3.1 Add `--no-stop-on-upgrade` to `override_dh_installsystemd` in `debian/rules`, keeping `--no-start`, and update its comment; verify with `dpkg-deb -e` on the rebuilt package that `preinst` contains no `stop`
- [ ] 3.2 Add the guarded `deb-systemd-invoke try-restart vtpm-mds.service` to `debian/postinst` after the `daemon-reload`, on `configure` with a non-empty previous version, warning on stderr and exiting 0 on failure (D-c, D-d, D-e); verify by reading the extracted `postinst` that the order is `#DEBHELPER#` → reload → try-restart
- [ ] 3.3 Update `debian/README.Debian` where it tells the operator to start the service after an upgrade, and name the matching root `README.md` line to the coordinator; verify with `grep -n 'systemctl start' debian/README.Debian README.md`
- [ ] 3.4 Run `lintian --fail-on error,warning` on the built `.deb`s and `openspec validate --all --strict`, and verify both pass

## 4. Verify on hogan

- [ ] 4.1 Upgrade a **running** daemon: poll `http://169.254.169.1/health` from the host every 50 ms during `hogan-lab dpkg-install --force-confold`, and verify `systemctl is-active` is `active` afterwards with no manual start, the journal shows the new version, the longest gap in seconds is recorded, and VM 399 reads `instance-id` `200`
- [ ] 4.2 Upgrade a **stopped** daemon (needs the maintainer's go: the node's IMDS is down for the test), and verify it is still `inactive` afterwards; then start it and verify VM 399 reads `instance-id` `200`
- [ ] 4.3 Verify the first-install case on a host without the IMDS bridge (cloud lab, or a chroot where `policy-rc.d` denies), and say which was used
- [ ] 4.4 Say what was not verified, in particular the failing-restart scenario if no version that fails to start was installed

## 5. Archive

- [ ] 5.1 After merge, the coordinator archives with `openspec archive restart-the-service-on-upgrade` and verifies `openspec validate --all --strict` stays clean
