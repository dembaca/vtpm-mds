# Design

## Context

`proposal.md` sets out why this exists. This document records the evidence,
so nobody has to derive it again, and lays out the decisions for the
maintainer. Sources were read on 2026-10-17. Line numbers drift between
versions, so each source names the copy it came from.

### Evidence: Proxmox does not keep a hand-written key

- **`check_type` rejects unknown keys.** `PVE/QemuServer.pm`, `sub check_type`:
  `die "unknown setting '$key'\n" if !$schema->{$key};`. That is line 1804 in
  `proxmox/qemu-server` at `4127436e5a7e` (`src/PVE/QemuServer.pm`), and line
  1769 in `/usr/share/perl5/PVE/QemuServer.pm` on hogan.
- **Parse errors only warn, and the key is dropped.** In `sub parse_vm_config`,
  `$handle_error` is `if ($strict) { die $msg } else { warn $msg }` (upstream
  1951–1959, hogan 1912). For a `key: value` line, `$conf->{$key} = $value`
  runs only when `check_type` succeeds. Otherwise the message is
  `vm $vmid - unable to parse value of '$key' - $@` (upstream 2053).
- **A write serialises only parsed keys.** In `sub write_vm_config`,
  `$generate_raw_config` (upstream 2153–2172) writes the description and then
  `"$key: $conf->{$key}\n"` for each parsed key. `qm set`, a web UI edit, a
  snapshot or a migration therefore removes the hand-written line.
- **Our parser keeps it.** `internal/proxmox/vmconfig.go` stores every
  `key: value` or `key=value` line in `RawConfig` and skips lines starting with
  `#`. That matches `Derive VM Records From Proxmox Guest Configuration`.

### Evidence: the carriers Proxmox does keep

- **Notes (`description`).** These are written as URL-encoded `#` lines at the
  top of the file: `$raw .= '#' . PVE::Tools::encode_text($cl) . "\n"`
  (upstream 2157–2161, hogan ~2118). They are read back with `decode_text`
  (upstream 2021–2024). `PVE::ParseUtils::encode_text` (`proxmox/pve-common`
  at `38395cd28456`) escapes "all control and hi-bit characters, ':' and '%'".
  A Notes line `public-ipv4: 203.0.113.7` is therefore stored as
  `#public-ipv4%3A 203.0.113.7`. The form `description: …` is accepted on read
  but never written.
- **Tags (`tags:`).** `PVE::JSONSchema` defines
  `$PVE_TAG_RE = qr/[a-z0-9_][a-z0-9_\-\+\.]*/i`, so there is no `:` or `=`.
  `.` is allowed, so `mds.zone.fra1` and even `mds.public-ipv4.203.0.113.7`
  are legal tags.
- **Who may set a tag.** `PVE/DataCenterConfig.pm` (`proxmox/pve-cluster` at
  `c8f55d92e1ef`) has two settings:
  - `user-tag-access.user-allow` (`none` / `list` / `existing` / `free`,
    default `free`) governs users without `Sys.Modify` on `/`.
  - `registered-tags` is "a list of tags that require a `Sys.Modify` on '/' to
    set and delete". It lists **exact tags**, with no prefix or pattern. An
    operator can therefore reserve `mds.zone.fra1` for administrators, but
    cannot reserve "every `mds.public-ipv4.*`".

### Evidence: topology today

- `GET /latest/meta-data/placement/availability-zone` is the fixed string
  `proxmox`. The identity document's `region` is `local` and its
  `availabilityZone` is `proxmox`. `getAvailabilityZone` and `getRegion` carry
  the same `TODO: Extract from Proxmox cluster config` that `getPublicIP` did.
  `instance-metadata` pins the constants.
- `placement/region` is not served at all.
- **The service does not know its own node.** `ParseVMConfigs` takes the first
  entry under `/etc/pve/nodes` that has a `qemu-server` directory. On hogan's
  three-node cluster (`hogan`, `holger`, `horst`) that is `hogan` only because
  it sorts first. Found by the coordinator, and part of its separate change.
  Proxmox's own pointer to the local node is the `/etc/pve/local` symlink.
  It was verified on hogan on 2026-10-17: it points to `nodes/hogan`, and
  `/etc/pve/local/qemu-server` exists (task 1.3).

### Evidence: what has already been rejected

`serve-a-public-address-or-omit-the-key`, `design.md`, options B and C.
`ipconfig0` is what cloud-init was told, often `dhcp`, and "not a statement
about a public address". The guest agent "returns all addresses with nothing
marking which is public" and "turns a metadata read into a synchronous call
into the guest". Both stay rejected here; see Non-Goals.

