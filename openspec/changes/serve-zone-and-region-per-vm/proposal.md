# Proposal

## Why

**`placement/availability-zone` is the constant `proxmox` for every guest, and
`placement/region` is not served at all.** `getAvailabilityZone` and `getRegion`
in `imds/handlers.go` return `"proxmox"` and `"local"` behind a `TODO: Extract
from Proxmox cluster config`; the identity document reports the same two
constants. A guest cannot tell which node it runs on or which cluster it belongs
to, and an operator cannot place two guests in two zones. `placement/region` is
a genuine key of the EC2 metadata tree that tooling reading the EC2 shape
(cloud-init, bootstrap scripts, AWS-derived components) asks for; today it is
answered by the index handler like any unknown key.

The maintainer decided on 2026-10-17 ("M1–M5 alle wie empfohlen", answering the
coordinator's decision sheet in `declare-per-vm-metadata`, #33) to serve zone and
region first and to leave `public-ipv4` deferred. `design.md` there records why
Kubernetes itself does not read these paths (D6); this change is for the
consumers that do.

## What Changes

- **A VM declares its zone and region with Proxmox tags**, `mds.zone.<name>` and
  `mds.region.<name>`, read from the live section of its config. The YAML
  inventory gets `zone` and `region` fields with the same meaning.
- **Without a declaration the host's placement is served**, from the first
  source that gives a name: the `topology.kubernetes.io/zone` and `.../region`
  lines of the node's description, the node name as the zone and the cluster name
  from `/etc/pve/corosync.conf` as the region, then `proxmox` and `local`. A name
  keeps the case its source gives it. The YAML backend's host placement stays
  `proxmox` and `local`.
- **`placement/region` becomes a real key**, and the identity document's `region`
  and `availabilityZone` follow the same values.
- **Faults are local.** Two zone tags on one VM, an invalid name, or a bad YAML
  value are logged and treated as "not declared" for that VM; nothing else is
  affected.
- **A guest never declares its own placement.** The sources are the operator's
  tags or file and the host's own configuration. A hand-written `zone:` or
  `region:` line in a Proxmox config is not stored, as `public-ipv4:` is not.

## Capabilities

### Modified Capabilities

- `instance-metadata`:
  - `Serve The EC2 Metadata Tree With A Caller-Dependent Index` — the two
    placement keys and their resolution; two scenarios reworded, five added.
  - `Serve The Instance Identity Document Without A Signature` — `region` and
    `availabilityZone` per caller; one scenario reworded, one added.
  - `Refuse An Instance Identity To Callers With No VM Record` — one sentence:
    the placement keys are the host's, not fixed strings.
- `vm-inventory`:
  - `Derive VM Records From Proxmox Guest Configuration` — placement tags, host
    placement, no raw `zone`/`region`; seven scenarios added.
  - `Load VM Records From The YAML Inventory` — `zone` and `region`; two
    scenarios added.

## Impact

- `internal/proxmox/vmconfig.go` (tags, host placement), `internal/inventory/`
  (carrying the host placement with the cache), `imds/handlers.go` and
  `internal/server/server.go` (the new route and the two resolvers), their tests,
  and `scripts/qemu-lab/` inventory files only if the lab half wants a declared
  zone (the cloud lab's, not touched here).
- Operators: Proxmox tags become a trust input for what a guest reads about its
  placement. `design.md` recommends restricting the `mds.*` tags with
  `registered-tags`.
- **Ordering:** independent of `declare-per-vm-metadata`'s remaining decisions
  (D2 values carrier, D8/D9 SPIRE). It touches `internal/proxmox/vmconfig.go`,
  which `read-guest-configs-from-this-node` changed and archived on 2026-10-17;
  the two do not overlap.
