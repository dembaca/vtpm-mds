# Design

## Context

`serve-vm-tags-as-instance-metadata` (archived) serves the host's zone and region,
read from `topology.vtpm_mds.io/zone` and `.../region` in the description of the
node, else from the node and cluster names. The maintainer has since asked for a
default set `region`, `zone`, `rack`, `cluster`, `node`, and for any key under that
prefix to be served under `placement/` (see the proposal).

### Evidence read in this repository (2026-10-18, `main@407bfca`)

- `internal/proxmox/vmconfig.go`: `descriptionPlacement(path)` reads
  `<root>/local/config`, decodes the `#` lines and matches the regular expression
  `^\s*topology\.vtpm_mds\.io/(zone|region):\s*(\S+)\s*$`; `HostPlacement(root)`
  resolves zone and region from it, then the `local` link and `cluster_name`, then
  `proxmox` and `local`.
- `internal/inventory/placement.go`: `type Placement struct{Zone, Region string}`,
  `DefaultPlacement`, a mutex-guarded `SetHostPlacement`/`HostPlacement`, and
  `ValidPlacementName`.
- `imds/handlers.go`: `getAvailabilityZone`/`getRegion` read `HostPlacement()`;
  the document uses the same two functions. `internal/server/server.go` registers
  `placement/availability-zone` and `placement/region` by name and
  `GET /latest/meta-data/` as the subtree index handler, so `placement/` and every
  other unregistered path under it are answered by the index.
- The consumer on the data-centre side (read by the leitstand agent, 2026-10-18)
  parses the same description line by line with a prefix match on
  `topology.kubernetes.io/` and cuts the value at the first space. A line under the
  other prefix does not disturb it, so both prefixes can stand side by side.

## Decided (maintainer, 2026-10-18)

| Said | Decision |
|---|---|
| Any key under `topology.vtpm_mds.io/` | Served under `placement/<key>`, including EC2's `group-name`, `host-id`, "if it is necessary for some reason, it costs us nothing to be open here". |
| `zone` means `availability-zone` | `topology.vtpm_mds.io/zone` is the line for `placement/availability-zone`. |
| `cluster` and `node` | Defaults from the PVE host, overridden by the description. |
| The default set `region`, `zone`, `rack`, `cluster`, `node` | `rack` has no default. |
| Trust | The node description can be written only by a Proxmox administrator (`Sys.Modify` on `/`, `declare-per-vm-metadata` D9), who can do everything in the cluster anyway. |

Taken from the coordinator's assignment: a key with neither line nor default,
`rack` for example, is answered `404` and is not listed; `zone` and
`availability-zone` both set declare nothing, with a log line.

## Decided by this record's author, for the maintainer to overrule

- **D-a. Key and value syntax.** A key is 1 to 63 characters from `a-z`, `A-Z`,
  `0-9`, `_`, `-` and `+` (the instance-tag key syntax, so a key is a safe path
  segment); a value is 1 to 63 characters from the same set plus `.`, starting with
  a letter or digit (the existing placement-name syntax). Both keep their case.
  The description reader today takes a value to be `\S+`; an invalid value is
  logged and ignored, as it is today.
- **D-b. Duplicates.** Two lines for one key, valid or not, declare nothing for it,
  as for zone and region today; `zone` with `availability-zone` counts as one key.
  A line that does not parse is ignored without a log entry; a line that parses
  with a bad key or value is logged.
- **D-c. Limit.** At most 64 keys are read; beyond the 64th in ascending byte order
  they are logged and ignored. A description is free text edited by hand and the
  limit bounds the listing; 64 is mine.
- **D-d. Defaults for `node` and `cluster`.** `node` is the target of the
  `/etc/pve/local` link, `cluster` the `cluster_name` of `corosync.conf`. When the
  cluster file is missing there is **no** `cluster` key (`404`), while `region`
  stays `local`: a `region` is never empty (the existing rule), a `cluster` that
  does not exist should not be invented.
