# Tasks

Written on 2026-10-18 by the coordinator on hogan, after the maintainer's
answer to D-f of `harden-the-service-unit`. Based on the archive branch that
carries that change's requirement; rebase on `main` once the archive is merged.
**Code only after the record has its pre-review (Fable) and the maintainer has
answered D-d.**

## 1. Decide

- [ ] 1.1 **Maintainer** — answer D-d (missing key fails the start, or enrollment stays optional), and optionally overrule D-a to D-e, and verify the outcome is recorded in `design.md`
- [ ] 1.2 **Pre-review** — a Fable review of this record, and verify its findings are fixed or deferred by the maintainer

## 2. Implement

- [ ] 2.1 Prefer `$CREDENTIALS_DIRECTORY/devid_ca.key` in `loadDevIDEnroller`, and verify with a unit test for both cases (credential present, absent) and a mutation that fails it
- [ ] 2.2 Create the `vtpm-mds` system user in `postinst`, own `/var/lib/vtpm-mds` by it, remove both on purge, and verify with `dpkg-deb -I`/`-c` and `lintian --fail-on error,warning`
- [ ] 2.3 Set `User=vtpm-mds`, `Group=vtpm-mds`, `SupplementaryGroups=www-data` and `LoadCredential=` in the unit, and verify `systemd-analyze verify` and `--offline` security
- [ ] 2.4 Run `go vet ./...`, `go test ./...` and `openspec validate --all --strict`, and verify all pass

## 3. Verify on hogan

- [ ] 3.1 Install the git-stamped package (maintainer's go), and verify the process runs as `vtpm-mds` with groups `vtpm-mds www-data`, the bounding set is `CAP_NET_BIND_SERVICE` only, and `systemd-analyze security` is at most the 1.9 measured for #51
- [ ] 3.2 Verify the journal says `DevID CA key from systemd credential`, and enroll VM 399 once (maintainer's go) so a `devid issued` line proves the key is used
- [ ] 3.3 Verify metadata from VM 399 and the host's refusals as before, and that a reload rereads 3 VMs
- [ ] 3.4 **Maintainer, needs root** — run `setpriv --reuid=vtpm-mds --regid=vtpm-mds --groups=www-data ls /etc/pve/priv` and verify it is refused while `ls /etc/pve/local/qemu-server` under the same identity succeeds
- [ ] 3.5 Say what was not verified

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md`, and verify `openspec validate --all --strict` is still clean
