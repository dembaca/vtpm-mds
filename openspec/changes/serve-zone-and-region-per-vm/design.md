# Design

## Context

Written on 2026-10-17 against `main` at `983eb61`, from the decisions the
maintainer gave on the agent bus ("M1–M5 alle wie empfohlen") to the sheet in
`declare-per-vm-metadata` (#33). That record holds the evidence for Proxmox tags
(character set, `registered-tags`, the `tags:` line), for what a Proxmox config
keeps and drops, and for what Kubernetes reads (D6).

### Evidence read in this repository

- `imds/handlers.go`: `getAvailabilityZone` returns `"proxmox"` and `getRegion`
  `"local"`, each behind `TODO: Extract from Proxmox cluster config`.
  `HandleAvailabilityZone` is registered in `internal/server/server.go`; no route
  exists for `placement/region`, so that path is answered by the metadata
  index handler (`instance-metadata`, "Unknown metadata key returns the index").
- The identity document takes `region` and `availabilityZone` from the same two
  functions. `PublicIPv4(r)` shows the pattern for a per-VM value: the bound
  record's `RawConfig` through `inventory.GetVMConfigFromRequest(r)`.
- `instance-metadata` pins the constants in three places: the tree requirement,
  the identity document requirement and one sentence of the unbound-caller
  requirement.
- The cache is reloaded on SIGHUP and on start (`vm-inventory`, "Load The Cache
  At Startup And Refresh It On SIGHUP"); a failed reload keeps the old cache.

## Decided (maintainer, 2026-10-17)

- **M1, two mechanisms.** Labels (zone, region) are Proxmox tags; values such as
  addresses go to their own file when `public-ipv4` comes back. This change does
  the labels only.
- **M2, tag format.** `mds.zone.<name>` and `mds.region.<name>`. Two zone tags on
  one VM: log, ignore both, the default applies. Operators are advised to allow
  those tags only to admins through `registered-tags` (`Sys.Modify`).
- **M3, defaults.** Zone is the node name, region the cluster name. Precedence:
  VM tag over default.
- **M4, who declares.** Operator (tag or file) or host (derivation), never the
  guest.
- **M5, faults.** A broken or unknown tag is logged and treated as not set; every
  other VM is untouched. YAML gets `zone` and `region` with the same meaning.

## Decided later the same day, by the maintainer

On the agent bus, after the coordinator proposed it from what the DG-i data centre
does (a `k8s-proxmox-node-label` controller copies `topology.kubernetes.io/zone`
and `region` lines from the **description of the Proxmox node** onto Kubernetes
nodes):

- **Node description, yes.** The order is **VM tag, then node description, then
  node name and cluster name.** The same source and the same keys as the
  controller, so the metadata service reads what the labels carry, with nothing to
  keep twice. It amends M3: the defaults gain a middle step.
- **Case as in the source.** "übernimm groß/klein wie es in der Quelle steht,
  kein lc() uc()". No source's case is changed, neither tag, description, node
  name nor cluster name. `DUS4` stays `DUS4`.

## Decided by this record's author, for the maintainer to overrule

These fill gaps the sheet left; none changes M1–M5.

- **D-a. Name syntax.** `<name>` is 1 to 63 characters from `a-z`, `A-Z`, `0-9`,
  `_`, `-`, `+`, `.`, starting with a letter or digit: Proxmox's tag characters
  (its tag format ignores case), so every valid tag can carry it, and valid in a
  URL path and a JSON string. Capitals were added on the maintainer's decision
  below; the first draft allowed lowercase only.
- **D-b. Splitting `tags`.** At `;`, `,` and whitespace. Proxmox writes `;`; the
  lenient split costs nothing and avoids a silent miss on a hand edit.
- **D-c. "Two tags" includes an invalid one.** Two tags of one kind, valid or not,
  declare nothing. Otherwise a typo next to a good tag would silently pick the
  good one.
- **D-d. An unbound caller gets the host's placement.** The old text called these
  values fixed strings and served them to unbound callers; the host's zone and
  region are no more an identity than the constants were, and no VM's declaration
  is visible to a caller the service cannot bind.
- **D-e. No raw `zone`/`region` lines on Proxmox.** Proxmox drops unknown keys on
  the next write, as it does `public-ipv4:`; storing them would offer a source
  that vanishes, and would let anyone who can write the file, rather than the
  tag-editing role, declare a placement.
- **D-f. A bad YAML value is logged, not fatal.** `public_ipv4` fails the load
  because an absent address is indistinguishable from none; a bad `zone` falls
  back to the host's value. That differs from `public_ipv4` on purpose and is
  stated in the delta. Why one typo empties the whole YAML inventory today is a
  separate question, named by the coordinator, for its own change.
- **D-g. Host placement refreshes with the cache.** Node name and cluster name are
  read on every load, so SIGHUP picks up a renamed cluster. A tag edit is also
  visible only after a SIGHUP or restart, like any inventory change.
- **D-h. Invalid host names.** A node name, a cluster name or a description value
  that is not a valid placement name is logged and the next source is used, so the
  served value is never empty or unsafe. (The first draft lowercased the names
  before checking; the maintainer decided against any case change, below.)
- **D-i. YAML backend host placement stays `proxmox` and `local`.** The lab and
  any existing deployment keep what they get now unless an entry declares
  otherwise.

## Implementation notes

- A per-VM value is read from the record's raw configuration map under `zone` and
  `region`, as `public-ipv4` is. The host placement is a pair carried with the
  loaded cache, so handlers read both without touching `/etc/pve` per request.
- The new route is `GET /latest/meta-data/placement/region`, registered next to
  the zone route; the index handler keeps answering other unknown keys. The index
  lists `placement/` already and needs no change.
- `corosync.conf` is read as text for the `cluster_name:` line inside `totem`;
  no cluster library is involved.

## Non-Goals

- **`public-ipv4` and any value carrier** (D2 in #33): deferred by the maintainer.
- **A Kubernetes consumer.** D6 in #33 found that Kubernetes does not read these
  paths; this serves the EC2 shape for the consumers that do.
- **Why a typo in the YAML inventory empties it,** and non-virtio NICs: their own
  questions.
- **Other `placement/` keys** (`availability-zone-id`, `region` aliases) and any
  tag beyond the two.
- **The SPIRE side** (D8/D9): untouched.

## Risks

- **Tags are an input anyone with `VM.Config.Options` can write.** Zone and region
  then reach a guest's view of itself. M2's advice, `registered-tags`, is the
  control; this change does not enforce it and cannot.
- **A zone that changes under a running guest.** Kubernetes assumes topology
  labels do not change for a node. A tag edit changes what the guest reads on its
  next request after a reload; a consumer that read once keeps its old value.
- **The node description is writable by anyone with `Sys.Modify` on the node**,
  a higher bar than a VM's tags, and read from `/etc/pve/local/config`, which
  most nodes lack. Format taken from Proxmox's `PVE/NodeConfig.pm` as the
  coordinator read it on hogan (a `#` line per description line, URL-encoded);
  hogan has no node config, so the step has not run on a real file here.
- **`/etc/pve/corosync.conf` may not be readable by the service's user or may
  name no cluster,** in which case the region is `local`. Not yet checked on
  hogan (task 3.1).
- **A node rename** changes the zone of every guest on it at the next reload.
- **A reload that fails** keeps the old cache, host placement included.
