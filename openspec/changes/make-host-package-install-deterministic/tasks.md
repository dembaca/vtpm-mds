# Tasks

## 1. Reproduce both defects

- [ ] 1.1 With two host packages in `dist/`, verify `dpkg-deb -f dist/vtpm-mds_*.deb Version` reports the stale version and `printf '%s\n' dist/vtpm-mds_*.deb` shows the release package sorting first — record both outputs as the baseline
- [ ] 1.2 Extract the built package with `dpkg-deb -e` and verify its `postinst` contains no `systemctl daemon-reload`, only the `deb-systemd-helper` enable calls

## 2. Reload the manager on install

- [ ] 2.1 Extend `override_dh_installsystemd` in `debian/rules` so the generated `postinst` reloads the systemd manager, keeping `--no-start`, and verify with `dpkg-deb -e` that the rebuilt package's `postinst` now contains the reload
- [ ] 2.2 Verify the reload is guarded on a running manager and cannot fail the install, by checking the snippet tests `[ -d /run/systemd/system ]` and ends in `|| true`
- [ ] 2.3 Verify the package still does not start the daemon: install on a host where the listen address cannot be bound and verify the install succeeds with the unit enabled and nothing running
- [ ] 2.4 Verify the warning is gone end to end: change a comment in `debian/vtpm-mds.service`, rebuild, `dpkg -i --force-confold`, and verify `systemctl status vtpm-mds` no longer reports that the unit file changed on disk — needs root on a lab host, record who ran it

## 3. Name the package in the documentation

- [ ] 3.1 Replace the globbed `dpkg -i` lines in `README.md` for the host package, the guest package and the remote install, and verify `grep -n 'dpkg -i' README.md` shows no `_\*.deb` pattern
- [ ] 3.2 Verify the documented selection form picks the git-stamped build: run it with both a release and a git-stamped package in `dist/` and confirm it yields the git-stamped one
- [ ] 3.3 Verify the same ambiguity is not repeated in `AGENTS.md` or in `scripts/deb-local.sh`'s closing hints

## 4. Confirm the package is otherwise unchanged

- [ ] 4.1 Run `dpkg-deb -c` on both rebuilt packages and verify the file lists are identical to the previous build apart from the maintainer scripts
- [ ] 4.2 Run `lintian --fail-on error,warning` on both packages and verify it reports nothing new
- [ ] 4.3 Run `systemd-analyze verify` on the shipped unit and verify it reports no unknown or ignored directive
- [ ] 4.4 Run `go vet ./...` and `go test ./...` and verify both pass, confirming no product code was touched
