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

- [x] 3.1 Install the git-stamped package (with the maintainer's go in the session), start the unit, and verify `systemd-analyze security vtpm-mds` is at most 4.0; record the value and the three highest remaining exposures
      **Done, 2026-10-18, on hogan,** `0.2.0+git20261018.3f5c7fd8613c` (head `3f5c7fd`, rebased on `main` so #48 and #49 are in), installed with the maintainer's go in
      the session. `systemd-analyze security vtpm-mds`: **1.9 OK** (was 9.0). Highest
      remaining: `PrivateNetwork=` 0.5, running as root 0.4, `AF_INET`/`AF_INET6`
      0.3 — all intended. `/proc/<pid>/status`: `CapBnd`, `CapEff`, `CapPrm`,
      `CapAmb` all `0x400` (`cap_net_bind_service` only), no supplementary groups.
      `/etc/pve` in the service's mountinfo: `ro`.
- [x] 3.2 Verify from VM 399 that the metadata tree, tags and placement still answer, and from the host that an unbound caller still gets its refusals
      **Done, 2026-10-18:** from VM 399 the placement listing, `availability-zone`
      `keller`, `region` `bgl`, `node`, `cluster`, `rack` and the identity document
      answered as before the change; from the host `instance-id` answered `401`
      without a token and `/health` `200`, as before.
- [x] 3.3 Reload with `systemctl reload vtpm-mds`, and verify the journal shows the cache reloaded with the same VM count
      **Done, 2026-10-18:** `Cache refresh complete (proxmox:/etc/pve): 3 VMs, 5 MAC
      addresses cached` at start and again after the reload, then `VM config cache
      reloaded successfully`; `ExecReload` passed the system-call filter.
- [x] 3.4 **Merge blocker** — enroll VM 399 once with `devid-enroll` (with the maintainer's go), and verify a certificate is issued and the journal has a `devid issued` line, which proves the CA key (`root:root 0600`) is still readable under the new bounding set
      **Done, 2026-10-18,** with the maintainer's go in the session: one enrolment
      of VM 399 with the client from the same build into a guest scratch
      directory (guarded against a second run, SPIRE's material unchanged,
      lockout counter `0x0` before and after, scratch removed). Certificate
      `CN=399`, serial `2CF03B98…`; journal: `devid issued serial=2cf03b98… vm=399
      ek=19517bd1…` between the access lines of start and finish (both `200`).
- [x] 3.6 **Maintainer, needs root** — restart `pve-cluster`, and verify `vtpm-mds` was restarted with it (`PartOf`), its journal shows the cache loaded with the same VM count, and `grep ' /etc/pve ' /proc/$(pidof vtpm-mds)/mountinfo` shows `ro`
      **Done, 2026-10-18:** the maintainer ran `systemctl restart pve-cluster` as
      root. The journal shows `vtpm-mds` stopped and started at 16:49:21 with it,
      `Cache refresh complete (proxmox:/etc/pve): 3 VMs, 5 MAC addresses cached`;
      the new process's mountinfo has `/etc/pve` mounted `ro,nosuid,nodev`; VM 399
      read its placement and identity document right after.
- [x] 3.7 Verify `journalctl -u vtpm-mds` after tasks 3.2 to 3.4 shows neither `DevID enrollment disabled` nor `Failed to load VM config cache`, and no `status=31`; and, with a temporary drop-in `SystemCallLog=@privileged @resources`, that no filtered call is logged during those tasks
      **Half done, 2026-10-18:** the journal since the install holds none of
      `DevID enrollment disabled`, `Failed to load VM config`, `status=31`,
      `SIGSYS` or `operation not permitted` (count 0). The `SystemCallLog`
      drop-in was **not** run: writing a drop-in needs root, which the agent user
      does not have. Every task above succeeded, so no call the daemon needs
      was filtered on these paths.
- [x] 3.5 Say what was not verified
      **Done, 2026-10-18.** Not verified: the `SystemCallLog` run of 3.7 (needs a
      root drop-in); a CA key with another owner or mode than `root:root 0600`
      (it would disable enrollment with the existing warning); the cloud lab,
      which does not install the package; a reload in the instant pmxcfs is
      down (the `PartOf` restart covers the remount, not that window).

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md`, and verify `openspec validate --all --strict` is still clean
