# Tasks

## 1. Reproduce both defects

- [x] 1.1 With two host packages in `dist/`, verify `dpkg-deb -f dist/vtpm-mds_*.deb Version` reports the stale version and `printf '%s\n' dist/vtpm-mds_*.deb` shows the release package sorting first — record both outputs as the baseline
      Reproduced on `hogan` 2026-10-03 with both builds in `dist/`:
      `printf '%s\n' dist/vtpm-mds_*.deb` lists `vtpm-mds_0.2.0_amd64.deb`
      first and the git-stamped build second, and
      `dpkg-deb -f dist/vtpm-mds_*.deb Version` reports `Version: 0.2.0` — the
      stale one, with a field-name prefix because the second filename was
      taken as a field.
- [x] 1.2 Extract the built package with `dpkg-deb -e` and verify its `postinst` contains no `systemctl daemon-reload`, only the `deb-systemd-helper` enable calls
      `dpkg-deb -e` on the installed build: `grep -c daemon-reload postinst`
      returns 0. The script contains only `deb-systemd-helper unmask`,
      `was-enabled`, `enable` and `update-state` — no `systemctl` call.

## 2. Reload the manager on install

- [x] 2.1 Extend `override_dh_installsystemd` in `debian/rules` so the generated `postinst` reloads the systemd manager, keeping `--no-start`, and verify with `dpkg-deb -e` that the rebuilt package's `postinst` now contains the reload
      Run by Andreas Dembach as root on `hogan`, 2026-10-03. `make deb-local`
      produced `0.2.0+git20261003.9fad3704a940`; `dpkg-deb -e` on it and
      `grep -n daemon-reload postinst`:
        33:# debhelper's own `systemctl daemon-reload`, which it emits as part of the
        37:#   Warning: The unit file ... changed on disk. Run 'systemctl daemon-reload'.
        42:    systemctl --system daemon-reload >/dev/null 2>&1 || true
      Line 42 is the command; the `[ -d /run/systemd/system ]` guard is the
      line above it. The build log also shows `override_dh_installsystemd`
      still passing `--no-start`.
- [x] 2.2 Verify the reload is guarded on a running manager and cannot fail the install, by checking the snippet tests `[ -d /run/systemd/system ]` and ends in `|| true`
      The snippet is `if [ "$1" = "configure" ] && [ -d /run/systemd/system ];`
      and the command ends in `|| true` with output discarded, so a host with
      no running manager, or a manager that refuses, cannot fail the install.
- [x] 2.3 Verify the package still does not start the daemon: install on a host where the listen address cannot be bound and verify the install succeeds with the unit enabled and nothing running
      Run by Andreas Dembach as root on `hogan`, 2026-10-03. Service stopped,
      then `dpkg -i --force-confold`:
        systemctl is-active  -> inactive
        systemctl is-enabled -> enabled
      Installed and enabled, nothing started. The substitute for "a host that
      cannot bind" was a stopped service on a host that can; what it shows is
      the same thing — the maintainer scripts start nothing.
- [x] 2.4 Verify the warning is gone end to end: change a comment in `debian/vtpm-mds.service`, rebuild, `dpkg -i --force-confold`, and verify `systemctl status vtpm-mds` no longer reports that the unit file changed on disk — needs root on a lab host, record who ran it
      Run by Andreas Dembach as root on `hogan`, 2026-10-03. A comment line
      was appended to `debian/vtpm-mds.service` so the unit genuinely differed,
      the package rebuilt (version then carried `.dirty`, which is the stamp
      doing its job) and installed over the previous one:
        Unpacking vtpm-mds (...dirty) over (0.2.0+git20261003.9fad3704a940)
        Setting up vtpm-mds (...dirty)
        keine Warnung
      `systemctl status` reported no "changed on disk" warning. Before this
      change the same sequence produced one, which is what made an earlier
      verification measure a stale in-memory unit.
      Afterwards the test line was reverted and the clean
      `0.2.0+git20261003.9fad3704a940` reinstalled, so the installed unit again
      matches a committed tree: `vtpm-mds -version` reports no `.dirty`, the
      installed unit carries no `# touch` line, `/health` answers, and
      `systemctl status` still reports no warning.

## 3. Name the package in the documentation

- [x] 3.1 Replace the globbed `dpkg -i` lines in `README.md` for the host package, the guest package and the remote install, and verify `grep -n 'dpkg -i' README.md` shows no `_\*.deb` pattern
      `README.md` now derives the package with
      `DEB=$(ls -t dist/vtpm-mds_*+git*.deb | head -1)` and installs `"$DEB"`,
      for the host, the guest and the remote install. It also explains why,
      so the next reader does not restore the glob. `grep -nE 'dpkg (-i|-deb).*_\*\.deb' README.md`
      returns nothing; the only remaining hit in the repository is AGENTS.md
      describing the defect itself.
- [x] 3.2 Verify the documented selection form picks the git-stamped build: run it with both a release and a git-stamped package in `dist/` and confirm it yields the git-stamped one
      With both `vtpm-mds_0.2.0_amd64.deb` and
      `vtpm-mds_0.2.0+git20260920.c2ad576144f2_amd64.deb` present, the
      documented form selected the git-stamped one and `dpkg-deb -f` on it
      reported `0.2.0+git20260920.c2ad576144f2`.
- [x] 3.3 Verify the same ambiguity is not repeated in `AGENTS.md` or in `scripts/deb-local.sh`'s closing hints
      `scripts/deb-local.sh` already prints full filenames in its closing
      hints (`dpkg -i --force-confold ${host_deb}`), so nothing to change.
      AGENTS.md's only occurrence is the defect description.
      Not fixed, outside this repository: `/usr/local/sbin/hogan-lab` prints
      `dpkg-install /usr/local/src/vtpm-mds/dist/vtpm-mds_*.deb` in its usage.
      Its own check is a pattern match on a single argument, so it does not
      expand the glob itself — but its argument loop assigns each `*.deb` it
      sees to the same variable, so a caller whose shell expanded that glob to
      several files installs the **last** one, silently. Same class of defect,
      failing quietly rather than loudly. It is root-owned; recorded in the
      queue as a maintainer item.

## 4. Confirm the package is otherwise unchanged

- [x] 4.1 Run `dpkg-deb -c` on both rebuilt packages and verify the file lists are identical to the previous build apart from the maintainer scripts
      Run by Andreas Dembach, 2026-10-03: `dpkg-deb -c` on the rebuilt package
      and on `vtpm-mds_0.2.0_amd64.deb`, compared on mode and path —
      `Dateiliste identisch`. The maintainer scripts live in the control
      section, not the file tree, so the payload is unchanged as expected.
- [x] 4.2 Run `lintian --fail-on error,warning` on both packages and verify it reports nothing new
      Run by Andreas Dembach as root on `hogan`, 2026-10-03:
      `lintian --fail-on error,warning` on both rebuilt packages exited 0,
      with only lintian's own "running with root privileges is not
      recommended" notice. `lintian` had to be installed first — it was absent
      on this host.
- [x] 4.3 Run `systemd-analyze verify` on the shipped unit and verify it reports no unknown or ignored directive
      `systemd-analyze verify debian/vtpm-mds.service` produced no output and
      exited 0 — no unknown or ignored directive.
- [x] 4.4 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
      `go vet ./...` clean; `go test ./...` all packages ok. No product code
      was touched.
