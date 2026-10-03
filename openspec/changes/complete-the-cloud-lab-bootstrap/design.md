# Design

## Context

See proposal.md — Why.

The two cloud environments differ in what they provide, not in what the lab
needs. Cursor ships a prepared image with the tools and a shell with `USER`
set; Claude's starts from a bare Ubuntu 24.04 with root and a non-login shell.
The scripts' needs are the same, so the fix is to make the bootstrap
state those needs completely, rather than to keep one script per environment.

## Goals / Non-Goals

**Goals:**

- One set of scripts that works unchanged on both cloud environments.
- On an environment where the lab already worked, behaviour is byte-for-byte
  the same: no extra apt run, no changed ownership, no new output.
- A bare Ubuntu 24.04 environment reaches a passing `e2e-netns.sh` through
  `cloud-install.sh` and `cloud-start.sh` alone.

**Non-Goals:**

- Splitting the scripts per environment. Decided against below.
- Making the scripts fail early with a precondition gate — that is
  `fail-loudly-when-the-lab-is-unprepared`, which deliberately does not install.
  This change is the installing side of the same gap: `cloud-install.sh` is the
  one script whose job is to install.
- Anything about `/dev/kvm`: TCG stays the behaviour when it is absent, and
  default selection is `confine-the-qemu-lab-to-its-own-host`.
- Provisioning on a non-Debian host. `cloud-install.sh` is apt-based today and
  stays so.

## Decisions

### One script set, not a Cursor/Claude split

Every difference found is a missing assumption, not a conflicting one. A
superset package list and a defaulted variable satisfy both environments, and a
split would duplicate 851 lines of scripts to carry two copies of the same lab
logic, which then drift. Revisit only if the environments need *conflicting*
behaviour — for example different network setup — which none showed.

### Default `USER`, do not replace it

`USER="${USER:-$(id -un)}"` at the top of the two scripts. Where `USER` is set
the expansion yields the same value, so the Cursor behaviour is untouched;
where it is not, it yields what `cloud-install.sh` already computes. Replacing
every `$USER` with `$(id -un)` was rejected: it changes the user when a caller
deliberately exports `USER` (for example under `sudo -E`), which is a
behaviour change for no gain.

The `chown "$USER:$USER"` group is left as it is. It assumes a group named like
the user, which holds on both environments (`ubuntu:ubuntu`, `root:root`);
changing it to `id -gn` is a separate behaviour change.

### Guard on the tools, not on two of them

`cloud-install.sh` computes the list of missing commands from the full set the
scripts invoke and runs `apt-get` when that list is non-empty. This keeps the
property that an already-provisioned host is untouched, and removes the one
that made the old guard unsafe: it could be satisfied by two tools while five
others were missing. The set is `qemu-system-x86_64 qemu-img cloud-localds
genisoimage swtpm swtpm_setup swtpm_localca certtool ip iptables ping
ssh-keygen curl jq openssl python3`.

The package list is a superset of the old one, so a host that took the
install branch before still gets everything it got.

## Risks / Trade-offs

- **[Risk] The wider guard installs on a Cursor host that previously skipped
  apt** → Only if a tool is genuinely missing there, in which case a later
  script would have failed anyway. A fully provisioned host still skips.
- **[Risk] The tool list drifts from what the scripts call** → Same trade-off
  `fail-loudly-when-the-lab-is-unprepared` accepts for its gate; a task checks
  the list against the scripts, and a miss costs a late `command not found`,
  which is today's behaviour.
- **[Trade-off] `genisoimage` and `cloud-localds` are both listed** → Kept
  because the existing install already pulls both; dropping either is outside
  this change.

## Migration Plan

None. Development tooling; nothing is deployed. Rerunning `cloud-install.sh`
on an existing environment installs only what is missing.

## Open Questions

None.
