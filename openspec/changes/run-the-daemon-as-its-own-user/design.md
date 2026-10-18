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

## Decisions, for the maintainer to overrule

- **D-a. A static system user, not `DynamicUser=`.** `adduser --system --group
  --no-create-home --home /var/lib/vtpm-mds vtpm-mds` in `postinst`, if absent.
  A fixed name shows in `ps` and the journal and owns the state directory
  across upgrades; `DynamicUser=` would also work with `SupplementaryGroups=`,
  but moves the state directory under `/var/lib/private` and allocates the UID
  at start.
- **D-b. The user stays on `remove`, goes on `purge`**, with the state
  directory, as Debian policy suggests for system users of a package.
- **D-c. The key as a credential.** The unit carries
  `LoadCredential=devid_ca.key:/etc/ssl/private/proxmox_devid_ca.key`, the path
  the packaged `config.yaml` uses. When `$CREDENTIALS_DIRECTORY/devid_ca.key`
  exists, `loadDevIDEnroller` uses it instead of `devid_ca_key` and logs
  `DevID CA key from systemd credential`; without it (a manual start, tests) the
  configured path is used as today. A site whose key lives elsewhere overrides
  the line with a drop-in; README.Debian shows one.
- **D-d. A missing key file now fails the start.** systemd refuses to start a
  unit whose `LoadCredential=` source does not exist (status 243/CREDENTIALS).
  Today a missing key only disables enrollment. **Recommended:** accept it — on
  a host that is meant to enroll, a silent `404` on enrollment is worse than a
  unit that does not start, and it settles part of the "an unreadable key should
  be fatal" backlog item. **Question for the maintainer:** accept, or keep
  enrollment optional (then the credential line has to move into a drop-in that
  `postinst` writes only when the key exists).
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
- **Upgrades.** `postinst` creates the user before the unit restarts; the
  `--no-start` policy stays, so an operator restarts as today.
