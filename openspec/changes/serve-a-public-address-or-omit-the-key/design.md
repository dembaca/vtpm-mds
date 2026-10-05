# Design

## Context

`getPublicIP` has been an unimplemented `TODO` since the service was written.
The known-gaps queue in `AGENTS.md` records it with the right framing: "Where
the address should come from is an open question, not just an implementation."
This change record exists to make that question answerable, not to answer it.

What the service actually knows about a caller today:

- The **peer address** of the connection, which is the guest's address on the
  IMDS bridge. By construction that is a private, link-local-adjacent address;
  it is what `local-ipv4` already serves, and it is never a public address.
- The **bound VM record** from `vm-inventory`: `VMID`, `Name`, `MACs`,
  `EKSHA256`, and `RawConfig map[string]string`, documented as "Optional
  key/value metadata".

Nothing else. The service does not see the guest's other NICs, its routing
table, or any upstream NAT. **A public address is therefore not something this
service can observe — only something it can be told.** That single fact
decides most of the options below.

## Goals

- The index lists only keys the caller can actually read.
- A declared public address is served to the VM it belongs to.
- The service never guesses, and never presents a private address as a public
  one.

## Non-Goals

- Discovering a public address by probing, by querying an external service, or
  by asking the guest.
- `getAvailabilityZone` and `getRegion`. They carry the same `TODO` comment but
  return the real fixed values the spec pins. A comment is not a defect.
- IPv6. `public-ipv6` is not an AWS IMDS key under this name and nothing asks
  for it.
- Changing the `404` response shape. Empty body, `Content-Type: text/plain`,
  unchanged.

## Decisions

### The key leaves the index when there is no value, rather than the index losing the key

Two ways to stop advertising something unreadable:

1. **Drop `public-ipv4` from the index permanently** and keep the endpoint
   answering `404`. Simple, caller-independent, and honest about a deployment
   that will never have public addresses.
2. **List the key only for a caller that has an address.** The index becomes
   caller-dependent.

Option 2 is chosen, because option 1 forecloses the feature: a VM that *does*
have a declared public address would still not have it advertised, and a guest
walking the index would never find it. Option 1 is also strictly worse than
what EC2 does, which is exactly option 2.

The cost is real and worth stating: **the index stops being the same for every
caller**, which it has been until now. A cached or shared index is wrong after
this change. Nothing in the repository caches it, and the handler re-renders
per request, so this is a property to preserve rather than a bug to fix.

### `404`, not an empty `200`

Serving `200` with an empty body would keep the index fixed and let the caller
distinguish "no address" by reading an empty string. Rejected: an empty body on
a `text/plain` metadata key is indistinguishable from a truncated response, and
`local-ipv4` already answers `404` for the analogous "no IPv4 address" case.
Two keys with the same question should not have two different answers.

## Open question for the maintainer — this is the one that matters

**Where does the address come from?** Four candidates, with what each costs.

### A. An explicit inventory field — *recommended*

A per-VM `public_ipv4` in the YAML inventory, or a `public-ipv4` key in the
existing `RawConfig` map, which `VMConfig` already carries for exactly this
kind of thing.

- **For:** the operator states it, so the service asserts only what it was
  told. No parsing of a format owned by someone else. Works identically under
  the YAML and the Proxmox backend. Testable without a hypervisor. It is the
  same trust model as the rest of `vm-inventory`, which the identity endpoints
  already depend on.
- **Against:** it is manual, and it will go stale if an address changes and
  nobody updates the inventory. A stale public address is worse than none.

### B. The Proxmox VM config — what the `TODO` intended

Parse `ipconfig0` from `/etc/pve/qemu-server/<vmid>.conf`.

- **For:** no second place to maintain; the operator already configures the VM
  there.
- **Against:** `ipconfig0` is a **cloud-init** setting. It exists only for VMs
  provisioned that way, it describes what cloud-init was told to configure on
  the first NIC, and it is frequently `dhcp`. It is not a statement about a
  public address, and treating it as one would make the service assert
  something it was never told. This is option A with a less explicit source and
  a worse failure mode.

### C. The QEMU guest agent

`network-get-interfaces` over the agent socket.

- **For:** it reflects reality rather than a declaration.
- **Against:** it requires the agent in every guest, it returns *all*
  addresses with nothing marking which is public, and the service would have to
  guess by excluding RFC1918 ranges — which is wrong behind NAT, where the
  public address is not on the guest at all. It also turns a metadata read into
  a synchronous call into the guest, which is a new failure and latency mode on
  a path that currently touches nothing but a cache. Recommended against.

### D. Leave it unimplemented and drop the key from the index

The honest minimum if no deployment has public addresses.

- **For:** smallest change; nothing can be stale or wrong.
- **Against:** closes the feature. See the decision above.

**Recommendation: A**, with the value read from `RawConfig["public-ipv4"]` so
no struct change is needed, validated as an IPv4 address at load time, and the
load failing loudly on a malformed value in the manner
`fail-fast-on-unusable-configuration` established. If the maintainer prefers
D, this change shrinks to the index line and `tasks.md` loses sections 2 and 3.

**Not for me to decide**, and recorded here rather than guessed, in the same
way the `422`, the IPv6 treatment and the LDevID subject were decided by the
maintainer.

## Second open question

**Should a declared public address also reach
`/latest/dynamic/instance-identity/document`?** That document has a fixed key
set and no public-address key today, so it would be an addition rather than a
fix, and it is deliberately not in this change's scope. Raised only so the
answer is not assumed.
