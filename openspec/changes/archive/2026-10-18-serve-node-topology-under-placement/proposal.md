# Proposal

## Why

**A node can declare only a zone and a region, and the metadata service serves
only those two under `placement/`.** `serve-vm-tags-as-instance-metadata` made
topology the node's alone: `placement/availability-zone` and `placement/region`
come from the lines `topology.vtpm_mds.io/zone` and `.../region` of the node's
description, else from the node and cluster names. Anything else a data centre
wants a guest to know about where it runs, a rack, a room, a row, has no path, and
the keys the maintainer proposed for a default set, `rack`, `cluster` and `node`,
cannot be served at all. `placement/` itself is answered by the metadata index,
so a tool that walks the tree cannot discover a key.

The maintainer decided on 2026-10-18, on the agent bus, after proposing the set
`region`, `zone`, `rack`, `cluster`, `node`: any key of the node's description under
`topology.vtpm_mds.io/` is delivered under `placement/<key>`, even where that
overrides an EC2 key such as `group-name` or `host-id`, "in the assumption that the
node description can only be written by a Proxmox admin, who can do everything in
the cluster anyway"; `zone` is `availability-zone`; `cluster` and `node` default
from the host and the description overrides them. The coordinator's recommendation
that a key without a value, `rack` for example, answers `404` and is not listed
was taken with it.

## What Changes

- **Any key of the node description is served under `placement/`.** A line
  `topology.vtpm_mds.io/<key>: <value>` in the description of the node becomes
  `GET /latest/meta-data/placement/<key>`. `zone` is the short form of
  `availability-zone`; both forms together declare nothing, with a log line.
- **Defaults come from the host.** `availability-zone` is the node name, `region`
  the cluster name, `node` the node name and `cluster` the cluster name, each
  overridden by its description line. The old constants `proxmox` and `local`
  remain the last resort for the first two. A key with no value, such as `rack`
  without a line, answers `404` and is not listed.
- **`placement/` lists the keys**, one per line, instead of answering the metadata
  index. A path `placement/<key>` the topology does not have answers `404`
  with an empty body, not the index. *(See D-g in `design.md`: this is an addition
  to the assignment and can be dropped.)*
- **EC2 keys can be overridden.** `group-name`, `host-id` and `partition-number`
  are ordinary keys; the service keeps no list of keys it refuses.
- **Nothing else moves.** The identity document keeps `availabilityZone` and
  `region`; the instance tags, the VM id and the certificate are untouched.

## Capabilities

### Modified Capabilities

- `instance-metadata`: the EC2 tree requirement serves every topology key under
  `placement/`, lists them, and answers an absent key `404`; six scenarios added.
- `vm-inventory`: the Proxmox requirement reads any `topology.vtpm_mds.io/<key>`
  line of the node's description, with defaults for four keys, a key syntax, a
  limit of 64 keys and the long form of the zone; eight scenarios added, one
  reworded.

## Impact

- **Code, after the coordinator's ACK:** `internal/proxmox/vmconfig.go` (the
  description reader and the defaults), `internal/inventory/placement.go` (the
  host topology becomes a map), `imds/handlers.go` and a new handler file for
  `placement/<key>` and `placement/`, `internal/server/server.go` (routes), and
  their tests. The YAML backend's host topology is decided in D-f.
- **Behaviour change visible to operators:** `placement/<key>` for an unknown key
  and `placement/` itself answer differently than before (`404` and a key list,
  not the index). Neither was used by anything known.
- **A new exposure:** every line under `topology.vtpm_mds.io/` in a node's
  description is readable by every guest of that node. Nothing secret belongs
  there (see "Risks" in `design.md`).
- **Not in scope:** the Kubernetes consumer (a per-node DaemonSet, outside this
  repository), certificates, `public-ipv4` as a tag, VM-level topology, and the
  YAML typo that empties the inventory.
