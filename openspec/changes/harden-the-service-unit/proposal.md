# Proposal

## Why

**The daemon runs as full root with almost no confinement.** On hogan,
`systemd-analyze security vtpm-mds` rates the installed unit **9.0 UNSAFE**
(2026-10-18, `0.2.0+git20261018.eaed1d1b54b6`). The unit sets only
`ProtectHome`, `PrivateTmp` and `NoNewPrivileges`. The process parses TPM
structures and certificates that guests send, and can read the DevID CA private
key, so a parsing defect would hand a guest root on the Proxmox host with every
capability. The production-readiness list of 2026-10-18 put this second; the
maintainer decided that every item on it is addressed before production.

What the daemon actually touches is small and known: it binds
`169.254.169.1:80`, reads `/etc/vtpm-mds/config.yaml`, `/etc/pve` (guest configs,
`corosync.conf`, the node's `config`), `/proc/net/arp`, the CA certificates under
`/etc/ssl/certs` and the DevID CA key under `/etc/ssl/private`, and writes
nothing but its log. Every file it reads on hogan is owned by root.

## What Changes

- **The shipped unit confines the daemon** with systemd's sandboxing: a read-only
  file system, no devices, no kernel tunables, modules or logs, no new namespaces,
  a system-call filter, the four address families it uses, and a capability
  bounding set of `CAP_NET_BIND_SERVICE` alone.
- **It may talk only to link-local and loopback addresses** (`IPAddressDeny=any`
  with `IPAddressAllow` for `169.254.0.0/16` and loopback). The service has no
  outbound traffic, so this costs nothing and contains a compromised process.
- **It still runs as `root`**, but without `CAP_DAC_OVERRIDE` or any other
  capability, so it reads only what root owns or what is world-readable. Running
  under a dedicated user is decided in D-f.
- **The score is checked and recorded:** `systemd-analyze security` on the
  installed unit must come out at or below **4.0** (OK), and the measured value is
  recorded in `tasks.md`.

## Capabilities

### Modified Capabilities

- `operability`: one requirement added, "Confine The Daemon To What It Uses".

## Impact

- **Packaging:** `debian/vtpm-mds.service` only. No code changes.
- **Docs:** `debian/README.Debian` gains a short note on what the unit allows,
  for an operator who moves the CA key or the inventory.
- **Behaviour visible to operators:** a configuration that points the daemon at a
  key or inventory outside the allowed, root-owned or world-readable paths fails at
  start with a permission error in the journal instead of being read. The
  `ReadWritePaths=/var/lib/vtpm-mds` line stays until the state directory is
  removed with the dead configuration keys (a later change).
- **Not in scope:** a dedicated user (D-f, unless the maintainer wants it here),
  the reload trigger for new VMs, MAC binding on the IMDS bridge, the dead keys.
