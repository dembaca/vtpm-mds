# Agent instructions for vtpm-mds

Read this before editing anything. This repository is spec-driven with
[OpenSpec](https://openspec.dev): `openspec/specs/` is the living description of
current behaviour, `openspec/changes/` holds in-flight work. Code that changes
behaviour without a change record is the thing this project most wants to avoid.

## Always start here

1. `ls openspec/changes/` and read every document of the change you are about to
   work on — `proposal.md`, `design.md`, `tasks.md` and each
   `specs/<capability>/spec.md` delta. The delta is the contract; `tasks.md` is
   the order of work and each task names its own verification.
2. Read `openspec/specs/` for the capabilities you touch, when they exist.
3. Work task by task. Tick a checkbox only after running the verification that
   the task states, and say what the output was.
4. Validate before opening a PR with `openspec validate --all --strict`. The
   CLI is installed on `hogan` (see "Lab host facts"), so that is the check to
   run here and it is authoritative. On a host without node, fall back to
   `scripts/openspec-validate.py --strict`, which covers the same structure but
   is a floor rather than a replacement.
5. When a change ships, archive it: move each `specs/<capability>/spec.md` delta
   into `openspec/specs/<capability>/spec.md`, move the change directory under
   `openspec/changes/archive/`, and update the status table in `README.md`.

## Scope discipline

The declared scope is a contract, and `design.md` non-goals are part of it.

- Work only on what the active change covers.
- If you find something else that must change — a behaviour gap, a lost flag, a
  broken default — **write a new change for it** (`proposal.md`, `design.md`,
  `tasks.md`, spec delta) rather than folding it into the change you are on.
  Implementing it in the same PR is acceptable when it blocks verification; the
  separate change record is not optional.
- Build tooling, packaging plumbing and docs that a change's Impact section
  already names are in scope. Product behaviour — flags, endpoints, status
  codes, defaults, wire formats — is not, unless a delta says so.

This rule exists because `-version` on both binaries and a systemd
`StartLimit*` fix were once shipped as a side effect of a packaging change,
against that change's own non-goals.

## Never leave work uncommitted

This repository has already lost work this way: the `-version` flags on
`vtpm-mds` and `devid-enroll`, their man page entries and a systemd fix existed
only in a dirty working tree, were built into an installed `.deb`, and vanished
when the tree was restored. `README.md` documented flags that no commit
contained.

- Commit or stash before switching branches, creating worktrees, or running
  anything that restores files.
- `make deb` and `make deb-clean` delete `.deb-build/`; never treat a staging
  copy as a backup.
- Before any destructive step, check `git status --porcelain` and copy anything
  untracked somewhere durable. `/tmp` on the lab host is tmpfs.

## Verify on real artifacts

For packaging and daemon work, inspect what is actually produced, not just the
source: `dpkg-deb -c` / `-I` on each `.deb`, `lintian --fail-on error,warning`,
`systemd-analyze verify` on the unit, and run the extracted binary. Where a
check cannot run (no passwordless sudo on the lab host, tag-gated release
jobs), say so explicitly instead of implying it passed.

Tests and vet must pass: `go vet ./...` and `go test ./...`.

## Coordinating subagents

- Give each subagent a disjoint, explicit file list; overlapping edits to
  `debian/` or `README.md` are the usual source of conflicts.
- Subagents share the parent's shell session. Do not mutate `PATH`,
  `PERL5LIB` or other environment variables in it — a leaked `PATH` entry once
  shadowed `make` with an unusable build and broke the parent's session.
- Tell each subagent to commit on its own branch and not to push, so the
  coordinator merges and verifies the combined tree.
- Use a git worktree per subagent when they run in parallel. They share one
  shell session, so they cannot each check out a different branch in the same
  tree.

## Coordinating with cloud agents

Cursor and Claude cloud agents work on this repository too, and they branch
from `main` rather than from whatever a local session has in flight. On
2026-10-02 that produced two independent implementations of the same two
changes, from the same base. Nothing was lost, but only because both sides had
committed everything and the proposals and designs were untouched on both.

- **Push `main` before handing work to a cloud agent**, and have it branch from
  the pushed commit. A local session sitting on twenty unpushed commits is how
  the duplicate happened.
- **Split by file, and say so in the briefing.** The split that works today:
  `scripts/qemu-lab/` is the cloud lab's, `debian/` and the root `README.md`
  belong to whoever is doing packaging, and product code under `imds/`,
  `identity/`, `internal/` is a third set.
- **One hand on `AGENTS.md` and the root `README.md`.** Both attract edits from
  every direction; concurrent ones conflict for no benefit.
- **Archiving stays with the coordinator**, so the queue below, the README
  status table and `openspec/specs/` move together.
- **A cloud agent verifies what only it can.** It has a working QEMU lab; this
  host does not and must not — see the warning below. Tasks that need the lab
  belong there. Conversely, anything needing the Proxmox stack, VM 399 or the
  installed package belongs here.
- Tell it to record what it could **not** verify in its environment. A cloud
  box has the default `swtpm-localca` layout and `/usr/share/OVMF/`; this host
  has a site CA named in `/etc/swtpm_setup.conf` and `pve-edk2-firmware`. A run
  that only covers one of those should say which.

## Known gaps, and the queue as of 2026-10-17

All six numbered entries below are **implemented, verified and archived** —
item 6 as a removal rather than an implementation, which is what the analysis
under it concluded. So are the three unnumbered defects further down —
`public-ipv4`, the `127.0.0.1` fallback and the vTPM dictionary-attack
lockout, and so is the constant identity signature. Nothing specified is
open; the queue holds only the deferred work listed next.

Verification used the real Proxmox stack rather than the QEMU lab wherever it
could: VM **399** (`vtpm-pilot`) on `vmbr_imds` with MAC `bc:24:11:06:1d:b2`
and a vTPM is the project's test guest, reachable through
`sudo hogan-lab qm guest exec 399 …`. It has `python3` but **no `curl`**. The
host itself, which has no ARP entry for its own address, serves as an unbound
caller without needing a netns. Prefer both to `scripts/qemu-lab/`, which
cannot run here — see the warning below.

Item 6 was written, implemented and archived on 2026-10-16, as a **removal**:
`openspec/changes/archive/2026-10-16-remove-the-unverifiable-identity-endpoints/`.
Its design decisions — signing key type, key location, rotation, which PCRs a
quote must cover — never had to be settled, because the conclusion was that
nothing should consume them. The analysis is kept under item 6 so it is not
re-derived.

### Open changes as of 2026-10-19

None. Five were archived on 2026-10-17, seven on 2026-10-18 and four on 2026-10-19; the first two in this order:

- `openspec/changes/archive/2026-10-17-serve-a-public-address-or-omit-the-key/`,
  the `public-ipv4` defect.
- `openspec/changes/archive/2026-10-17-remove-the-constant-instance-identity-signature/`,
  the `dGVzdC1zaWduYXR1cmU=` placeholder. `GET
  /latest/dynamic/instance-identity/signature` no longer exists for any
  caller and answers the router's `404 page not found`; `document` stays,
  byte-identical. Verified on VM 399 and from the host against the installed
  `0.2.0+git20261017.4ba058c78e7b`.

A third was found, fixed and archived the same day:
`openspec/changes/archive/2026-10-17-create-devid-under-the-srk-spire-loads/`.
`devid-enroll` created the DevID under go-tpm-tools' `SRKTemplateRSA()`
(256 zero bytes of `unique`), while SPIRE's `tpm_devid` attestor loads it
under the H-1 template (empty `unique`); on a TPM without a matching
persistent SRK the material was unloadable for SPIRE. The client now derives
a transient H-1 parent and never touches `0x81000001`. Verified in the
go-tpm-tools simulator, on the cloud lab (`libtpms 0.9.3`, SPIRE `1.14.0`) and
on VM 399 (`libtpms 0.9.7`), which was re-enrolled with it: its agent ID is now
`…/tpm_devid/7a5c4121…` with `CN=399`, and the material from 2026-09-14
(`CN=vtpm-pilot`, made by a tpm2-tools script, not by `devid-enroll`) is kept
in `/root/spire-agent-backup-20261017.tgz` in the guest. **Compare templates
through go-tpm, never `tpm2_createprimary -u`:** the latter treats 256 zero
bytes like an empty `unique` on both libtpms versions and once hid the defect.
`CreateEK`'s eviction of a non-matching object at `0x81010001`, a non-goal
there, was fixed separately (below). **Before any SPIRE upgrade,**
re-check that the new release's `SRKTemplateHighRSA` is unchanged.

The `internal/proxmox/vmconfig.go` fixes followed the same day:
`openspec/changes/archive/2026-10-17-read-guest-configs-from-this-node/`. The
Proxmox backend reads only `/etc/pve/local/qemu-server` (no scan, no fallback;
a missing link fails the load with that path), stops at the first `[…]`
section so snapshot, `[PENDING]` and `[special:…]` lines contribute neither
keys nor MACs, and never stores `public-ipv4`. `internal/proxmox` has tests
now, on constructed `/etc/pve` trees. On hogan nothing visible changed (it
sorts first and has no snapshots); VM 399 cannot take snapshots, so section
handling was never seen on the live stack.

`CreateEK` followed as a fifth, also archived on 2026-10-17:
`openspec/changes/archive/2026-10-17-stop-evicting-foreign-objects-at-the-ek-handle/`.
The EK is now a transient primary from `client.DefaultEKTemplateRSA()`,
derived twice per run (signing request, then `ActivateCredential`) and flushed
each time, because EK, AK, SRK parent and new DevID together exceed the TPM's
object slots. Nothing at `0x81010001` is read, reused or evicted any more. On
VM 399 the derived EK, the persistent EK and the key in the NV EK certificate
are the same key; the lab showed a foreign object there survives
byte-identical and SPIRE `1.14.0` still attests.

Zone and region followed on 2026-10-18, from the per-VM metadata exploration
(PR #33): `openspec/changes/archive/2026-10-18-serve-zone-and-region-per-vm/`.
It added `placement/region` beside `placement/availability-zone` and resolved
both per caller from `mds.zone.*`/`mds.region.*` tags, then the node description,
then the node and cluster names. **Its per-VM half was replaced the same day**
(next paragraph); what stays is the host fallback and names keeping the case of
their source. Its claim that Proxmox rejects `-` in tags is unproven: the
installed `PVE_TAG_RE` (pve-manager 9.2.3) allows `-`, and the rejection seen
was probably a `;` inside one GUI entry.

VM tags followed on 2026-10-18 as the second archive of the day:
`openspec/changes/archive/2026-10-18-serve-vm-tags-as-instance-metadata/`
(PR #46). A Proxmox tag `vtpm_mds.io.<key>.<value>` (YAML: a `tags` map) is
served under `/latest/meta-data/tags/instance/<key>`; `tags/` is listed only for
a VM that has a tag, and every tags path otherwise answers `404` with an empty
body. Tags never enter the identity document or a certificate (the maintainer's
decision). **Topology is per node, not per VM:** `placement/availability-zone`
and `placement/region` come from `topology.vtpm_mds.io/zone|region` lines in
this node's description, else the node name and the `cluster_name`; two lines of
one kind declare nothing and are logged. `topology.kubernetes.io/*` lines and
the former `mds.zone.*`/`mds.region.*` tags and YAML `zone`/`region` are ignored,
the latter with one log line per load. Verified on VM 399 against the installed
`0.2.0+git20261018.eaed1d1b54b6` (tags, no tags, one and two zone lines, region)
and in the lab for the YAML map. **In the data centre keep both prefixes in a
node description:** the node-label controller and an Ansible labelling role
still read `topology.kubernetes.io/`, and the role fails when those lines are
missing. Whether that consumer moves to this service is the maintainer's
decision and work outside this repository (see the archived design's "Open
question").

The node topology followed as the third archive of 2026-10-18:
`openspec/changes/archive/2026-10-18-serve-node-topology-under-placement/`
(PR #48). Any `topology.vtpm_mds.io/<key>: <value>` line of the node
description is served as `placement/<key>`, including EC2 keys such as
`host-id` (the maintainer's choice: only an `Administrator` can write the
description). `zone` is the short form of `availability-zone`; both set declare
neither, with a log line, and the zone falls back to the node name. `node` and
`cluster` default to the node name and the `cluster_name`. Keys are
case-sensitive; a key with no value answers `404` with an empty body and is not
listed; `placement/` now lists the keys and `placement` redirects with `301`.
Every line under the prefix is readable by every guest of the node. Verified on
VM 399 against `0.2.0+git20261018.3fcf3317b0d1` (six keys, the conflict, the
old prefix ignored) and in the lab for the YAML defaults. `/latest/meta-data`
without a slash answers `301` on hogan (go1.24) but `307` when built with a
newer Go; only `placement` has an explicit redirect.

The first item of the production-readiness list followed as the fourth
archive of 2026-10-18: `openspec/changes/archive/2026-10-18-quiet-the-log-and-record-issuance/`
(PR #49). `[DEBUG` lines are written only with `-debug`; four former debug lines
that report a failure or a reload (`Failed to load VM configs`, `Failed to read
/proc/net/arp`, an unexpected connection type, and the `N VMs, M MAC addresses`
summary) are always written, without the marker. Each request writes one access
line `METHOD "PATH" STATUS DURATION PEER VM`, the path quoted so a guest cannot
forge a second line, the peer from the connection and never from
`X-Forwarded-For`, `-` for an unbound caller. Each issued LDevID writes one line
`devid issued serial=<hex> vm=<id> ek=<sha256>` and nothing else. Verified on
hogan against `0.2.0+git20261018.57cc6cb3c8ac` (journal before and after, a
`-debug` run, one enrolment of VM 399 into a scratch directory with SPIRE's
material untouched) and in the lab.

The unit hardening followed as the fifth archive of 2026-10-18:
`openspec/changes/archive/2026-10-18-harden-the-service-unit/` (PR #51). The
daemon still starts as root but keeps only `CAP_NET_BIND_SERVICE`, so it reads
only what root owns (the CA key is `root:root 0600`, `/etc/pve` is
`root:www-data 0640`); the file system is read-only except `/var/lib/vtpm-mds`,
system calls are `@system-service` minus `@privileged @resources` and fail with
`EPERM`, sockets are `AF_INET`/`AF_INET6`/`AF_UNIX`, and traffic is limited to
`169.254.0.0/16` and loopback. The unit is `PartOf=pve-cluster.service`, so a
pmxcfs restart rebuilds the read-only view of `/etc/pve`. `systemd-analyze
security` went from 9.0 to **1.9** on the installed unit (systemd 257). **A CA
key the daemon cannot read does not stop the start**; it logs `DevID enrollment
disabled` — check for that line after any install. Verified on hogan against
`0.2.0+git20261018.3f5c7fd8613c`, including an enrolment of VM 399 and a
`pve-cluster` restart by the maintainer.

The daemon's own user followed as the sixth archive of 2026-10-18:
`openspec/changes/archive/2026-10-18-run-the-daemon-as-its-own-user/` (PR #55).
It runs as the system user `vtpm-mds` (from `systemd-sysusers`) with the extra
group `www-data`, which reads `/etc/pve` as Proxmox ships it but not
`/etc/pve/priv`; the CA key (`root:root 0600`) is read by systemd and handed over
as the credential `devid_ca.key`, which the daemon prefers to `devid_ca_key`. **A
missing key file now stops the start** (`243/CREDENTIALS`, the maintainer's
D-d), because every node with vTPM guests must be able to enroll. A key elsewhere
needs a drop-in that resets `LoadCredential=` first; a rotated key needs a
restart. `systemd-analyze security` is **1.5 OK**. Verified on hogan against
`0.2.0+git20261018.493063dcb393` (UID 987, an enrolment of VM 399, reload, and
the maintainer's `setpriv` check that `priv/` is refused). Two review follow-ups
wait for the next change to `debian/` or `loadDevIDEnroller` (see that design).
**Every node holds the CA key**, so a compromised node can issue for any VM ID;
an intermediate per node or per cluster is part of the open CA decision.

The connection-map leak followed as the seventh archive of 2026-10-18:
`openspec/changes/archive/2026-10-18-drop-the-connection-map/` (PR #52). The
server kept an entry per accepted connection that nothing read and nothing
removed; the map is gone, and no per-connection state outlives its connection.
A test opens and closes connections and checks nothing is retained, against a
seam for the ARP table (`arpTablePath`). Verified on hogan against
`0.2.0+git20261018.38bd29d4400c`: 70000 bound connections from VM 399, `VmRSS`
10 MB at start, then flat at 22–26 MB with collector drops over the last
45000. The old build was not run under the same load, and only `VmRSS` was
measured, no heap profile.

Throttling followed on 2026-10-19:
`openspec/changes/archive/2026-10-19-throttle-guest-requests/` (PR #50). Limits
are per peer address (`PeerIP`, never `X-Forwarded-For`): 30 token mints,
120 requests and 6 enrolments per 60 s, 16 concurrent connections; over a limit
a request answers `429` `Too many requests` with `Retry-After: 60` and
`Connection: close`, while a 17th connection is closed at accept without an
answer. Only `GET /health` is exempt. Each peer keeps at most 32 live tokens
(the oldest of **that** peer is evicted); a global brake of 65536 tokens answers
`429` with a log line, and only holds against spoofed addresses once callers are
bound to their bridge port. Idle peers are removed from the limiter. There are
no config keys. Verified in the lab and on hogan against
`0.2.0+git20261018.02dadca5c027`: the host flooded tokens, reads and
connections while VM 399 kept getting `200`. Not verified: a bound VM as the
flooder, enrolment under budget on hogan, long-running load. **After `dpkg -i`
the unit stayed stopped** (IMDS down 43 s until a manual start); the next
change fixed that.

Restart on upgrade followed as the third archive of 2026-10-19:
`openspec/changes/archive/2026-10-19-restart-the-service-on-upgrade/` (PR #60).
`dh_installsystemd` had generated a `preinst` that stopped the daemon on
every upgrade (its default), and nothing started it again. Now
`--no-stop-on-upgrade` removes that stop, and `postinst` runs
`deb-systemd-invoke try-restart` after the manager reload, only on an upgrade
(`$2` set), only on a running manager and without `DPKG_ROOT`: a running
daemon gets one restart, a stopped one stays stopped, a first install starts
nothing, and `dpkg-reconfigure` restarts too. A restart systemd **refuses**
(masked or missing unit) warns on stderr and does not fail `dpkg` (the
maintainer's E3); a daemon that exits after it was forked is **not** seen by
`dpkg` (`Type=simple`), only in the journal, where `Restart=on-failure` retries
it. Verified on hogan: upgrade while running, longest `/health` gap 0.257 s;
upgrade while stopped stays `inactive`; first install in a lab guest `enabled`
and `inactive`. hogan runs `0.2.0+git20261019.27d0db6e0a7c`. Not verified: a
refused restart, the `DPKG_ROOT` guard.

Reload on new VMs followed as the fourth archive of 2026-10-19:
`openspec/changes/archive/2026-10-19-reload-when-guest-configs-change/`
(PR #62). The daemon checks its inventory source every **5 s** (a constant,
the maintainer's E5) and reloads through the `SIGHUP` path when a SHA-256 over
the `local` link target and the names and contents of exactly the files the
loader reads has changed; an
unchanged source logs nothing, a failing load logs once per change and keeps
the previous inventory, and `SIGHUP` stays for an immediate reload. **A
`.path` unit cannot do this on a cluster:** inotify on `/etc/pve/nodes/*`
saw no event for writes that arrived from another node (measured on
`lrm_status`), and a migration or HA recovery writes a guest config the same
way. Verified on hogan against
`0.2.0+git20261019.90a2a368116a`: a tag set with `qm set` reached VM 399
within the interval and a write made **on holger** into
`nodes/hogan/qemu-server/399.conf` was picked up, both with the same PID; and
in the lab for the YAML backend (new VM in ~1.5 s, an unknown key gives
`degraded` with the old inventory kept). **Rebase onto current `main` before
every install on hogan.** A build of this change cut from a base without
`restart-the-service-on-upgrade` brought the old `preinst` stop back and took
the IMDS down for 16.6 s; rebased, the longest gap was 0.201 s.

Honest readiness followed as the second archive of 2026-10-19:
`openspec/changes/archive/2026-10-19-report-readiness-honestly/` (PR #61).
`GET /health` still always answers `200`, but `status` is `ok` only when the
inventory is loaded and, with attestation on, the DevID CA cert and key can be
opened **and** enrollment actually loaded; otherwise `degraded`. A caller
bound to no VM (the host) also gets `inventory_loaded`, `vm_count`,
`last_reload`, `last_error` (`inventory_load_failed`, `devid_ca_unreadable`)
and, with attestation on, `devid_ca_readable`; a bound guest gets only
`status` (the record's default; the maintainer did not overrule it). One predicate, `inventory.IsBound`,
decides "bound" for `/health` and the metadata path. The CA check is cached
5 s. **Config and YAML inventory are decoded strictly:** an unknown config key
stops the start; an unknown inventory key fails that load, so the service starts
`degraded` with an empty inventory, or on a reload keeps the previous one, and
`/health` reports it. Before installing on a node, check its
`/etc/vtpm-mds/config.yaml` decodes strictly (hogan's did, as did
`debian/config.yaml`). Verified in the lab (YAML half) and on hogan
(strict decode of the installed config, read-only).

**Pre-review rule (maintainer, 2026-10-18):** before the coordinator calls a PR
ready to merge, an agent on a **stronger** model than the author reviews it
read-only (Sonnet → Opus, Opus → Fable, Cursor/Composer → Fable); findings are
fixed or explicitly deferred by the maintainer.

The Kubernetes consumer will become a small per-node DaemonSet that reads
`placement/` from this service and labels its own node (maintainer, 2026-10-18),
outside this repository and later. Until then the data-centre node-label
controller and an Ansible role read `topology.kubernetes.io/` from the same
description, so keep both prefixes there.

#33 was closed on 2026-10-18 and nothing from it is open. D2 is decided (the
maintainer, 2026-10-18): the Proxmox backend gets no source for `public-ipv4`;
a guest that needs a public address reads it from a VM tag
(`vtpm_mds.io.<key>.<value>`, served under `tags/instance/<key>`), and the key
stays served only from the YAML inventory. D8 is decided (the LDevID CN stays the VM ID, a
readable SPIFFE ID comes from a SPIRE node alias on
`tpm_devid:subject:cn:<vmid>`), and D9 was researched (only `Administrator` has
`Sys.Modify`, which guards the node description; nothing from tags goes into a
certificate). Found on the way: one typo in the YAML inventory empties the
whole inventory while the service reports healthy; it is on the
production-readiness list of 2026-10-18, which the maintainer wants worked off
before production (debug logging, unit hardening, restart on upgrade (done), reload on new VMs (done), issuance
log, health detail (done), the connection-map leak (done), CA operation, throttling (done), and MAC
binding on `vmbr_imds` with an EK pin per VM on Proxmox).

Those first two both carry a `MODIFIED` block on `Refuse An Instance Identity
To Callers With No VM Record`, so the signature change's block **was rebuilt
against the text the first archive left** — its two edits applied
to the new paragraph, not a verbatim copy of the old one, which would silently
have reverted the `public-ipv4` correction. No test and no validator catches
that loss; a trial `openspec archive` on a scratch copy of `openspec/` showed
both survive.

Everything else specified has shipped and been verified.

The split that got there is worth keeping: the cloud lab agents verified what
needs a lab, this host verified what needs the Proxmox stack, and three tasks
in `fail-loudly-when-the-lab-is-unprepared` were deliberately recorded as half
done on each side until both halves existed — the site-CA layout in
`/etc/swtpm_setup.conf` and `/usr/share/pve-edk2-firmware/` exist only here,
the working lab only there.

Installing the host package now reloads the systemd manager, so an installed
unit is the unit systemd runs, and the documented `dpkg -i` names one package
instead of globbing.

The agent user **can** build a `.deb`. An earlier version of this file said it
could not, because `dpkg-buildpackage` needs `dpkg --print-architecture`; both
the conclusion and the cause were wrong. `/usr/bin/dpkg --print-architecture`
answers `amd64` for this user and always did. What refused it was `~/bin/dpkg`,
a PATH wrapper that forwarded only `dpkg -i` and rejected everything else — so
packaging was never split between a source half here and a build half for the
maintainer, it only looked that way. Since 2026-10-16 that wrapper passes
read-only queries through as well, which also restores the `dpkg -l vtpm-mds`
this file asks for as evidence. Options that change state, other than `-i`, are
still refused; the real boundary is the sudoers entry, which grants nothing but
`/usr/local/sbin/hogan-lab`.

The second half of the blocker was `.deb-build/` left root-owned by a
`make deb-local` run as root. Its Go module cache has `0555` root-owned
directories, so the Makefile's own `chmod -R u+w` cannot help — the agent is not
the owner — and the `rm -rf` before it fails. That directory has been removed.
**Do not run `make deb-local` as root inside the checkout**; if it must be root,
pass a `DEB_STAGE` outside the checkout and `chown` the result back to
`coding-agent`.

`scripts/deb-local.sh` stamps the version `.dirty` whenever
`git status --porcelain` is non-empty, and untracked files count — an untracked
`.envrc` alone is enough. Move such a file aside for the build if the version
has to identify the tree exactly; the version string is fixed when the script
starts, so putting it back mid-build is safe.

Three defects were found and reproduced here. All three are now implemented
and archived; they live in `imds/handlers.go` and in `scripts/qemu-lab/`, so they
stay a disjoint set from product work elsewhere:

- `GET /latest/meta-data/public-ipv4` always answers `404` because
  `getPublicIP` is an unimplemented `TODO`. Where the address should come from
  is an open question, not just an implementation.
  → **implemented and archived** 2026-10-17:
  `openspec/changes/archive/2026-10-17-serve-a-public-address-or-omit-the-key/`.
  The key is listed and served only when the bound VM record declares an
  address — `public_ipv4` in the YAML inventory. The Proxmox backend has no
  supported source: Proxmox itself never writes such a key, and a hand-written
  `public-ipv4:` line is dropped by the next `qm set`, and since
  `read-guest-configs-from-this-node` the Proxmox backend ignores it anyway. The YAML half was verified on
  the cloud lab, the Proxmox half on VM 399 against the installed package.
  Where a declared address should come from on Proxmox was decided on
  2026-10-18 when #33 closed: nowhere; a VM tag carries it when needed.
- `imds.getClientIP` falls back to the literal `"127.0.0.1"` when `PeerIP`
  cannot parse `RemoteAddr`, so with `mds.require_vm_identity: false` an
  unparseable peer is served the invented id `i-127-0-0-1`.
  → **fixed and archived** 2026-10-16, verified against the installed
  git-stamped package on the real Proxmox stack:
  `openspec/changes/archive/2026-10-16-fix-client-ip-fallback/`. The
  `127.0.0.1` opt-out path is covered by unit tests only — the daemon listens
  on `169.254.169.1:80` alone, so no connection here ever arrives with that
  peer.
- Reusing one guest vTPM across `scripts/qemu-lab/e2e-devid-guest.sh` runs used
  to end in TPM dictionary-attack lockout: a run on a reused vTPM failed with
  `Certify: ... DA lockout mode`, and after a single successful run the counter
  already read 1 of 3. Found by the cloud lab agent on 2026-10-03 while
  verifying `fail-loudly-when-the-lab-is-unprepared`, and not caused by it.
  → **fixed and archived** 2026-10-17:
  `openspec/changes/archive/2026-10-17-stop-lab-guest-without-burning-da-counter/`.
  **The cause was the teardown, not the enrollment.** `stop-guest.sh` hard-killed
  QEMU while the guest swtpm was still attached, and the TPM counted the
  interrupted session as an authorization failure. It now issues QMP
  `system_powerdown`, waits for QEMU to exit, and only then stops swtpm.
  Established from both sides rather than argued: on the cloud lab a hard kill
  moves the counter `0 → 1` while a graceful stop leaves it at `0`, and two
  full e2e cycles on the **same** vTPM both pass with the counter still `0`;
  on the Proxmox stack a successful enrollment against the installed package
  burned no counter at all, `0x0` before and after. The enrollment path was
  never implicated — which is why the fix lives in `scripts/qemu-lab/` and not
  in `internal/devid`, where this entry used to point.

Ordered by severity. The first four are recorded as current behaviour in
`openspec/specs/`, so read the requirement before implementing the change that
replaces it. All six numbered items are closed; item 6 closed as a removal
rather than an implementation, because the answer turned out to be "remove".

Apply order matters in two places, and each change says so in its own proposal:
`refuse-unbound-metadata-callers` comes after both
`parse-peer-address-correctly` (they modify the same requirement) and
`fail-fast-on-unusable-configuration` (its secure default needs the defaults
merge). Everything else is independent.

1. **The EK certificate is not bound to the endorsement key used for credential
   activation** (`internal/devid/verify.go`, `_ = pub`). An EK certificate minted
   over an unrelated key, chaining to `ek_ca_chain`, enrolls successfully — verified
   against a software TPM. The EK factor proves possession of a public certificate,
   not of the certified TPM, and `ek_sha256` pinning inherits the same weakness.
   See `openspec/specs/devid-enrollment/spec.md`, "Trust The EK Certificate".
   → **fixed and archived** 2026-10-03, verified against a software TPM and
   a guest end-to-end run on the cloud lab host:
   `openspec/changes/archive/2026-10-03-bind-ek-certificate-to-endorsement-key/`
2. **The LDevID subject is guest-controlled** (`internal/devid/enroll.go`,
   `subjectCNFromRequest`): the CSR's CN wins over the authenticated VM id.
   → **fixed and archived** 2026-10-03, same evidence:
   `openspec/changes/archive/2026-10-03-enforce-authenticated-ldevid-subject/`
3. **Unbound callers are served an identity instead of refused**
   (`imds/handlers.go`, `getInstanceID` fallback): `i-<caller-ip>`, with
   `X-Forwarded-For` taking precedence, so the caller picks its own id. It also
   lands in the instance identity document. `/latest/identity`
   (`identity/handlers.go`) ignores inventory entirely.
   → **fixed and archived** 2026-10-03, verified against VM 399 and an
   unbound host caller on the real Proxmox stack:
   `openspec/changes/archive/2026-10-03-refuse-unbound-metadata-callers/`.
   Adds `mds.require_vm_identity` (default true). It also added a
   `workload-identity` capability for `/latest/identity`; item 6 retired both
   on 2026-10-16, so `identity/handlers.go` and that capability no longer
   exist. `mds.require_vm_identity` stays.
4. **A clean stop leaves the unit failed**: `main.go` calls `log.Fatalf` on the
   error from `srv.Start()`, including `http.ErrServerClosed`, so every
   `systemctl stop`/`restart` exits 1 and ends in `failed`. Verified on 0.2.0.
   This belongs to the `operability` capability.
   → **fixed and archived** 2026-10-02, verified on the real unit on `hogan`:
   `openspec/changes/archive/2026-10-02-exit-cleanly-on-server-shutdown/`
5. Smaller, from writing the specs: `token_ttl` parse errors are discarded (a typo
   yields a zero TTL, so every metadata read 401s); `config.Load` never merges
   defaults, so omitting `enable_ec2_compat` silently disables the metadata tree
   while `/health` still reports ok; `local-ipv4` splits `RemoteAddr` on every
   `:` and returns `[fe80` for an IPv6 peer.
   → split in two: `openspec/changes/fail-fast-on-unusable-configuration/`
   (the two config defects) and `openspec/changes/parse-peer-address-correctly/`
   (the address defect).
6. **Probably remove rather than implement: the attest and JWT identity path.**
   Deferred 2026-10-03 — not relevant to the two use cases that matter (SPIRE
   via `tpm_devid`, Teleport via TPM join). Written up here so the analysis is
   not repeated. **Written, implemented and archived on 2026-10-16** as the
   removal this analysis argued for:
   `openspec/changes/archive/2026-10-16-remove-the-unverifiable-identity-endpoints/`.
   The `workload-identity` capability was retired with it, so
   `openspec/specs/` no longer carries it.

   This was queued as "implement TPM quote verification and real signing keys
   for the identity JWTs". Looking at it with the maintainer, that is the wrong
   framing. The reasons, in the order they decide the question:

   - **Nothing consumes `/latest/attest` or `/latest/identity`.** No client, no
     lab script, no guest component; they appear only in `ARCHITECTURE.md`, in
     the route registration, and as a refused path in a test. SPIRE reads the
     DevID files from disk and does its own credential activation against the
     `.priv`/`.pub` blobs.
   - **The JWKS cannot reach an external verifier.** The service listens on
     `169.254.169.1:80`, reachable from guests on the IMDS bridge and from the
     host. A SPIRE server, Vault or Teleport outside cannot fetch
     `/.well-known/jwks.json` at all. Real signing keys do not fix that; it is
     the design, not the implementation.
   - **The shape is borrowed from the wrong cloud.** `/latest/` is AWS, and
     having that namespace made it cheap to hang more under it — but AWS has no
     `/latest/identity` and no JWKS. A signed instance identity token verified
     against the provider's public JWKS is the GCP and Azure pattern, which
     works because Google's JWKS is on the public internet. The pattern was
     copied without the part that carries it.
   - **`ARCHITECTURE.md` already contradicts itself.** Its goals call the JWT
     "consumable by SPIRE, Teleport, and Vault", while its own integration
     table says Teleport integrates by *restricting to the EK CA* and SPIRE by
     *DevID PEM + TPM2B blobs*. Those two rows are the real use cases, and
     neither needs the JWT. The three rows that do — SPIRE JWT path, Vault
     `auth/jwt`, Kubernetes node labels — all hang on the unreachable JWKS.
   - **Both endpoints currently affirm without checking.** `POST /latest/attest`
     answers `Valid: true` to any quote once the nonce matches, and
     `/latest/identity` signs with an HS256 secret compiled into the binary.
     An endpoint that confirms without verifying is worse than no endpoint.

   Proposed direction when it is picked up: remove `/latest/attest/nonce`,
   `/latest/attest`, `/latest/identity` and `/.well-known/jwks.json`; archive
   the `workload-identity` capability with `REMOVED` requirements and a
   migration note; and correct `ARCHITECTURE.md` — the three JWKS integration
   rows and roadmap phases 3 and 6. The `workload-identity` spec written on
   2026-10-03 is not wasted work: writing the placeholder behaviour down is
   what made the conclusion visible.

   Defer the PCR-policy question until something consumes a policy. Quote
   verification without a policy consumer is its own purpose, which is no
   purpose.

   **Open sub-question, decides the scope:** does
   `/latest/dynamic/instance-identity/{document,signature}` stay? Those are
   genuine AWS paths that cloud-init tooling may expect, but `signature`
   returns the constant `dGVzdC1zaWduYXR1cmU=` to every caller — the same
   defect as `/latest/attest`, in an endpoint there may be a compatibility
   reason to keep.
   → **answered and archived** 2026-10-17: `document` stays, `signature` was
   removed, because cloud-init reaches leaves only through the listing and
   never requests `signature` by name:
   `openspec/changes/archive/2026-10-17-remove-the-constant-instance-identity-signature/`.

   Before writing any of this, check Teleport's TPM join against its current
   documentation rather than from memory, to confirm what `vtpm-mds` would have
   to supply at join time — the expectation is nothing, since the node talks to
   its own TPM and to the auth server.

The QEMU lab under `scripts/qemu-lab/` **must not be run on `hogan`**. Its
`setup-host.sh` creates `br-imds` and claims 169.254.169.1/16 and
169.254.169.254/16, which this host already owns as /32 on the production
`vmbr_imds`. That yields two connected routes for 169.254.0.0/16, and the
reply to a lab guest leaves through the production bridge — the guest sees
nothing but `curl: (28) Connection timed out`. Production kept winning the
route lookup during the attempt on 2026-10-02 and `br-imds` is runtime-only,
so nothing persisted, but which bridge wins is decided by route insertion
order rather than by design. Run the QEMU lab on the cloud lab host. The same
goes for `e2e-netns.sh`: it expects `i-100` for the lab YAML inventory, while
`hogan` resolves callers from `/etc/pve`.

Eight tooling defects surfaced while implementing and verifying the above. They
were written up as three changes, and **all three are implemented and
archived** (2026-10-03):

- `make-host-package-install-deterministic` — `README.md` documents an
  ambiguous `dpkg -i dist/vtpm-mds_*.deb`, and the host package's `postinst`
  never reloads systemd, so an installed unit is not the unit systemd runs.
- `fail-loudly-when-the-lab-is-unprepared` — `gen-lab-pki.sh` ignores
  `/etc/swtpm_setup.conf` and reports success without an EK chain, no script
  creates the swtpm command socket `TestEnrollAgainstSwtpm` opens, and
  `e2e-devid-guest.sh` checks none of its tools.
- `confine-the-qemu-lab-to-its-own-host` — the lab collides with a production
  IMDS, `cloud-install.sh` can disturb `pve-qemu-kvm`, and `MDS_LAB_ACCEL`
  defaults to `tcg` even where KVM works.

Two further defects were pre-existing and unspecified when this list was
written; both now have changes, both archived (see the three defects above):

- `GET /latest/meta-data/public-ipv4` always answered `404` because
  `getPublicIP` in `imds/handlers.go` was an unimplemented `TODO`. The
  `instance-metadata` spec recorded that as current behaviour, so this was a
  missing feature rather than a contradiction — but it meant the key was listed
  in the index and never had a value.
- `imds.getClientIP` fell back to the literal `"127.0.0.1"` when `PeerIP`
  could not parse `RemoteAddr`. With `mds.require_vm_identity: false`, an
  unparseable peer address was therefore served the instance id `i-127-0-0-1`
  — a made-up identity, in the one mode where unbound callers are served at
  all.

Two things used to need the maintainer and no longer do. A root session widened
`/usr/local/sbin/hogan-lab` on 2026-10-16:

- `dpkg-install` accepts `dist/{vtpm-mds,devid-enroll}_*.deb`, so the guest
  package installs through the wrapper too. It also refuses more than one
  `.deb` instead of silently installing the last match of an expanded glob —
  the same ambiguity `make-host-package-install-deterministic` removed from the
  repository.
- `hogan-lab qm fresh-tpm 399` replaces VM 399's vTPM with an empty one. It
  requires the VM to be stopped, and it detaches the old state volume rather
  than deleting it, so that volume stays as `unusedN` and a wrong call is
  recoverable. Clearing it out is a human decision. Item 6 task 4.2 needs a vTPM
  that has never enrolled: on a reused one the dictionary-attack lockout trips
  and the failure reads as a regression in the daemon.

Know what `dpkg-install` is: a path allowlist, not a content one. `dist/` is
writable by `coding-agent` and `dpkg -i` runs maintainer scripts as root, so
anything placed there under a matching name is root code. That has always been
true of this wrapper, and it is accepted for this host, where the agent user
builds the package anyway — a documented property rather than an oversight.

`openspec init` has been run — see
`openspec/changes/initialize-openspec-agent-tooling/`. It needed no root: the
checkout root grants `coding-agent` write access through an ACL. The generated
files under `.cursor/` and `.claude/` own only the command and skill
definitions; the rules in this file and in
`.cursor/rules/openspec-workflow.mdc` keep precedence and were not touched.

## Lab host facts

The Proxmox lab host is `hogan.bgl.dembach.org`; the checkout is
`/usr/local/src/vtpm-mds`. The daemon under test must come from a git-stamped
package (`make deb-local`, then `dpkg -i --force-confold`), never from
`make build`, so `dpkg -l vtpm-mds` and `vtpm-mds -version` identify exactly
which tree is running. The agent user has no general sudo: its single
`NOPASSWD` entry is `/usr/local/sbin/hogan-lab`, whose allowlist is the whole
of what it may do as root. `~/bin/` holds thin forwarders — `qm`, `systemctl`,
`nft`, `dpkg-lab-install` — so those names on `PATH` are the wrapper, not the
real tool. Reading the journal needs no sudo: the user is in
`systemd-journal`.

The OpenSpec CLI is installed here — `/usr/local/bin/openspec` with node in
`/usr/bin/node` — so `openspec validate --all --strict` runs on this host. An
earlier version of this file claimed the lab hosts had neither, and that the
CLI therefore could not be installed; that was wrong, and it sent agents to the
fallback script first.
