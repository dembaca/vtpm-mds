# Tasks

Written on 2026-10-18 against `main` at `407bfca` by the coordinator on hogan,
from the production-readiness list. **Unit changes only after the record is
pushed and the maintainer has not overruled D-a to D-g; D-f needs his answer.**

## 1. Decide

- [ ] 1.1 **Maintainer, optional** — overrule or accept D-a to D-g in `design.md`, and answer D-f (dedicated user now or later), and verify the outcome is recorded there

## 2. Implement

- [x] 2.1 Add the directives of D-a to D-e to `debian/vtpm-mds.service`, and verify `systemd-analyze verify` on the file reports nothing new
      **Done, 2026-10-18,** on hogan (systemd 257): `systemd-analyze verify
      debian/vtpm-mds.service` printed nothing beyond the absent binary on the
      build path. `systemd-analyze security --offline=true` on the file: **1.9 OK**
      (was 9.0); the largest remaining items are running as root (0.4) and
      `AF_INET`/`AF_INET6` (0.3), both intended. The installed score is task 3.1.
- [x] 2.2 Add the operator note to `debian/README.Debian`, and verify the package still passes `lintian --fail-on error,warning`
      **Done, 2026-10-18:** `make deb-local` at `c1ba511` built
      `vtpm-mds_0.2.0+git20261018.c1ba511d7b9c_amd64.deb`; `lintian --fail-on
      error,warning` exited 0, and the unit in the package carries the new
      directives (`dpkg-deb --fsys-tarfile`).
- [x] 2.3 Run `openspec validate --all --strict`, and verify it passes
      **Done, 2026-10-18:** 6 passed, 0 failed.

## 3. Verify on hogan

- [ ] 3.1 Install the git-stamped package (with the maintainer's go in the session), start the unit, and verify `systemd-analyze security vtpm-mds` is at most 4.0; record the value and the three highest remaining exposures
- [ ] 3.2 Verify from VM 399 that the metadata tree, tags and placement still answer, and from the host that an unbound caller still gets its refusals
- [ ] 3.3 Reload with `systemctl reload vtpm-mds`, and verify the journal shows the cache reloaded with the same VM count
- [ ] 3.4 Re-enroll VM 399 with `devid-enroll` (with the maintainer's go), and verify a certificate is issued, which proves the CA key is still readable under the new bounding set
- [ ] 3.6 **Maintainer, needs root** — restart `pve-cluster`, then `systemctl reload vtpm-mds`, and verify the journal shows the cache reloaded with the same VM count and no `ENOTCONN`; if it does not, record that a restart of `vtpm-mds` is needed and add `PartOf=pve-cluster.service` in a follow-up
- [ ] 3.7 Verify `journalctl -u vtpm-mds` shows no `SIGSYS`, `status=31` or `operation not permitted` after tasks 3.2 to 3.4
- [ ] 3.5 Say what was not verified

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md`, and verify `openspec validate --all --strict` is still clean
