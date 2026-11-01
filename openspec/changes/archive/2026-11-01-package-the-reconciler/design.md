# Design

## Context

`vtpm-mds-reconcile` (change `reconcile-spire-node-aliases-from-proxmox`)
reads each configured Proxmox cluster over its HTTPS API and writes SPIRE
node aliases through the SPIRE server's local admin socket. It runs once per
trust domain, beside the SPIRE server (that change's A-3), never on a
hypervisor and never in a guest. It keeps no state on disk: ban counts are
held in memory. Its configuration is `/etc/vtpm-mds-reconcile/config.yaml`
by default, decoded strictly, and names per cluster an API URL, an optional CA
file, a token file and the cluster intermediates. Its man page,
`debian/vtpm-mds-reconcile.8`, already exists and shows token files under
`/run/credentials/vtpm-mds-reconcile.service/`.

The source package builds two binary packages today, both with
`CGO_ENABLED=0`, `-trimpath` and `-ldflags "-s -w -X main.Version=…"`; the
release workflow (`.github/workflows/deb.yml`) builds them on
`ubuntu-24.04`, checks xz members and lintian, and attaches both to a
GitHub Release for a `v<changelog version>` tag.

## Goals / Non-Goals

**Goals**

- A `.deb` an operator installs on the SPIRE server host and that runs the
  reconciler as a hardened service once configured.
- A versioned standalone binary for a SPIRE server host that is not Debian.
- The same version string in both, and in `vtpm-mds-reconcile -version`.

**Non-Goals**

- Any change to the reconciler's behaviour, flags or configuration.
- Deployment: which host, the configuration's content, the tokens, the
  SPIRE server's socket ownership. Those are the operator's.
- Container images, `arm64`, an apt repository, signing the release
  artifacts.
- A dependency on SPIRE: SPIRE is not packaged in Debian, and the reconciler
  only needs its socket.

## Decisions

- **D-a. A third binary package `vtpm-mds-reconcile` from the same source.**
  `Architecture: any`, `Section: net`, `Depends: ${misc:Depends}` only. It
  neither depends on nor conflicts with `vtpm-mds` or `devid-enroll`, and
  they do not depend on it. Versioned with them from one changelog.
- **D-b. Contents.** `/usr/sbin/vtpm-mds-reconcile` (a daemon, like
  `vtpm-mds`), `/usr/lib/systemd/system/vtpm-mds-reconcile.service`,
  `vtpm-mds-reconcile(8)`, a `sysusers.d` entry, and
  `/usr/share/doc/vtpm-mds-reconcile/examples/config.yaml` (the man page's
  example, installed by `dh_installexamples` from
  `debian/vtpm-mds-reconcile.examples`), and the package's own
  `debian/vtpm-mds-reconcile.README.Debian` (the unprefixed
  `debian/README.Debian` belongs to `vtpm-mds`). The unit runs
  `ExecStart=/usr/sbin/vtpm-mds-reconcile -config
  /etc/vtpm-mds-reconcile/config.yaml`, explicit like `vtpm-mds.service`.
  Its `README.Debian` is excluded from `dh_compress` so the path the unit,
  the man page and the example name stays valid whatever the file's size. **No conffile in `/etc`:** a shipped `config.yaml` would carry an
  invented cluster, and a strict decoder plus a start-time privilege check
  would make the service fail at once. The host packages' binaries are not in
  it, and its binary is in neither of them.
- **D-c. Its own system user `vtpm-mds-reconcile`**, from `systemd-sysusers`,
  no home, no login, no state directory. Not the SPIRE server's user: the
  reconciler then cannot read SPIRE's keys or datastore, only reach the
  socket an operator lets it reach. The socket is an admin credential in
  itself (whoever opens it is a SPIRE admin), so this limits files, not
  SPIRE rights.
- **D-d. Access to the admin socket is the operator's.** SPIRE creates the
  socket and its directory with its own modes; which group may open it
  depends on how the SPIRE server is installed. The unit names no group; the
  operator adds `SupplementaryGroups=` in a drop-in (documented, with the
  failure the reconciler logs when it cannot open the socket). SPIRE 1.14
  creates the socket `0770` and its two parent directories `0750`, owned
  by the server's user and primary group (task 1.2), so the drop-in is
  `SupplementaryGroups=<the SPIRE server's group>`.
- **D-e. Tokens as systemd credentials, through a drop-in.** Cluster names
  are site-specific, so the unit ships no `LoadCredential=`. The operator's
  drop-in adds one line per cluster,
  `LoadCredential=<cluster>.token:/etc/vtpm-mds-reconcile/<cluster>.token`
  (file `root:root 0600`), and the configuration points `token_file` at
  `/run/credentials/vtpm-mds-reconcile.service/<cluster>.token`, as the man
  page already shows. `ImportCredential=` would need no drop-in but needs
  systemd ≥ 254; Debian 12 ships 252, so it is not the default. Everything
  else the reconciler reads from `/etc/vtpm-mds-reconcile/` (the
  configuration, `ca_file`, the intermediates) must be readable by the
  user: the documentation shows the directory `0750 root:vtpm-mds-reconcile`
  and those files `0640`, while the token files stay `root:root 0600` and
  reach the process only as credentials.
- **D-f. Installing starts nothing; an upgrade restarts a running one.** The
  unit has `ConditionPathExists=/etc/vtpm-mds-reconcile/config.yaml` and is
  enabled on installation, so it starts at the next boot once configured and
  is skipped (not failed) before. `dh_installsystemd` runs for this package
  with `--no-start --no-stop-on-upgrade`, and its `postinst` reloads the
  manager and `try-restart`s the unit on an upgrade, with the guards of the
  host package (`$2` set, no `DPKG_ROOT`, a running manager) and a refused
  restart reported on stderr without failing `dpkg`. The operator starts it
  the first time. Both flags need no new override: `debian/rules` already
  runs `dh_installsystemd --no-start --no-stop-on-upgrade` once for every
  package, so the only per-package work is
  `debian/vtpm-mds-reconcile.postinst` (the hand reload and the guarded
  `try-restart`, after `debian/postinst`) and
  `debian/vtpm-mds-reconcile.prerm`: with `--no-start` debhelper generates
  no stop on removal (the built package had no `prerm` at all), so the
  package stops the unit on `remove` itself, as `debian/prerm` does for the
  host package.
- **D-g. Hardening.** The confinement set of `vtpm-mds.service`, without
  its capability, `StateDirectory` and IP lines: `User=`/`Group=vtpm-mds-reconcile`,
  an empty `CapabilityBoundingSet=`, no `AmbientCapabilities=`, and
  `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`; everything else
  (`ProtectSystem=strict`, `PrivateTmp`, `PrivateDevices`, the kernel,
  clock, hostname and control-group protections, `RestrictNamespaces`,
  `LockPersonality`, `MemoryDenyWriteExecute`, `RestrictRealtime`,
  `RestrictSUIDSGID`, `RemoveIPC`, `UMask=0077`,
  `SystemCallArchitectures=native`, `SystemCallFilter=@system-service`
  minus `@privileged @resources` with `EPERM`) as there. `ProtectSystem=strict`
  does not stop a `connect()` to the admin socket: a socket on a read-only
  mount can still be connected to.
  **`PrivateTmp=true` hides SPIRE's default socket**, which is
  `/tmp/spire-server/private/api.sock`. The unit keeps `PrivateTmp`, and
  the documentation says: put the SPIRE server's `socket_path` under
  `/run` (recommended; the man page example already uses
  `/run/spire/server/private/api.sock`), or, for a server on the default
  path, a drop-in with `PrivateTmp=false`. The socket path is at most 107
  bytes (the kernel's `sun_path`).
  **No `IPAddressAllow=`**: the Proxmox API addresses are site-specific; an
  operator may add them in a drop-in. `Restart=on-failure`, `RestartSec=10s`,
  `StartLimitIntervalSec=300`, `StartLimitBurst=5`,
  `After=`/`Wants=network-online.target`; no ordering on a SPIRE unit, whose
  name differs per installation, since a failed start is retried. Target:
  `systemd-analyze security` at or below 2.0.
- **D-h. The static binary is the packaged binary.** The `build-deb` job,
  which runs on every PR, takes `/usr/sbin/vtpm-mds-reconcile` out of the
  built `.deb` (`dpkg-deb -x`) rather than building again, checks it, and
  uploads it as a fourth artifact; the tag-gated `release` job publishes it
  as `vtpm-mds-reconcile_<version>_linux_amd64` with a
  `vtpm-mds-reconcile_<version>_linux_amd64.sha256` beside it, in
  `sha256sum` format (`<hash>  <filename>`) so `sha256sum -c` works. Both are the
  same bytes, so a checksum of either identifies the other. It is static
  because the rules build with `CGO_ENABLED=0`; the job checks that `file`
  calls it statically linked and that `-version` prints the changelog
  version.
- **D-i. Release notes** name three packages: host, guest, and the
  reconciler for the SPIRE server host, plus the standalone binary. The
  current sentence that the guest package "installs no daemon and no systemd
  unit" is stale since the renewal timer shipped and is dropped.

## Risks / Trade-offs

- **An operator who forgets the socket group, or runs SPIRE on the default
  `/tmp` socket,** gets a reconciler that stops at start with the socket
  error. It restarts every 10 s, each time checking the privileges against
  every Proxmox API first, until the start limit (5 starts in 300 s) leaves
  it failed. The error names the socket; the documentation names both
  drop-ins.
- **No IP restriction by default** (D-g). The reconciler only connects out,
  to configured URLs, but a compromised one could reach anything the host
  reaches. Accepted: the address list is the operator's.
- **The binary outside a package** has no unit, no user and no man page; the
  release notes say so and point to the `.deb` for those.
- **Three packages at one version** means a reconciler-only fix bumps all
  three. Accepted, as for the other two today.

## Migration

None: nothing was packaged before. An operator who built the binary by hand
removes that copy and its unit before installing the package.

## Open Questions

None for the maintainer. SPIRE's socket modes (D-d) are a fact to record
(task 1.2), not a decision.
