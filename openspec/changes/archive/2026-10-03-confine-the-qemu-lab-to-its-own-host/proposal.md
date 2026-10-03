# Proposal

## Why

`scripts/qemu-lab/` builds a self-contained QEMU lab: its own bridge, its own
link-local addresses, its own DNAT, its own guest. It was written for the
cloud agent box, and it says so nowhere. Run on the Proxmox host it collides
with the metadata service it is supposed to be testing.

Nobody should run it there, and nothing needs to: on Proxmox the service is
tested against the pilot VM 399 and the installed package, under real
conditions, which is strictly the better test. The reason it happens anyway is
that the checkout lives at `/usr/local/src/vtpm-mds` **on** that host, so
whoever works in the repository reaches for `scripts/qemu-lab/` without
leaving it. That is how it was run on 2026-10-02.

On `hogan`, `setup-host.sh` creates `br-imds` and adds `169.254.169.1/16` and
`169.254.169.254/16`. The host already owns both as `/32` on `vmbr_imds`, the
production IMDS bridge declared in `/etc/network/interfaces`. The result is
two connected routes for the same prefix:

    169.254.0.0/16 dev vmbr_imds scope link
    169.254.0.0/16 dev br-imds proto kernel scope link src 169.254.169.1

`ip route get 169.254.169.10` resolved through `vmbr_imds`. A lab guest's SYN
arrived on `br-imds`, the reply left through the other bridge, and the guest
logged nothing but `curl: (28) Connection timed out` until the run timed out —
after a nine-minute TCG boot. `/proc/net/arp` meanwhile held two entries for
`169.254.169.10` with different MAC addresses, one per bridge.

`vmbr_imds` kept winning the route lookup and `br-imds` is runtime-only, so
nothing persisted. But which of two identical-prefix connected routes wins is
decided by insertion order, not by design, and the next run could order it the
other way.

What that costs is worth stating accurately, because it decides how much
machinery this change deserves. `hogan` carries lab workloads; a short outage
that gets fixed straight away is acceptable there. The damage is not an
incident, it is an afternoon: timeouts with no obvious cause, a nine-minute TCG
boot before the symptom appears, and a debugging detour away from whatever was
actually being verified. That is what happened, and it is reason enough for a
check that costs three lines.

Two smaller faults share the cause. `cloud-install.sh` runs
`apt-get install -y qemu-system-x86 qemu-utils qemu-kvm ovmf`; on a Proxmox
host `/usr/bin/qemu-system-x86_64` comes from `pve-qemu-kvm`, and installing
Debian's packages over it can displace it, taking every guest on the host with
it until someone reinstalls. On `hogan` that is three VMs. Recoverable, and
still not a thing to trigger by accident.

`complete-the-cloud-lab-bootstrap` made that materially more likely, for a good
reason: the install used to be guarded on `qemu-system-x86_64` and
`cloud-localds` both being present, which silently skipped hosts that had those
two but lacked `swtpm_setup` or `ping`. It now installs whenever **any** of
sixteen tools is missing. The guard is better; the blast radius on the wrong
host is larger. Checked on `hogan` on 2026-10-03, exactly one of those sixteen
is absent — `jq`. A `cloud-install.sh` run there would therefore apt-get the
whole QEMU package set over `pve-qemu-kvm` because a JSON parser is missing.
Nothing stops it today.

And
`MDS_LAB_ACCEL` defaults to `tcg` with a comment about a KVM bug on cloud
agent hosts — a cloud-box workaround that costs every other host a factor of
ten in boot time, on `hogan` with `/dev/kvm` present and usable.

## What Changes

- The lab scripts refuse to run on a host that already has a conflicting
  metadata setup: an existing interface other than the lab bridge carrying the
  lab's addresses, or an existing route for the lab prefix. The refusal names
  the conflicting interface and exits non-zero before changing anything.
- `cloud-install.sh` refuses to install packages on a host where
  `pve-qemu-kvm` provides QEMU, and says that it is a cloud lab bootstrap.
- `MDS_LAB_ACCEL` defaults to KVM when `/dev/kvm` is present and usable, and
  falls back to TCG otherwise. The cloud-box workaround becomes the fallback
  rather than the default, and stays reachable with `MDS_LAB_ACCEL=tcg`.
- `scripts/qemu-lab/README.md` states which host the lab is for, and that on a
  Proxmox host the service is tested against a real guest and the installed
  package instead — so a reader who reached for the wrong tool is pointed at
  the right one rather than merely stopped.
- No change to the daemon, the client, the packaging, or any capability.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change alters lab tooling under `scripts/qemu-lab/` and its
documentation; no behaviour of the service changes, so `.openspec.yaml` sets
`skip_specs: true`.

## Impact

- `scripts/qemu-lab/setup-host.sh`, `cloud-install.sh`, `start-guest.sh`,
  `README.md`.
- Anyone who runs the lab on a Proxmox host: it now stops instead of producing
  a guest that cannot reach the service.
- Lab runs on the intended host: faster, because KVM becomes the default.
- `AGENTS.md` already warns against running the lab on `hogan` after the
  2026-10-02 attempt; this change makes the scripts enforce what that warning
  asks for.
