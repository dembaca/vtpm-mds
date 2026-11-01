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
      client (`connect: invalid argument`); the limit is the kernel's
      `sun_path`, worth one line in the documentation. **Open:** a connect
      by a second user in that group; this host has only one unprivileged
      user, so it waits for 4.1.
- [ ] 1.3 Review of the record by a stronger model (Fable), findings taken or
      deferred by the maintainer
- [ ] 1.4 Maintainer's go for the code

## 2. Package

- [ ] 2.1 `debian/control`: package `vtpm-mds-reconcile` (D-a); `debian/rules`
      builds and installs `bin/vtpm-mds-reconcile` and its unit;
      `dh_installsystemd` for it with `--no-start --no-stop-on-upgrade` (D-f)
      **Verify:** `make deb` (or `make deb-local`) yields three `.deb` at one
      version; `dpkg-deb -c` of each matches "Reconciler Package Ships The
      Reconciler Only" and the host and guest requirements
- [ ] 2.2 `vtpm-mds-reconcile.service` (D-f, D-g), `.sysusers` (D-c),
      `.manpages`, the example config (D-b), `postinst` (D-f),
      `lintian-overrides` (`statically-linked-binary`)
      **Verify:** `systemd-analyze verify` on the unit; `systemd-analyze
      security vtpm-mds-reconcile` ≤ 2.0 after install;
      `lintian --fail-on error,warning` on all three `.deb`
- [ ] 2.3 Documentation for the operator: the socket-group drop-in (D-d), the
      credential drop-in (D-e), first start; in `README.Debian` of the
      package or `vtpm-mds-reconcile(8)` (the man page's SEE ALSO and FILES
      only; no behaviour text changes)
      **Verify:** `man -l --warnings` clean; the drop-ins shown are the ones
      used in 4.1
- [ ] 2.4 `Makefile` (`deb` message, `deb-clean`), `scripts/deb-local.sh`
      (prints the third package)
      **Verify:** `make deb-clean && make deb-local` from a clean tree

## 3. Release

- [ ] 3.1 `.github/workflows/deb.yml`: the third `.deb` in metadata, xz check,
      lintian, upload, release; the standalone binary and `.sha256` from
      `dpkg-deb -x` (D-h) with the static and `-version` checks; release notes
      (D-i)
      **Verify:** the workflow on the PR is green and its artifacts list three
      `.deb`; the release job is tag-gated and cannot run on a PR, so say so
      instead of implying it passed; run its binary-extraction steps locally
      against a `make deb-local` output

## 4. Verification

- [ ] 4.1 On a scratch host with a scratch SPIRE 1.14 server: install the
      package; unit enabled, inactive, condition unmet on `start`; write a
      config, the socket-group and credential drop-ins, start: the log shows
      the version and a pass; upgrade while running (restart, new version in
      the journal) and while stopped (stays stopped); `dpkg -r` leaves no unit
      **Verify:** commands and key journal lines recorded here
- [ ] 4.2 Standalone binary from the local extraction: `file` says statically
      linked, `-version` prints the package version, hash equals the
      package's binary
- [ ] 4.3 `go vet ./...`, `go test ./...`, `openspec validate --all --strict`

## 5. Ship

- [ ] 5.1 Review of the implementation by a stronger model before ready
- [ ] 5.2 `README.md` status row: "packaged"; after merge the coordinator
      archives the change
