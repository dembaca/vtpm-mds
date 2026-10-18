# Design

## Context

Read on hogan on 2026-10-18 (`main` with #51 merged, installed
`0.2.0+git20261018.3f5c7fd8613c`):

- The daemon reads `/etc/vtpm-mds/config.yaml` (`root:root 0644`), the guest
  configs, `corosync.conf` and the node `config` under `/etc/pve`
  (`root:www-data 0640`), `/proc/net/arp` (`0444`), the CA certificates under
  `/etc/ssl/certs` (`0644`) and the CA key `/etc/ssl/private/proxmox_devid_ca.key`
  (`root:root 0600`, directory `root:ssl-cert 0710`). It writes nothing but its
  log; `/var/lib/vtpm-mds` (`root:root 0750`) is unused.
- `/etc/pve/priv` is `root:www-data 0700`: root reads it, `www-data` does not.
- `loadDevIDEnroller` (`internal/server/server.go`) takes the key path from
  `devid_ca_key`; a key it cannot load only disables enrollment with
  `Warning: DevID enrollment disabled`.

## Decided (maintainer, 2026-10-18)

A dedicated user, and no permission change on `/etc/pve` or the PKI.

**D-d answered (maintainer, 2026-10-18): (a), a missing key fails the start.**
Enrollment happens on every node that runs vTPM guests, because a guest only
reaches the service of the node it runs on; a node without its key must stand
out, not serve `404` quietly.

## Decisions, for the maintainer to overrule

- **D-a. A static system user from `systemd-sysusers`, not `DynamicUser=`.**
  `debian/vtpm-mds.sysusers` with `u vtpm-mds - "vtpm-mds metadata service"
  /var/lib/vtpm-mds`, installed by `dh $@ --with installsysusers` (compat 13
  does not run it by default). That brings its own dependency, so lintian's
  `maintainer-script-needs-depends-on-adduser` does not apply, and needs no
  hand-written `postinst` code. A fixed name shows in `ps` and the journal;
  `DynamicUser=` would move the state directory under `/var/lib/private` and
  allocate the UID at start. `StateDirectory=vtpm-mds` replaces
  `ReadWritePaths=/var/lib/vtpm-mds` and makes systemd own the directory by the
  user at each start (pre-review, 2026-10-18).
- **D-b. The user stays after `purge`.** adduser's own guidance advises against
  deleting system users (files left behind, UID reuse); sysusers never removes
  them. This is a choice, not a policy requirement.
- **D-c. The key as a credential.** The unit carries
  `LoadCredential=devid_ca.key:/etc/ssl/private/proxmox_devid_ca.key`, the path
  the packaged `config.yaml` uses. When `$CREDENTIALS_DIRECTORY/devid_ca.key`
  exists, `loadDevIDEnroller` uses it instead of `devid_ca_key` and logs
  `DevID CA key from systemd credential`; without it (a manual start, tests) the
  configured path is used as today. A site whose key lives elsewhere overrides
  the line with a drop-in; README.Debian shows one, and it must begin with an
  empty `LoadCredential=` line, because the setting accumulates and the packaged
  path would otherwise still be required. The configured `devid_ca_key` is not
  read or even stat-ed when the credential is present: as `vtpm-mds` the service
  cannot traverse `/etc/ssl/private` (`root:ssl-cert 0710`), and a stat would
  fail. SIGHUP never rereads the key, so credentials fixed at start are enough;
  a rotated key needs a restart, not a reload.
- **D-d. A missing key file now fails the start.** systemd refuses to start a
  unit whose `LoadCredential=` source does not exist (status 243/CREDENTIALS).
  Today a missing key only disables enrollment. **Recommended:** accept it — on
  a host that is meant to enroll, a silent `404` on enrollment is worse than a
  unit that does not start, and it settles part of the "an unreadable key should
  be fatal" backlog item. **Question for the maintainer:** accept, or keep
  enrollment optional. The optional variant needs no generated drop-in:
  `SetCredential=devid_ca.key:` (an empty default) next to `LoadCredential=`
  makes a missing file non-fatal, and the daemon treats an empty credential as
  absent (man `systemd.exec`, confirmed in the pre-review).
- **D-e. Everything else of the confinement stays** (bounding set, read-only
  system, filters, `PartOf`). `RemoveIPC=` now takes effect, because the user is
  not root.

## Non-Goals

- Changing owners or modes of anything under `/etc/pve` or `/etc/ssl`.
- A TPM-backed or remote CA key; rotation; the MAC binding on `vmbr_imds`.

## Risks

- **`www-data` reads the whole cluster file system except `priv/`.** That is what
  `pveproxy` reads; it is less than root reads today.
- **A key path in a site's config that differs from the unit's line** makes the
  start fail (D-d) until the drop-in is added; README.Debian and the journal
  (`243/CREDENTIALS`) say so.
- **Upgrades.** The `preinst` of `dh_installsystemd` stops the running root
  process before unpacking; `postinst` then creates the user and reloads
  systemd, and nothing starts the unit (`--no-start`), so the operator runs
  `systemctl start vtpm-mds`. At that start `StateDirectory=` chowns the old
  root-owned `/var/lib/vtpm-mds` to the user (seen on hogan, 3.1).

## Follow-ups from the code pre-review (Fable, 2026-10-18)

Left out so the package measured on hogan stays the one that ships:
the `mkdir`/`chmod` of `/var/lib/vtpm-mds` in `debian/postinst` is now dead
(`StateDirectory=` owns the directory); and when a site's `devid_ca_key` names
another path, the credential wins with only the generic log line — a line
naming the ignored path would help. Both belong to the next change that
touches `debian/` or `loadDevIDEnroller`.
