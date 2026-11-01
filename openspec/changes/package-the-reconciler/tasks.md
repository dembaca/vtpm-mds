# Tasks

Written on 2026-11-01 by `agent-claude-vtpm-05a6b5e52` against `main@ccd0f47`,
from the maintainer's Q-2 decision of 2026-10-29 and the go of 2026-11-01
(ask `mpj6ah7d…`). **Record only until the maintainer's go for the code, and
code only after `reconcile-spire-node-aliases-from-proxmox` is merged.**
Files: `debian/`, `.github/workflows/deb.yml`, `Makefile`,
`scripts/deb-local.sh`, the `README.md` status row. Not `cmd/`, not
`internal/`.

## 1. Record

- [x] 1.1 `proposal.md`, `design.md`, this file, the `debian-packaging` delta
      **Verify:** `openspec validate --all --strict`: Totals 7 passed, 0 failed
      (2026-11-01). The validator refuses a MODIFIED block that drops
      scenarios of the requirement it replaces, so "Two Binary Packages" is
      REMOVED and "Three Binary Packages" ADDED rather than renamed.
- [ ] 1.2 Record SPIRE 1.14's admin socket: path default, mode of the socket
      and of its directory, owner; and whether a user in the socket's group
      can list entries (D-d)
      **Verify:** `stat` output and one `spire-server entry show -socketPath`
      as another user, from a scratch SPIRE server, recorded here
      **Half done, 2026-11-01, scratch SPIRE 1.14.0** run as one user with
      umask `0002`, `socket_path` `<dir>/private/api.sock` in a directory that
      did not exist: SPIRE created `<dir>` and `<dir>/private` as `0750` and
      the socket as `0770`, all owned by the server's user and primary group.
      So a member of the server's group can traverse both directories and
      connect; nobody else can. The default `/tmp/spire-server/private/` is
      created the same way. A socket path of 107 bytes was refused by the
      client (`connect: invalid argument`); the kernel's `sun_path` holds
      107 usable bytes, worth one line in the documentation. **Open:** a connect
      by a second user in that group; this host has only one unprivileged
      user, so it waits for 4.1.
- [x] 1.3 Review of the record by a stronger model (Fable), findings taken or
      deferred by the maintainer
      **Fable, 2026-11-01, of `d98fa9e`:** all taken. Must-fix: `PrivateTmp`
      hides SPIRE's default `/tmp` socket (D-g: socket under `/run` or a
      `PrivateTmp=false` drop-in, tested in 4.1); the living spec's Purpose
      paragraph still says two packages and archiving does not rewrite it
      (5.2). Should-fix: "Local Lab Builds" MODIFIED for "both packages"; a
      start limit (`300 s`/`5`) so the risk text is true, and D-g as the
      set of `vtpm-mds.service`; the Makefile tar exclude and `deb-clean`
      (2.4); D-f needs no new override, `prerm` stops on remove; modes of
      `/etc/vtpm-mds-reconcile/` (D-e). Nits: `.examples` and a
      package-prefixed `README.Debian`, `sha256sum` format, extraction in
      `build-deb`, explicit `ExecStart`, the stale guest sentence in the
      release notes, 107 usable bytes.
