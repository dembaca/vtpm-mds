# Tasks

Written on 2026-10-18 by the coordinator on hogan, after the maintainer's
answer to D-f of `harden-the-service-unit`. Based on the archive branch that
carries that change's requirement; rebase on `main` once the archive is merged.
**Code only after the record has its pre-review (Fable) and the maintainer has
answered D-d.**

## 1. Decide

- [x] 1.1 **Maintainer** — answer D-d (missing key fails the start, or enrollment stays optional), and optionally overrule D-a to D-e, and verify the outcome is recorded in `design.md`
      **Done, 2026-10-18:** on the bus, (a) — a missing key fails the start;
      recorded under "D-d answered". Nothing else overruled.
- [x] 1.2 **Pre-review** — a Fable review of this record, and verify its findings are fixed or deferred by the maintainer
      **Done, 2026-10-18:** Fable found no reason to rethink and eight points; all
      are taken into the record: the enroll-route requirement is MODIFIED so a
      credential satisfies the key and the configured path is not stat-ed (it
      would fail as `vtpm-mds`); the user comes from `systemd-sysusers` (avoids a
      lintian warning and a hand-written `postinst`) with `StateDirectory=`; the
      drop-in example resets `LoadCredential=`; D-d records `SetCredential=` as
      the optional variant; task 3.4's positive control reads a `.conf` file;
      reload versus restart for a rotated key; D-b no longer cites policy.

## 2. Implement

- [x] 2.1 Prefer `$CREDENTIALS_DIRECTORY/devid_ca.key` in `loadDevIDEnroller` without touching the configured path, and verify with unit tests for credential present (with `devid_ca_key` pointing at a path that does not exist), absent, and empty, each with a mutation that fails it
      **Done, 2026-10-18:** `credentialCAKeyPath` and its use in
      `loadDevIDEnroller` (`internal/server/server.go`); tests in
      `internal/server/credential_test.go`: a credential with `devid_ca_key`
      pointing into a missing directory loads and logs `DevID CA key from systemd
      credential`; an empty `devid_ca_key` with a credential loads; no directory,
      a directory without the file and an empty file each fall back to the
      configured key without the log line. Mutations, each red: credential
      ignored (2 tests fail), empty credential accepted (`empty credential`
      fails), log line removed (1 test fails). `go vet ./...` and `go test ./...`
      pass.
- [x] 2.2 Add `debian/vtpm-mds.sysusers` and `--with installsysusers`, and verify with `dpkg-deb -c`/`-I` that the sysusers file and its dependency are in the package, and `lintian --fail-on error,warning` passes
      **Done, 2026-10-18:** `vtpm-mds_0.2.0+git20261018.493063dcb393_amd64.deb`
      ships `/usr/lib/sysusers.d/vtpm-mds.conf`, `Depends:` carries `systemd |
      systemd-standalone-sysusers | systemd-sysusers`, `postinst` runs
      `systemd-sysusers vtpm-mds.conf` (added by `dh_installsysusers` 13.24.2), and
      `lintian --fail-on error,warning` exits 0.
- [x] 2.3 Set `User=vtpm-mds`, `Group=vtpm-mds`, `SupplementaryGroups=www-data`, `StateDirectory=vtpm-mds` and `LoadCredential=` in the unit (drop `ReadWritePaths=`), update the unit comments and README.Debian (drop-in example with a reset line, restart after key rotation), and verify `systemd-analyze verify` and `--offline` security
      **Done, 2026-10-18:** unit and README.Debian changed as stated;
      `systemd-analyze verify` reports nothing, `--offline=true` security **1.5 OK**
      (1.9 as root).
- [x] 2.4 Run `go vet ./...`, `go test ./...` and `openspec validate --all --strict`, and verify all pass
      **Done, 2026-10-18:** `go vet ./...` clean, `go test ./...` passes, `openspec
      validate --all --strict` 6 passed.

## 3. Verify on hogan

- [x] 3.1 Install the git-stamped package (maintainer's go), and verify the process runs as `vtpm-mds` with groups `vtpm-mds www-data`, the bounding set is `CAP_NET_BIND_SERVICE` only, and `systemd-analyze security` is at most the 1.9 measured for #51
      **Done, 2026-10-18, on hogan,** `0.2.0+git20261018.493063dcb393` (code at
      `493063d`), installed with the maintainer's go in the session. `postinst`
      printed `Creating user 'vtpm-mds' … with UID 987 and GID 987`. The process:
      `Uid 987`, `Gid 987`, `Groups: 33 987` (`www-data`, `vtpm-mds`),
      `CapEff`/`CapBnd` `0x400`. `/var/lib/vtpm-mds` is now `vtpm-mds:vtpm-mds
      0750`. `systemd-analyze security vtpm-mds`: **1.5 OK** (1.9 as root).
- [x] 3.2 Verify the journal says `DevID CA key from systemd credential`, and enroll VM 399 once (maintainer's go) so a `devid issued` line proves the key is used
      **Done, 2026-10-18:** at start the journal says `DevID CA key from systemd
      credential` and `DevID enrollment routes registered`. One enrolment of VM
      399 (maintainer's go; client from the same build; guest scratch directory,
      guarded, removed afterwards; SPIRE's material unchanged; lockout counter
      `0x0` before and after) issued `CN=399`, serial `BD43C303…`; journal
      `devid issued serial=bd43c303… vm=399 ek=19517bd1…`.
- [x] 3.3 Verify metadata from VM 399 and the host's refusals as before, and that a reload rereads 3 VMs
      **Done, 2026-10-18:** VM 399 read the placement listing, zone, rack and the
      identity document as before; the host got `401` without a token and `200`
      on `/health`; a reload printed `3 VMs, 5 MAC addresses` and `reloaded
      successfully`. The journal since the install holds no `DevID enrollment
      disabled`, `Failed to load VM config`, `permission denied`, `operation not
      permitted` or `SIGSYS`.
- [x] 3.4 **Maintainer, needs root** — run `setpriv --reuid=vtpm-mds --regid=vtpm-mds --groups=www-data ls /etc/pve/priv` and verify it is refused while `cat /etc/pve/local/qemu-server/399.conf` under the same identity succeeds
      **Done, 2026-10-18, by the maintainer as root:** `ls /etc/pve/priv` as
      `vtpm-mds` with `www-data` → `cannot open directory '/etc/pve/priv':
      Permission denied`; `cat /etc/pve/local/qemu-server/399.conf` under the same
      identity printed the VM's configuration (not recorded here).
- [x] 3.5 Say what was not verified
      **Done, 2026-10-18.** Not verified: a missing key file failing the start
      with `243/CREDENTIALS` on the live host (shown by `man systemd.exec` and the
      pre-review, not provoked here — it would take the service down); a key at
      another path via the documented drop-in; a purge (the user stays by design);
      the cloud lab, which does not install the package.

## 4. Archive

- [x] 4.1 Archive with `openspec archive`, update `AGENTS.md`, and verify `openspec validate --all --strict` is still clean
      **Done, 2026-10-18,** by the coordinator from `main` at `18dd13f`, after #55
      was merged. `AGENTS.md` updated.
