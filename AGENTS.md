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

### Open changes as of 2026-10-17

None. Three were archived on 2026-10-17; the first two in this order:

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
Left for a separate change, by that record's non-goals: `CreateEK` still evicts
a non-matching persistent object at `0x81010001`. **Before any SPIRE upgrade,**
re-check that the new release's `SRKTemplateHighRSA` is unchanged.

Deferred by the maintainer until the above was closed, and needing a record
and a go before any code: per-VM metadata (`declare-per-vm-metadata`, PR #33;
its D8 is decided: the LDevID CN stays the VM ID and a readable SPIFFE ID comes
from a SPIRE node alias on `tpm_devid:subject:cn:<vmid>`, shown end to end in
the lab; host-side names as extra subject attributes wait for a SPIRE release
with `subject:oid` selectors)
and the `internal/proxmox/vmconfig.go` fixes — read this node via
`/etc/pve/local` rather than the first node alphabetically, section awareness
so snapshot sections do not contribute MACs, and ignoring a hand-written
`public-ipv4:` line.

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
  `public-ipv4:` line, which `internal/proxmox` would still read, is dropped by
  the next `qm set`. Ignoring it is part of the pending `vmconfig.go` change. The YAML half was verified on
  the cloud lab, the Proxmox half on VM 399 against the installed package.
  Where a declared address should come from on Proxmox is deferred, with the
  wider per-VM metadata question, to PR #33.
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