- [x] 1.4 Maintainer's go for the code
      **Verify:** ask `rt4y3rt3…`, 👍 2026-11-01 17:20, after #94 and its
      archive (#98) were merged; record rebased onto `1f2d920`

## 2. Package

- [x] 2.1 `debian/control`: package `vtpm-mds-reconcile` (D-a); `debian/rules`
      builds and installs `bin/vtpm-mds-reconcile` and its unit; the existing
      `dh_installsystemd --no-start --no-stop-on-upgrade` covers it, no new
      override (D-f)
      **Verify:** `make deb` (or `make deb-local`) yields three `.deb` at one
      version; `dpkg-deb -c` of each matches "Reconciler Package Ships The
      Reconciler Only" and the host and guest requirements
      **2026-11-01, `make deb-local` at `0.3.2+git20261101.bd447c1edb25`:** three `.deb` at that
      version. `vtpm-mds-reconcile`: `/usr/sbin/vtpm-mds-reconcile`, its unit,
      `sysusers.d/vtpm-mds-reconcile.conf`, the man page, `README.Debian`,
      `examples/config.yaml`, nothing in `/etc`; `Depends:` only the sysusers
      alternatives debhelper adds. `vtpm-mds` and `devid-enroll` contain no
      `reconcile` path and are otherwise unchanged.
- [x] 2.2 `vtpm-mds-reconcile.service` (D-b, D-f, D-g), `.sysusers` (D-c),
      `.manpages`, `.examples` with the example config (D-b), `.postinst`
      (D-f), `.lintian-overrides` (`statically-linked-binary`)
      **Verify:** `systemd-analyze verify` on the unit; `systemd-analyze
      security vtpm-mds-reconcile` ≤ 2.0 after install;
      `lintian --fail-on error,warning` on all three `.deb`
      **2026-11-01:** `lintian --fail-on error,warning` exit 0 on all three.
      `systemd-analyze verify` on the extracted unit: only that
      `/usr/sbin/vtpm-mds-reconcile` does not exist on this host (not
      installed here). `systemd-analyze security --offline=true` on the
      extracted tree: **1.4 OK**; the installed unit waits for 4.1. Found on
      the way: with `--no-start` debhelper generates no stop on removal (the
      first build had no `prerm`), so `debian/vtpm-mds-reconcile.prerm` was
      added (D-f corrected); the rebuilt package has
      `postinst`, `postrm` and `prerm`.
- [x] 2.3 Documentation for the operator in
      `debian/vtpm-mds-reconcile.README.Debian`: the socket-group drop-in
      (D-d); the SPIRE socket under `/run` or a `PrivateTmp=false` drop-in
      for the default `/tmp` path, and the 107-byte limit (D-g); the
      credential drop-in and the modes of `/etc/vtpm-mds-reconcile/` (D-e);
      first start. `vtpm-mds-reconcile(8)` gets only SEE ALSO and FILES
      lines, no behaviour text
      **Verify:** `man -l --warnings` clean; the drop-ins shown are the ones
      used in 4.1
      **2026-11-01:** written; `man -l --warnings` prints nothing. Whether
      the drop-ins work as shown is 4.1.
- [x] 2.4 `Makefile`: the `deb` message; `--exclude=debian/vtpm-mds-reconcile`
      in the `deb` tar (the `debian/vtpm-mds` exclude does not match it);
      `debian/vtpm-mds-reconcile` in the `deb-clean` `rm -rf`.
      `scripts/deb-local.sh` prints the third package
      **Verify:** `make deb-clean && make deb-local` from a clean tree
      **2026-11-01:** the first `make deb-local` ran in a fresh worktree and
      printed all three packages; `make deb-clean` afterwards removed
      `debian/vtpm-mds-reconcile` and `dist/`.

## 3. Release

- [ ] 3.1 `.github/workflows/deb.yml`: the third `.deb` in metadata, xz check,
      lintian, upload, release; in `build-deb` the standalone binary from
      `dpkg-deb -x` with the static and `-version` checks, uploaded as a
      fourth artifact; in `release` the binary and its `sha256sum`-format
      `.sha256` (D-h); release notes (D-i)
      **Verify:** the workflow on the PR is green and its artifacts list three
      `.deb` and the binary; the release job is tag-gated and cannot run on a
      PR, so say so instead of implying it passed
      **2026-11-01, written; open until the PR's run is green.** The
      extraction step's commands ran locally (4.2). The `release` job has not
      run and cannot on a PR.

## 4. Verification

- [ ] 4.1 On a scratch host with a scratch SPIRE 1.14 server: install the
      package; unit enabled, inactive, condition unmet on `start`; write a
      config, the socket-group and credential drop-ins, start: the log shows
      the version and a pass, with the SPIRE socket under `/run`; once more
      with SPIRE on its default `/tmp/spire-server/private/api.sock`: fails
      under the shipped unit, works with the `PrivateTmp=false` drop-in; a
      start without the socket group ends failed after the start limit;
      upgrade while running (restart, new version in the journal) and while
      stopped (stays stopped); `dpkg -r` stops it and leaves no unit; a
      second user in the SPIRE group connects (task 1.2's open half)
      **Verify:** commands and key journal lines recorded here
- [x] 4.2 Standalone binary from the local extraction: `file` says statically
      linked, `-version` prints the package version, hash equals the
      package's binary, `sha256sum -c` passes on the `.sha256`
      **2026-11-01, `0.3.2+git20261101.bd447c1edb25`:** `file`: "ELF 64-bit LSB executable, x86-64,
      … statically linked, … stripped"; `-version`: `vtpm-mds-reconcile
      0.3.2+git20261101.bd447c1edb25`; `cmp` with the packaged binary: identical; `sha256sum -c`: OK
- [x] 4.3 `go vet ./...`, `go test ./...`, `openspec validate --all --strict`
      **2026-11-01:** vet silent, every test package `ok`, validate 8/8

## 5. Ship

- [ ] 5.1 Review of the implementation by a stronger model before ready
- [ ] 5.2 `README.md` status row: "packaged"; after merge the coordinator
      archives the change and edits the Purpose paragraph of
      `openspec/specs/debian-packaging/spec.md` by hand to three packages
      (host, guest, SPIRE server host), since archiving merges requirements
      only