- **D-e. A key is never refused.** The service keeps no list of keys it will not
  serve (`group-name`, `host-id` and `partition-number` among them): the maintainer's
  decision, and the trust is the administrator's. The index keeps listing
  `placement/` and the document keeps its fixed key set.
- **D-f. The YAML backend declares no topology.** Its host topology stays zone
  `proxmox` and region `local`, with no `node` and no `cluster`, so the lab can only
  show the defaults and the `404`. A top-level `topology` mapping in the YAML
  inventory would let the lab show arbitrary keys too; it is a small addition
  (a mapping from key to value, same syntax, same rules) and is left out only
  because the assignment does not name it. **Question for the maintainer:** add it
  here, or leave the lab with defaults only?
- **D-g. `placement/` lists the keys.** Without a listing a tool that walks the
  tree, cloud-init for example, cannot find a key it does not know by name, and
  the listing was already named a gap (`placement/` answers the metadata index).
  It is one handler. It was not in the assignment: drop D-g and the proposal's
  third item and nothing else changes. With it, an unknown `placement/<key>` is
  `404` with an empty body, not the index.
- **D-h. Order of the listing.** Ascending byte order, for a stable answer; EC2
  lists in its own fixed order, and nothing known depends on it.
- **D-i. The identity document is unchanged.** It keeps exactly `availabilityZone`
  and `region` (EC2 fields); the other keys are not added.

## Trust and exposure

D9 found that the node description needs `Sys.Modify` on `/`, which only
`Administrator` holds among the standard roles, so the topology is the most
trustworthy of the declared values. This change widens what it carries: every line
under `topology.vtpm_mds.io/` in a node's description is readable, with no token
besides the session token every metadata read needs, by every guest of that node.
A guest that could read two values can now read all of them. Anything in a
description that is not meant for guests must not stand under that prefix, and a
description is also shown in the Proxmox web interface to whoever may see the node.

## Open question

The maintainer's aim for the data centre is a per-node reader of this topology,
a small DaemonSet, outside this repository (decided later, not here). The keys
`rack`, `cluster` and `node` are what such a reader would label a Kubernetes node
with; this record does not depend on it.

## Implementation notes

For the coordinator's review; code only after the ACK.

- Generalise `descriptionPlacement` into a reader that returns a map of key to
  value for every line under the prefix, applying D-a to D-c, with `zone` mapped
  to `availability-zone` and a conflict of the two logged.
- `inventory.Placement` becomes a topology map (key to value) with the four
  defaults; `HostPlacement()` keeps working for the identity document by reading
  `availability-zone` and `region`. The mutex-guarded setter stays.
- `imds`: a handler for `placement/{key}` and one for `placement/`; the two
  existing named handlers can stay or become cases of the first. `404` with an
  empty body is the shared helper from `tags.go`.
- Tests: the host-placement tests keep their shape (they pass with the new reader
  for zone and region); new tests for each new scenario; mutations for the
  duplicate rule, the long form, the limit, and the `404` of an absent key.

## Non-Goals

- **A VM-level topology**, decided away on 2026-10-18.
- **Anything in the certificate**, and `public-ipv4` as a tag.
- **The Kubernetes consumer**, outside this repository.
- **Writing a description.** The service only reads it.

## Risks

- **Exposure of everything under the prefix** (see above).
- **Overriding an EC2 key** such as `host-id` can confuse a tool that assumes
  EC2's meaning; the maintainer chose it knowingly, and the administrator's
  trust is the safeguard.
- **A description is free text.** The reader ignores what does not parse and logs
  what parses wrongly; a typo in a key name serves a different key and nothing
  notices. The listing makes the effective set visible.
- **Migration.** Nodes that carry only the former prefix `topology.kubernetes.io/`
  are not read; they fall back to the node and cluster names, as since the last
  change.
