# Proposal

## Why

`harden-the-service-unit` left the daemon running as **root** without capabilities
(`systemd-analyze security` 1.9). Root without `CAP_DAC_OVERRIDE` still reads
everything root owns, including `/etc/pve/priv/` (`root:www-data 0700`), where
Proxmox keeps its authentication keys and tokens; the daemon never needs that.
The maintainer decided on 2026-10-18 (D-f of that change) that the daemon gets
its own Unix user, **without any permission change on the host**: `/etc/pve`
keeps the Proxmox default and the site PKI keeps its layout.

## What Changes

- **The daemon runs as the system user `vtpm-mds`**, created by the package, with
  the supplementary group `www-data`, which Proxmox already uses to let
  `pveproxy` read the cluster file system (`root:www-data 0640`). `priv/` stays
  closed to it. Port 80 stays reachable through the ambient
  `CAP_NET_BIND_SERVICE`, which remains the only capability.
- **The CA key reaches the daemon as a systemd credential.** systemd reads it as
  root (`LoadCredential=`) and places a copy readable only by this service under
  `$CREDENTIALS_DIRECTORY`; the daemon prefers that copy to `devid_ca_key`. The
  key file on the host is not touched.
- **`/var/lib/vtpm-mds`** belongs to the new user.

## Capabilities

### Modified Capabilities

- `operability`: "Confine The Daemon To What It Uses" — the daemon runs as
  `vtpm-mds`, not root, and cannot read `/etc/pve/priv/`.
- `devid-enrollment`: one requirement added — the CA key is taken from a systemd
  credential when one is present.

## Impact

- **Code:** `internal/server/server.go` (`loadDevIDEnroller` picks the
  credential), a unit test.
- **Packaging:** `debian/vtpm-mds.service`, `debian/postinst` (system user,
  ownership of `/var/lib/vtpm-mds`), `debian/postrm` if purge removes the user
  (D-b), `debian/README.Debian`.
- **Behaviour visible to operators:** see D-d — a configured CA key that cannot be
  loaded as a credential fails the start instead of disabling enrollment quietly.
- **Not in scope:** a TPM-backed CA key, rotation, the MAC binding on the bridge.