## Goals

- Choose a carrier **per category of data**, not per key.
- Keep Proxmox guest configs free of anything Proxmox would not write itself.
- Make the YAML inventory and the Proxmox backend give the same answer for the
  same declaration.
- Serve real topology where the host knows it, and a declared value where it
  does not.
- Keep the existing trust property: per-VM data comes from the host side, and
  never from the guest.

## Non-Goals

- **Observing a public address.** It cannot be done (see (c) in
  `proposal.md`), and #29 already rejected `ipconfig0` and the guest agent for
  that reason. Do not re-propose them.
- Fixing the section-blind parser and the node selection in
  `internal/proxmox`. Those are the coordinator's separate change. This record
  only depends on the node fix.
- Implementing anything before the maintainer decides.

## The decisions

### D1. One mechanism or two: labels vs. values

The coordinator's thesis: **labels** (zone, region, later rack or instance
type) are short, enumerable identifiers, which is what a tag is. **Values**
(addresses, IDs) have structure, and putting them in a tag name builds a small
language that is written but never read.

The tag permission model supports the split. `registered-tags` can reserve each
label value for administrators, but not an open-ended family of address tags.
With tags, a public address could be set by any user who may edit the VM's
options, unless `user-allow` is narrowed for everyone.

- **Two mechanisms:** labels in tags, values somewhere else (D2).
- **One mechanism:** everything in one place (Notes, or a file of our own).
  This costs the Proxmox-UI visibility that tags give labels.

### D2. Carrier for values (`public-ipv4`)

| Carrier | Proxmox-native | UI-visible | Validation | Cost |
|---|---|---|---|---|
| Notes | yes | yes (Notes) | log and ignore, since Notes are free text | decode `#` lines, a marker syntax, rules for duplicates and prose |
| Tags | yes | yes | log and ignore | a prefix convention, and cannot be reserved (see above) |
| Own file, keyed by VM ID | no, by design | no | fail to start, as with YAML | loader, merge, config key, packaging; drift when a VM ID is reused |
| YAML inventory field | n/a | n/a | fail to start (exists in `main`) | none; Proxmox hosts would also need a YAML overlay |
| None on Proxmox | — | — | — | the feature is unavailable on Proxmox hosts |

### D3. Carrier for labels (zone, region)

Tags are the natural fit, as the maintainer suggested. Open points:
- The prefix (`mds.zone.<x>`, `mds.region.<x>`).
- Whether to recommend `registered-tags` for them.
- What two zone tags on one VM mean: reject, or log and ignore.

YAML gets `zone` and `region` fields with the same meaning.

### D4. Derived defaults

- **Zone** defaults to the hosting node's name, or to its HA group.
- **Region** defaults to the cluster name.

A tag overrides, and so may a per-node mapping in `config.yaml`. The
precedence would be: VM declaration > per-node mapping > derived default >
the current constants. **Prerequisite:** the service must know its own node,
which is the coordinator's change. Without SDN, (b) has no derivable source;
where SDN IPAM exists, it could be added later.

### D5. Validation and failure

- **Files we own** (YAML, our own file) fail loudly at load, as #29 does.
- **Data in Proxmox's files** (tags, Notes) is logged and treated as absent.
  Failing the whole inventory over one VM's tag would deny metadata to every
  VM on the host. #29 already wrote this asymmetry down.

### D6. The consumer

To be verified against current Kubernetes documentation before designing
(task 1.2). The working assumption is that kubelet does not read metadata
services itself, since the in-tree cloud providers were removed. The labels
`topology.kubernetes.io/region` and `topology.kubernetes.io/zone` come either
from a cloud-controller-manager or from `--node-labels` at bootstrap, for
example a cloud-init script reading `placement/availability-zone` and
`placement/region`. If that holds, serving those two paths with real values
is all this service has to do.

## Interim step, under every decision

Stop honouring a hand-written `public-ipv4:` line in a Proxmox guest config
(task 2.1). It costs little, needs no decision, and removes the one behaviour
in `main` that works only until Proxmox next writes the file.

## Recommendation

**The decision is the maintainer's.** The two agents who explored this lean
towards:
- **two mechanisms**, labels in tags and values in a file we own, with the
  existing YAML fields as the non-Proxmox form;
- **derived defaults** for zone and region once the node fix lands;
- **the interim step now.**
