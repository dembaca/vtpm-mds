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

## Known gaps, and the queue as of 2026-09-20

Items 1 to 5 below are written up as changes and **implemented on `main`**.
Three of them are **not yet archived**, because each still has tasks that need
root on the lab host or a software TPM, and archiving a change whose
verification never ran would put an unchecked claim into the living spec:

| Change | Open tasks | What is missing |
|---|---|---|
| `bind-ek-certificate-to-endorsement-key` | 4.2, 4.3, 5.1 | `TestEnrollAgainstSwtpm` **skipped**, no swtpm socket; guest e2e needs root; 5.1 is the archive-time Purpose edit |
| `enforce-authenticated-ldevid-subject` | 4.2, 4.3 | same swtpm skip and guest e2e |
| `refuse-unbound-metadata-callers` | 6.2, 6.3 | the netns runs, which need root and a git-stamped package |

Run those, tick the boxes with the output, then `openspec archive <change>`.
Until then `openspec validate --all --strict` passes but
`scripts/openspec-validate.py --strict` correctly reports the unticked tasks.

Item 6 is still unwritten. Write a proposal, design, tasks and spec delta for it
before touching code — do not fix it inline.

Ordered by severity. The first four are recorded as current behaviour in
`openspec/specs/`, so read the requirement before implementing the change that
replaces it.

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
   → `openspec/changes/bind-ek-certificate-to-endorsement-key/`
2. **The LDevID subject is guest-controlled** (`internal/devid/enroll.go`,
   `subjectCNFromRequest`): the CSR's CN wins over the authenticated VM id.
   → `openspec/changes/enforce-authenticated-ldevid-subject/`
3. **Unbound callers are served an identity instead of refused**
   (`imds/handlers.go`, `getInstanceID` fallback): `i-<caller-ip>`, with
   `X-Forwarded-For` taking precedence, so the caller picks its own id. It also
   lands in the instance identity document. `/latest/identity`
   (`identity/handlers.go`) ignores inventory entirely.
   → `openspec/changes/refuse-unbound-metadata-callers/`, which adds
   `mds.require_vm_identity` (default true) and a new `workload-identity`
   capability for `/latest/identity`.
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
6. Long-standing and unspecified: TPM quote verification, and real signing keys
   for the identity JWTs. Still unwritten. The `workload-identity` capability
   added by `refuse-unbound-metadata-callers` records the placeholder HS256
   secret, the placeholder JWKS and the ignored `mds.jwt_ttl` as current
   behaviour, so this can now be proposed against a written contract.

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

Eight tooling defects surfaced while implementing and verifying the above, and
are now written up as three changes — none of them implemented:

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

Two further defects are pre-existing and unspecified, and still need a change
each before anyone touches them:

- `GET /latest/meta-data/public-ipv4` always answers `404` because
  `getPublicIP` in `imds/handlers.go` is an unimplemented `TODO`. The
  `instance-metadata` spec records that as current behaviour, so this is a
  missing feature rather than a contradiction — but it means the key is listed
  in the index and never has a value.
- `imds.getClientIP` falls back to the literal `"127.0.0.1"` when `PeerIP`
  cannot parse `RemoteAddr`. With `mds.require_vm_identity: false`, an
  unparseable peer address is therefore served the instance id `i-127-0-0-1`
  — a made-up identity, in the one mode where unbound callers are served at
  all.

One thing needs root on the lab host, so it needs the maintainer:

- `/usr/local/sbin/hogan-lab` whitelists only `dist/vtpm-mds_*.deb` for
  `dpkg-install`, so the guest package cannot be installed through the wrapper.

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
which tree is running. There is no passwordless sudo for the agent user.

The OpenSpec CLI is installed here — `/usr/local/bin/openspec` with node in
`/usr/bin/node` — so `openspec validate --all --strict` runs on this host. An
earlier version of this file claimed the lab hosts had neither, and that the
CLI therefore could not be installed; that was wrong, and it sent agents to the
fallback script first.
