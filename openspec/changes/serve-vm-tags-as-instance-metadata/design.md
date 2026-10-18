# Design

## Context

#44 (`serve-zone-and-region-per-vm`, archived) lets a VM declare its zone and
region by Proxmox tags `mds.zone.<name>` and `mds.region.<name>` and reads the
host's placement from a node's description under `topology.kubernetes.io/`. The
maintainer has since decided to drop the per-VM placement and to serve generic
tags (see the proposal). `declare-per-vm-metadata` (#33) holds the evidence for
the trust of tags, D9, which this record applies.

### Evidence read in this repository (2026-10-18, `main@24087c4`)

- `internal/proxmox/vmconfig.go` has `placementFromTags`/`placementOfKind` for the
  prefixes `mds.zone.` and `mds.region.`, and `descriptionPlacement` for the
  `topology.kubernetes.io/` lines.
- `imds/handlers.go` has `placementOf(r, key, host)`: the bound VM's
  `RawConfig["zone"|"region"]` when valid, else the host's.
- `internal/server/server.go` registers every `placement/` and `services/` key
  by name and `GET /latest/meta-data/` as the subtree index handler, so a path
  under it that is not registered is answered by the index.
- A VM record is `inventory.VMConfig` with `RawConfig map[string]string`; there
  is no field for tags.

## Decided (maintainer, 2026-10-18)

The five answers, as in the proposal. Mapped to this record:

| Answer | Decision |
|---|---|
| 1 | Tag `vtpm_mds.io.<key>.<value>`, key first, value the rest with dots; served as EC2 instance tags. A duplicate key: log, none valid (the coordinator's assignment). The case of key and value as the source has it (the maintainer's rule of 2026-10-17, kept). |
| 2 | Topology only from the node. The `mds.zone.*`, `mds.region.*` tags and YAML `zone`, `region` are removed. |
| 3 | The node description carries `topology.vtpm_mds.io/zone` and `/region`. Only that prefix, as the coordinator's assignment states. |
| 4 | Nothing into the certificate; the identity document does not carry tags. |
| 5 | `public-ipv4` as a tag is a possible later step. |

## Decided by this record's author, for the maintainer to overrule

These fill gaps the answers left.

- **D-a. Key and value syntax.** A key is 1 to 63 characters from `a-z`, `A-Z`,
  `0-9`, `_`, `-`, `+`; a value 1 to 255 from the same set and `.`. Proxmox's
  tag format allows `a-z0-9_-+.` and ignores case (D9 in #33, `PVE_TAG_RE`), so
  every value is expressible; there is no `/`, `:`, `=` or space, which is also
  what keeps a key a safe URL path segment. The key cannot contain a dot because
  the first dot after it ends it; that is the decision's own price. The limits
  63 and 255 are mine; EC2 allows 128 and 256 characters.
- **D-b. The prefix is matched without regard to case.** Proxmox ignores case in
  tags, so `VTPM_MDS.IO.Owner.Team-A` is a plausible way to write the same tag.
  Key and value keep their spelling. A stricter reading, case-sensitive prefix, is
  easy to ask for.
- **D-c. Two tags for one key declare nothing, valid or not.** As for the
  placement tags in #44: a typo next to a good tag must not silently pick the
  good one.
- **D-d. What a missing or unknown tag answers.** `404` with an empty body, for
  no tags at all, an unknown key and an unbound caller alike, distinct from the
  router's `404 page not found`. How EC2 answers for an instance without tags was not checked against its
  documentation. The index lists
  `tags/` only for a VM that has some, as it lists `public-ipv4` only for one
  that declares an address.
- **D-e. Where the tags live in the record.** A new `Tags map[string]string` on
  `inventory.VMConfig`, not entries in `RawConfig`, so a tag can never collide
  with a Proxmox key such as `name` and the handlers cannot read one by mistake.
- **D-f. `tags/` answers `instance`.** EC2's directory listing; a fixed line, so
  a tool that walks the tree finds the same shape.
- **D-g. YAML parity.** `tags` is a mapping, trimmed, an invalid pair logged and
  ignored; it cannot fail the load, unlike `public_ipv4`, because it degrades to
  one tag fewer and nothing else.
- **D-h. Old declarations fail soft.** A VM that still carries `mds.zone.x`, or a
  YAML `zone`, loads without a message and reads the host's placement; neither
  is logged, because the YAML key is an ignored unknown key and the Proxmox tag is
  one without the prefix.

## Trust (D9 applied)

D9 in #33 found that anyone with `VM.Config.Options` on a VM, which includes
`PVEVMAdmin`, can set any unregistered tag under the default
`user-tag-access: free`, and that `registered-tags` protects only tags listed
exactly. For this change that means: **a VM administrator decides what the guest
reads under `tags/instance/`.** That is acceptable because the data reaches the
guest of that same VM and nothing else (answer 4: nothing goes into a certificate), and
because the placement, which a Kubernetes consumer acts on, comes from the node
description (`Sys.Modify` on `/`, `Administrator` in the standard roles). Where it
matters, `user-tag-access: user-allow: none` closes the tag path for everyone
without `Sys.Modify`. The documentation says so; no code does.

## Open question

**The aim "the DG-i controller asks this service for the topology"** (the
maintainer, answer 3): the service answers each VM about itself only, binding the
caller by MAC and ARP, so a central controller cannot ask for another VM's zone.
The model works if each Kubernetes node reads its own zone, for example by a
DaemonSet that patches its own node object or by `--node-labels` at bootstrap. The
coordinator asked the maintainer on the bus whether that is the plan; this record
does not depend on the answer, but the sentence would become a goal if it is.

## Implementation notes

Written for the coordinator's review; code only after the ACK.

- Parse tags in `internal/proxmox/vmconfig.go` where `placementFromTags` is
  today: one function that splits at `;`, `,` and whitespace, matches the prefix
  case-insensitively, takes the key up to the next dot, collects per key, and
  returns the valid, unique pairs. Remove `placementFromTags`/`placementOfKind`.
- Change the description prefix constant; the reader stays.
- `imds`: `placementOf` becomes a read of `inventory.HostPlacement()`; the
  document uses it. New handlers for `tags/`, `tags/instance` and
  `tags/instance/{key}`; the key route needs a path value, so a pattern such as
  `GET /latest/meta-data/tags/instance/{key}`. The three more specific routes win
  over the subtree index.
- Remove YAML `zone` and `region` from the inventory structure and add `tags`.
- Tests: unchanged tests for the host placement keep their shape with the new
  prefix; the tag and placement-per-VM tests are replaced.

## Non-Goals

- **Certificates.** No tag, no placement, goes into a certificate or an identity
  document. SPIRE 1.14 reads only `subject:cn`, and D9 advises against it.
- **`public-ipv4` as a tag**, and mirroring individual keys onto EC2 paths.
- **A `placement/` listing** (it answers the `meta-data` index today) and the
  YAML typo that empties the inventory. Each is its own change.
- **Changing the VM id or the CN of the LDevID.** D8 a stands.
- **Changing tags from the service.** It only reads.

## Risks

- **Migration.** Nodes whose description carries `topology.kubernetes.io/` lines
  are not read any more; they fall back to the node and cluster names until a
  `topology.vtpm_mds.io/` line is added. hogan carries a test line in the old
  form (`TEST_Z`), which stops having an effect.
- **A tag value that looks like a secret.** Anyone who can tag a VM can put a
  string in front of its guest; nothing prevents putting something sensitive in a
  tag, and tags are visible in the Proxmox UI to anyone who can see the VM.
  Operators should not.
- **The dot rule.** A key cannot contain a dot, so a source key such as
  `cost.centre` has to be spelled `cost-centre` or `cost_centre`.
