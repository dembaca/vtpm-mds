# Agent instructions for vtpm-mds

Read this before editing anything. This repository is spec-driven with
[OpenSpec](https://openspec.dev): `openspec/specs/` is the living description of
current behaviour, `openspec/changes/` holds in-flight work. Code that changes
behaviour without a change record is the thing this project most wants to avoid.

This file holds rules, not history. What was done and why is in git and in
`openspec/changes/archive/`. Details of any particular operator's lab (host
names, VM IDs, sudo wrappers, CA names) are kept outside this public
repository by that operator; do not add them here, nor anywhere else in the
product (`operability`: "Name No Particular Operator's Site In The Product").

## Always start here

1. `ls openspec/changes/` and read every document of the change you are about to
   work on — `proposal.md`, `design.md`, `tasks.md` and each
   `specs/<capability>/spec.md` delta. The delta is the contract; `tasks.md` is
   the order of work and each task names its own verification.
2. Read `openspec/specs/` for the capabilities you touch.
3. Work task by task. Tick a checkbox only after running the verification that
   the task states, and say what the output was.
4. Validate before opening a PR with `openspec validate --all --strict`. On a
   host without node, fall back to `scripts/openspec-validate.py --strict`,
   which covers the same structure but is a floor rather than a replacement.
   CLI versions differ in what they warn about; say which version you ran.
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

## Spec deltas

- A `MODIFIED` block replaces the whole requirement. Build it from the text
  currently in `openspec/specs/`, not from an older copy: when two changes
  modify the same requirement, the second must be rebuilt against what the
  first archive left, or it silently reverts the first. No validator catches
  that loss; a trial `openspec archive` on a scratch copy of `openspec/` does.
- Keep requirement text short and put examples and edge cases into scenarios.

## Never leave work uncommitted

- Commit or stash before switching branches, creating worktrees, or running
  anything that restores files.
- `make deb` and `make deb-clean` delete `.deb-build/`; never treat a staging
  copy as a backup.
- Before any destructive step, check `git status --porcelain` and copy anything
  untracked somewhere durable.

## Verify on real artifacts

For packaging and daemon work, inspect what is actually produced, not just the
source: `dpkg-deb -c` / `-I` on each `.deb`, `lintian --fail-on error,warning`,
`systemd-analyze verify` on the unit, and run the extracted binary. Where a
check cannot run, say so explicitly instead of implying it passed.

Tests and vet must pass: `go vet ./...` and `go test ./...`.

- **Test the installed package, not `make build`.** Build with
  `make deb-local`, install with `dpkg -i --force-confold`, so `dpkg -l
  vtpm-mds` and `vtpm-mds -version` identify exactly which tree runs.
- **Rebase onto current `main` before every test install.** A build cut from
  an older base brings back older maintainer scripts (once: a `preinst` that
  stopped the daemon on upgrade).
- `scripts/deb-local.sh` stamps the version `.dirty` whenever `git status
  --porcelain` is non-empty, untracked files included. Move such a file aside
  if the version has to identify the tree exactly.
- Do not run `make deb-local` as root inside the checkout: it leaves a
  root-owned Go module cache in `.deb-build/` that a normal user cannot
  remove. If it must be root, pass a `DEB_STAGE` outside the checkout.
- After an install, check the journal for `DevID enrollment disabled` (a CA key
  the daemon cannot use) and that `/etc/vtpm-mds/config.yaml` still decodes
  strictly (an unknown key stops the start).
- Test against a real guest on a Proxmox host where you can: real `/etc/pve`,
  real ARP binding, a real vTPM. Enrol into a scratch directory, never over a
  running SPIRE agent's material. Arguments to `qm guest exec` must be ASCII:
  a non-ASCII byte truncates the request and hangs the guest agent channel.
- **The QEMU lab under `scripts/qemu-lab/` must not run on a host that serves
  a production IMDS.** Its `setup-host.sh` claims `169.254.169.1/16` and
  `169.254.169.254/16`; on such a host there are two connected routes for
  `169.254.0.0/16` and replies leave through the production bridge.
  `scripts/qemu-lab/e2e-netns.sh` likewise expects the lab's YAML inventory
  (`i-100`) and cannot run against a Proxmox-backed inventory.
- Redirect codes depend on the Go version: `/latest/meta-data` without a slash
  answers `301` with go1.24 and `307` with newer Go; only `placement` has an
  explicit redirect. Do not pin a test or spec to the implicit one.

## TPM and SPIRE pitfalls

- Compare TPM key templates through go-tpm, never with
  `tpm2_createprimary -u`: the latter treats 256 zero bytes of `unique` like
  an empty one and once hid a template mismatch.
- Before any SPIRE upgrade, re-check that the release's `SRKTemplateHighRSA`
  is unchanged; `devid-enroll` derives the DevID's parent from it.
- SPIRE checks the LDevID only at node attestation, and a renewal changes the
  agent ID. Registrations select the node alias on
  `tpm_devid:subject:cn:<vmid>` and `tpm_devid:ca:fingerprint:<sha1 of the
  issuing intermediate>`, never an agent ID.
- `openssl verify` rejects the LDevID leaves while parsing the TCG extensions;
  Go's `x509.Verify` and SPIRE accept them.

## Review before merge

Before the coordinator calls a PR ready to merge, an agent on a **stronger**
model than the author reviews it read-only (Sonnet → Opus, Opus → Fable,
Cursor/Composer → Fable); findings are fixed or explicitly deferred by the
maintainer.

## Coordinating subagents

- Give each subagent a disjoint, explicit file list; overlapping edits to
  `debian/` or `README.md` are the usual source of conflicts.
- Subagents share the parent's shell session. Do not mutate `PATH`,
  `PERL5LIB` or other environment variables in it.
- Tell each subagent to commit on its own branch and not to push, so the
  coordinator merges and verifies the combined tree.
- Use a git worktree per subagent when they run in parallel.

## Coordinating with cloud agents

Cloud agents branch from `main` rather than from whatever a local session has
in flight.

- **Push `main` before handing work to a cloud agent**, and have it branch from
  the pushed commit.
- **Split by file, and say so in the briefing.** `scripts/qemu-lab/` is the
  cloud lab's, `debian/` and the root `README.md` belong to whoever is doing
  packaging, and product code under `imds/`, `internal/` is a third set.
- **One hand on `AGENTS.md` and the root `README.md`.**
- **Archiving stays with the coordinator**, so the README status table and
  `openspec/specs/` move together.
- **Each side verifies what only it can**: the cloud lab what needs the QEMU
  lab, a Proxmox host what needs the Proxmox stack and an installed package.
  Tell each to record what it could **not** verify in its environment.
