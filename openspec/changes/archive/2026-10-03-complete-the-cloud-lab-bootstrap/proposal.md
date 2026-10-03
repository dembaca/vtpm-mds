# Proposal

## Why

`scripts/qemu-lab/` was written for the Cursor cloud agent, where a prepared
base image and a login shell make its unstated assumptions true. Run on the
Claude cloud environment on 2026-10-03 — Ubuntu 24.04, root, no `/dev/kvm`,
QEMU absent — `cloud-install.sh` and the scripts after it needed four manual
interventions before `e2e-netns.sh` and `e2e-devid-guest.sh` passed. Both then
passed, so the lab is usable there; the interventions are the defect.

**`$USER` is assumed to be set.** `setup-host.sh` (`usermod`, two `chown`s) and
`start-guest.sh` (`ip tuntap ... user "$USER"`) read it under `set -u`. In a
non-login shell it is unset, and `setup-host.sh` aborts with
`USER: unbound variable`. `cloud-install.sh` already uses `$(id -un)` for the
same purpose, so the repository disagrees with itself.

**`cloud-install.sh` installs too little, and cannot notice.** It guards the
whole `apt-get install` on `qemu-system-x86_64` and `cloud-localds` being
absent, so a host that has those two never gets anything else. The package
list also omits three tools the scripts call:

| Tool | Package | First needed by |
|---|---|---|
| `ping` | `iputils-ping` | `e2e-netns.sh` (and `e2e-imds.sh`, in the guest) |
| `swtpm_setup`, `swtpm_localca`, `certtool` | `swtpm-tools`, `gnutls-bin` | `setup-guest-tpm.sh` |
| `ssh-keygen`, `ssh` | `openssh-client` | `create-guest.sh`, `e2e-imds.sh` |

Each surfaced as `command not found`, in a different script, minutes apart.
On the Cursor image all of them happen to be present, which is why this was
never seen.

## What Changes

- `setup-host.sh` and `start-guest.sh` default `USER` to `id -un` when it is
  unset. When it is set, nothing changes.
- `cloud-install.sh` decides whether to install by checking **every** tool the
  lab scripts call, not two, and installs the package set that provides them,
  extended by `iputils-ping`, `swtpm`, `swtpm-tools`, `gnutls-bin`,
  `openssh-client`. A host that has everything installs nothing, exactly as
  today.
- `scripts/qemu-lab/README.md` states that the scripts support both the Cursor
  and the Claude cloud environment, and what each assumes.
- No change to the daemon, the client, the packaging, any capability, or the
  behaviour of any lab script on an environment where it already worked.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This changes lab tooling under `scripts/qemu-lab/` and its
documentation; no behaviour of the service changes, so `.openspec.yaml` sets
`skip_specs: true`.

## Impact

- `scripts/qemu-lab/setup-host.sh`, `start-guest.sh`, `cloud-install.sh`,
  `README.md`.
- Cursor cloud: no observable change. Claude cloud: the lab bootstraps from a
  bare image with `cloud-install.sh` and `cloud-start.sh` alone.
- Independent of `fail-loudly-when-the-lab-is-unprepared` and
  `confine-the-qemu-lab-to-its-own-host`; they edit other lines of the same
  files, and `confine-the-qemu-lab-to-its-own-host` adds a refusal at the top
  of `cloud-install.sh`'s install branch, which this change keeps intact.
