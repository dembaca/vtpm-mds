# Design

## Context

See proposal.md — Why.

The lab's network parameters are already overridable — `MDS_LAB_BRIDGE`,
`MDS_LAB_HOST_IP`, `MDS_LAB_IMDS_IP`, `MDS_LAB_PREFIX` — so a conflicting host
could in principle run the lab on a different subnet. That is not what this
change does, and the reason matters: the guest side carries the addresses too,
through the cloud-init network config written by `create-guest.sh` and through
`devid-enroll`'s default base URL of `http://169.254.169.254`. Moving the lab
to another subnet means moving all of them together, and no one has tested
that combination.

So the honest fix is a guard, not a reconfiguration: detect the conflict and
refuse.

`/dev/kvm` on `hogan` is `crw-rw-rw-` and the CPU carries the virtualization
flags, yet `start-guest.sh` reads `ACCEL_MODE="${MDS_LAB_ACCEL:-tcg}"`. The
comment in `cloud-install.sh` records why: nested KVM broke on the cloud agent
host. That is a property of one host, encoded as everyone's default.

## Goals / Non-Goals

**Goals:**

- The lab refuses to damage, or be defeated by, a host that already runs a
  metadata service.
- The refusal happens before the first side effect and names the conflict.
- A host that can run KVM does.

**Non-Goals:**

- Making the lab coexist with a production IMDS on one host. Two services
  claiming the same link-local addresses cannot both be right, and picking a
  different subnet for the lab means changing the guest image, the cloud-init
  network config and the client's default URL together. That is a larger
  change, and nobody needs it today — the cloud box exists.
- Removing the TCG path. It is the only option where KVM is unavailable, and
  it stays reachable explicitly.
- Teaching the lab to clean up a previous run's bridge. Deleting an interface
  that might be production is exactly the kind of guess this change exists to
  avoid.
- Changing `vm-inventory`'s ARP lookup to cope with one IP on two bridges.
  That ambiguity is a symptom of the conflict, not a defect to paper over, and
  it would need its own change and a spec delta.

## Decisions

### Refuse on conflict, do not reconfigure

`setup-host.sh` checks, before creating anything, whether any interface other
than `$MDS_LAB_BRIDGE` carries `$MDS_LAB_HOST_IP` or `$MDS_LAB_IMDS_IP`, and
whether a route for the lab prefix already exists through another interface.
Either is fatal, with a message naming the interface found.

Auto-selecting a free subnet was rejected: the guest's cloud-init config and
the client's default URL would have to follow, and a lab that silently moves
is a lab whose failures are harder to read than the conflict it avoided.

### Detect the hypervisor by what provides QEMU, not by a hostname

`cloud-install.sh` checks whether `pve-qemu-kvm` owns
`/usr/bin/qemu-system-x86_64` — via `dpkg -S` — and refuses to run its
`apt-get install` there. Matching on `/etc/pve` or on a hostname was rejected:
the thing that breaks is the QEMU package, so the test should be about the
QEMU package.

The refusal names the script as a cloud lab bootstrap, which is the
documentation gap that let it be run in the wrong place.

### KVM by default, TCG as the fallback

`ACCEL_MODE` defaults to `kvm` when `/dev/kvm` is present and both readable
and writable, and to `tcg` otherwise. The existing explicit `MDS_LAB_ACCEL`
override keeps precedence in both directions, so the cloud agent host sets
`MDS_LAB_ACCEL=tcg` once rather than everyone else paying for it.

The decision is recorded in the run output, as it already is, so which path a
run took is visible in the log.

## Risks / Trade-offs

- **[Risk] The conflict check is too eager and blocks a host that would in
  fact work** → Mitigation: it triggers only on the lab's own two addresses
  appearing on a foreign interface, or on a pre-existing route for the lab
  prefix. A host with neither is unaffected, and `MDS_LAB_BRIDGE` with the
  matching address variables still lets a deliberate operator move the lab.
- **[Risk] `dpkg -S` is unavailable or the QEMU binary comes from somewhere
  else, so the hypervisor check passes on a host it should stop** → Accepted
  and bounded: the check is a guard against a known-bad combination, not a
  proof of safety, and the README carries the rule in prose as well.
- **[Risk] Defaulting to KVM surfaces the nested-KVM bug the comment
  describes, on a host where it still exists** → Mitigation: that host sets
  `MDS_LAB_ACCEL=tcg`, and the failure is loud and immediate rather than
  silent. The comment moves to where the fallback is chosen so the next reader
  finds it.
- **[Trade-off] The lab becomes unrunnable on the hypervisor, where someone
  might want it for convenience** → Accepted. It was never actually runnable
  there; it only looked like it was, which cost a maintainer a nine-minute
  boot and a debugging session on a production host.

## Migration Plan

None. Development tooling. A host that has been running the lab successfully
keeps doing so; a host where it was silently broken is now told.

If a previous run left `br-imds` behind on a conflicting host, the operator
removes it — the scripts deliberately will not, since deleting an interface
they did not create is the guess this change avoids.

## Open Questions

None.
