# Proposal

## Why

**A VM has no way to carry information of its own to its guest.** The service
serves an instance id, a zone, a region and, when declared in the YAML
inventory, a public address. Anything else an operator wants a guest to know
about its VM, an owner, a rack, a role, a cost centre, has no path. EC2 has one:
*instance metadata tags*, served under `/latest/meta-data/tags/instance/`, which
cloud-init and scripts written for EC2 already know.

**#44 spent two Proxmox tag prefixes on one narrow use.** `mds.zone.<name>` and
`mds.region.<name>` let a VM override the host's zone and region. On
2026-10-18 the maintainer decided that topology is a property of the node only
("ja nur node. wüsste nicht, warum topologie granularer als node level sinn
ergeben könnte") and that VM tags become generic instead. That removes what #44
added at VM level before anything used it, and it makes room for the generic
mechanism.

The decision, from the maintainer on the agent bus, 2026-10-18, answering the
coordinator's five questions:

1. A VM tag `vtpm_mds.io.<key>.<value>`: the key is the first segment after the
   prefix, the value the rest, dots allowed.
2. Topology only from the node, nothing per VM.
3. The node description carries `topology.vtpm_mds.io/zone` and `/region`. The
   maintainer's aim is that the DG-i controller reads the topology from this
   service instead of the Proxmox API, so a node needs neither a firewall rule to
   the API nor a key (see "Open question" in `design.md`).
4. Nothing from these tags goes into a certificate.
5. `public-ipv4` as a tag is a possible later step, not part of this change.

## What Changes

- **A VM carries instance tags**, declared as Proxmox tags
  `vtpm_mds.io.<key>.<value>` read from the live section of its config. The YAML
  inventory gets a `tags` mapping with the same meaning.
- **They are served as EC2 instance tags**: `GET /latest/meta-data/tags/`,
  `.../tags/instance` (the keys) and `.../tags/instance/<key>` (a value). The
  metadata index lists `tags/` only for a VM that has some. A caller with no VM
  record, or a VM without tags, gets `404` with an empty body, as
  `public-ipv4` does.
- **Placement is the node's only.** `placement/availability-zone`,
  `placement/region` and the identity document serve the host's zone and region
  to every caller. The VM tags `mds.zone.<name>` and `mds.region.<name>` and the
  YAML fields `zone` and `region` stop having any effect.
- **The node's description is read under `topology.vtpm_mds.io/`** instead of
  `topology.kubernetes.io/`. The node name and the cluster name, then `proxmox`
  and `local`, remain the fallbacks.
- **Nothing goes into a certificate** and the identity document does not carry
  tags.

## Capabilities

### Modified Capabilities

- `instance-metadata`: a new requirement for instance tags; the EC2 tree
  requirement loses the per-VM placement and lists `tags/`; the identity
  document's placement scenario follows the host.
- `vm-inventory`: the YAML and Proxmox requirements are replaced (a scenario
  cannot be dropped from a `MODIFIED` block): tags in, per-VM placement out, the
  description prefix changed.

## Impact

- **Code, after the coordinator's ACK:** `internal/proxmox/vmconfig.go` (tag
  parsing, description prefix), `internal/inventory/` (a `Tags` field, YAML
  `tags`, placement per VM removed), `imds/handlers.go` (placement from the host
  only, new tag handlers), `internal/server/server.go` (routes), and their tests.
- **Behaviour change visible to operators:** a node whose description carries
  `topology.kubernetes.io/zone` stops being read; it needs a
  `topology.vtpm_mds.io/zone` line. #44 is merged and nothing consumes it yet
  except the test line on hogan.
- **Not in scope:** certificates (D9 in #33 recommends nothing from tags), a
  `public-ipv4` tag, `placement/` listing, the YAML typo that empties the
  inventory.
