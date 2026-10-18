# Design

## Context

Evidence read on hogan on 2026-10-18 against `main@407bfca` and the installed
`0.2.0+git20261018.eaed1d1b54b6`:

- `debian/vtpm-mds.service`: `AmbientCapabilities=CAP_NET_BIND_SERVICE`,
  `LimitNOFILE=65536`, `ReadWritePaths=/var/lib/vtpm-mds`, `ProtectHome=true`,
  `PrivateTmp=true`, `NoNewPrivileges=true`; no `User=`, so root with the full
  capability set. `systemd-analyze security`: **9.0 UNSAFE** (systemd 257).
- Files read, with owner and mode: `/etc/pve/local/qemu-server/*.conf`,
  `/etc/pve/corosync.conf` and `/etc/pve/local/config` are `root:www-data 0640`
  (pmxcfs, FUSE); `/etc/ssl/private` is `root:ssl-cert 0710`; the CA
  certificates are `root:ssl-cert 0644`. The mode of the CA key itself cannot be
  read by the agent user (it cannot traverse `/etc/ssl/private`), so whether a
  member of `ssl-cert` could read it is unknown; root owns it.
- `/proc/net/arp` is world-readable. The service makes no outbound connection: it
  only accepts on `169.254.169.1:80`.

## Decisions, for the maintainer to overrule

- **D-a. Keep `root`, drop every capability but one.**
  `CapabilityBoundingSet=CAP_NET_BIND_SERVICE` with the existing ambient set. Root
  without `CAP_DAC_OVERRIDE`/`CAP_DAC_READ_SEARCH` still reads files it owns,
  which covers every path above, and loses the power to read anything else. This
  is the largest gain for the smallest risk, because file access does not change.
- **D-b. Read-only system.** `ProtectSystem=strict`, so `/usr`, `/boot`, `/etc`
  and the rest are read-only; `ReadWritePaths=/var/lib/vtpm-mds` stays.
  `/etc/pve` is a FUSE mount and is reached read-only through the same bind; the
  daemon never writes there. `ProtectHome=true` and `PrivateTmp=true` stay.
- **D-c. Kernel and devices.** `PrivateDevices=true`, `ProtectKernelTunables`,
  `ProtectKernelModules`, `ProtectKernelLogs`, `ProtectControlGroups`,
  `ProtectClock`, `ProtectHostname`, `RestrictNamespaces=true`,
  `LockPersonality=true`, `MemoryDenyWriteExecute=true` (Go does not need W+X),
  `RestrictRealtime=true`, `RestrictSUIDSGID=true`, `RemoveIPC=true`,
  `UMask=0077`. `RemoveIPC` removes IPC objects of the service's user and does
  nothing for root; it is kept for when D-f lands and is not counted as
  protection now, although it lowers the offline score. `ProtectProc=invisible` and `ProcSubset=pid` are **not** set:
  the caller binding reads `/proc/net/arp`.
- **D-d. System calls and families.** `SystemCallArchitectures=native`,
  `SystemCallFilter=@system-service` and `~@privileged @resources`;
  `SystemCallErrorNumber=EPERM`, so a call the filter blocks fails with an
  error the daemon logs instead of killing it with `SIGSYS`;
  `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`. No `AF_NETLINK`: the
  daemon lists no interfaces (no `net.Interface*` outside tests on `main`), it
  only listens and reads `/proc/net/arp` (pre-review, 2026-10-18).
- **D-e. Address filter.** `IPAddressDeny=any`,
  `IPAddressAllow=169.254.0.0/16 localhost` (`localhost` is systemd's token for
  `127.0.0.0/8` and `::1/128`). Guests reach the service from link-local
  addresses on `vmbr_imds`, and the host's own calls come from `169.254.169.1`.
  A guest given any other source address on that bridge times out silently;
  README.Debian says so.
- **D-f. A dedicated user, not in this change.** `User=vtpm-mds` with
  `SupplementaryGroups=www-data ssl-cert` would remove root entirely, but
  `www-data` reads the whole cluster file system except `priv/`, and whether
  `ssl-cert` can read the CA key depends on a mode only root can see. **Question
  for the maintainer:** stay with D-a (recommended for now), or have the key's mode
  checked (`stat /etc/ssl/private/proxmox_devid_ca.key`) and add the user here?
- **D-g. The target is a score of at most 4.0** on systemd 257, measured on the
  installed unit, not on the file in the repository.

## Non-Goals

- Code changes. The daemon is not told about the sandbox.
- A dedicated user (D-f), unless the maintainer pulls it in.
- Moving the CA key, or putting it into a TPM.
- The reload trigger, MAC binding on the bridge, rate limits: separate changes.

## Risks

- **A directive that breaks a path not exercised by the checks.** Enrollment reads
  the CA key only when a guest enrolls; SIGHUP rereads `/etc/pve`. Both are in the
  verification tasks, with an enrollment of VM 399 under the new unit.
- **`ProtectSystem=strict` over pmxcfs.** Already ruled out: the unit had a mount
  namespace before (`ReadWritePaths`) and `/etc/pve` is visible in it.
- **`/etc/pve` after a `pve-cluster` restart (Fable pre-review, 2026-10-18).**
  The namespace receives the new pmxcfs mount by propagation, but systemd's
  read-only remount runs only at service start, so the remounted `/etc/pve`
  would arrive writable. **D-h: `PartOf=pve-cluster.service`**, so a restart of
  `pve-cluster` restarts `vtpm-mds` and rebuilds the read-only view. Task 3.6
  checks `ro` in the service's mountinfo after such a restart.
- **An unreadable CA key does not fail the start (Fable pre-review).**
  `loadDevIDEnroller` failing only logs `Warning: DevID enrollment disabled`;
  the service starts, `/health` says ok, enrollment answers `404`. Without
  `CAP_DAC_OVERRIDE`, root reads only files it owns or may read by group, and a
  sibling file of the same site PKI (`proxmox_imds_ca.crt`) is
  `ansible:ssl-cert 0640`. **D-i: `SupplementaryGroups=ssl-cert`**, which also
  lets the process traverse `/etc/ssl/private` (`root:ssl-cert 0710`) by group.
  Task 3.4 (an enrollment) is a merge blocker, and 3.7 greps for the warning.
  Making an unreadable configured key fatal is a separate change.
- **`EPERM` is not logged by itself.** A swallowed system call leaves no line;
  for the verification run, `SystemCallLog=@privileged @resources` in a drop-in
  shows any hit in the audit/journal. `~@resources` blocks `setrlimit` but not
  `prlimit64`, which Go uses, so that part of the filter is cosmetic; Go skips the
  raise anyway because `LimitNOFILE` sets soft equal to hard.
