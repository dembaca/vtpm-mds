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
  **Since fixed in `main`** (2026-10-17, `read-guest-configs-from-this-node`, #40,
  archived in #41): the Proxmox backend no longer stores a `public-ipv4` key, in
  either the `:` or the `=` form, and reads only the live section of a config.

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
- **(Fixed in `main` on 2026-10-17 by `read-guest-configs-from-this-node`,
  #40/#41: the service reads `/etc/pve/local/qemu-server` and fails the load,
  naming that path, when it is missing. What follows is the state before.)**
  **The service did not know its own node.** `ParseVMConfigs` takes the first
  entry under `/etc/pve/nodes` that has a `qemu-server` directory. On hogan's
  three-node cluster (`hogan`, `holger`, `horst`) that is `hogan` only because
  it sorts first. Found by the coordinator, and part of its separate change.
  Proxmox's own pointer to the local node is the `/etc/pve/local` symlink.
  It was verified on hogan on 2026-10-17: it points to `nodes/hogan`, and
  `/etc/pve/local/qemu-server` exists (task 1.3).

### Evidence: the two sources the maintainer raised for an address

Both were raised on 2026-10-17.

- **A static address in cloud-init (`ipconfigN: ip=<addr>/<len>,gw=…`).**
  This is a real Proxmox key: it is in the schema, kept on every write, and set
  by the operator, not the guest. `internal/proxmox` already stores it as
  `RawConfig["ipconfig0"]`. But it is what the guest is *told* to configure
  on NIC N, which is meaning (b), not (c). It equals a public address only
  when that NIC is on the public network without NAT. It is also intent, not
  observation, and it is absent with `ip=dhcp`. **Measured on VM 399**
  by the coordinator on 2026-10-17. VM 399 *does* use cloud-init
  (`ide0: …vm-399-cloudinit.qcow2,media=cdrom`, `ciuser: ansible`), and its
  config shows both cases: `ipconfig0: ip=dhcp` (no address for the main NIC)
  and `ipconfig1: ip=169.254.169.10/16` (static, on the IMDS bridge). A static
  `ipconfigN` is therefore an operator declaration in a field that means
  "configure this on first boot", and it can go stale without anyone
  noticing. **Conclusion:** a sound Proxmox-native source for a (b) key where
  it exists. #29 rejected it for `public-ipv4` because (b) is not (c), not
  because the source is unsound.
- **The address the Proxmox UI shows.** The VM summary takes it from the
  QEMU guest agent (`network-get-interfaces`). Served by the MDS to the same
  guest, it would repeat what that guest reported about itself. The guest has
  that already, and the guest controls it, so it is worthless as something the
  service vouches for. Behind NAT the public address is not on the guest, so
  the UI cannot show it either. This is #29's option C, still rejected for
  anything the service vouches for. **Measured on VM 399:**
  `qm guest cmd 399 network-get-interfaces` reports `lo 127.0.0.1`,
  `eth0 10.7.10.21` (what the UI shows, and an RFC1918 address, so meaning b)
  and `eth1 169.254.169.10` (the IMDS bridge). Meaning (b) therefore has an
  observable value, but a guest-asserted one. Trusting it for anything in the
  identity document is the same class of mistake as honouring
  `X-Forwarded-For` (queue item 3) or a guest-chosen CN (queue item 2).

### Evidence: what has already been rejected

`serve-a-public-address-or-omit-the-key`, `design.md`, options B and C.
`ipconfig0` is what cloud-init was told, often `dhcp`, and "not a statement
about a public address". The guest agent "returns all addresses with nothing
marking which is public" and "turns a metadata read into a synchronous call
into the guest". Both stay rejected here; see Non-Goals.

## Maintainer's steer, 2026-10-17

`public-ipv4` is deferred, and is no longer the lead case. Region and
availability zone are the first keys; `public-ipv4` is the test case for a
*value*, and may follow later through the same carrier. The decisions below
still apply, but D2 (the carrier for values) is no longer urgent. D1, D3, D4
and D7 are what unlock region and zone.

## What SPIRE's `tpm_devid` attestor can use (read 2026-10-17)

Raised by the maintainer: more data in the certificate than `CN=<vmid>`, and
a more telling SPIFFE ID than one built on VM IDs. Read from the current SPIRE
documentation, not from memory. Sources are `spiffe/spire`:
`doc/plugin_server_nodeattestor_tpm_devid.md` (on `main` at `9dbd864e89`,
2026-10-08), the same file at tag `v1.15.4`, and
`doc/tpm_deployment_walkthrough.md` (`main` at `562b3f4d6d`).

- **The agent's SPIFFE ID is not configurable.** It is
  `spiffe://<trust_domain>/spire/agent/tpm_devid/<fingerprint>`, where the
  fingerprint is the SHA1 of the DER-encoded identity certificate. The
  walkthrough says so in as many words: "That identity is not configurable."
  Nothing in the certificate subject changes it. Even a re-issued certificate
  for the same VM gets a new agent ID.
- **What the attestor offers as selectors, in release `v1.15.4`:**
  `tpm_devid:subject:cn:<cn>`, `tpm_devid:issuer:cn:<cn>`, and
  `tpm_devid:fingerprint:<sha1>` for each certificate in the proof chain except
  the leaf. Nothing is read from a SAN, and no selector is listed for O or OU.
- **What `main` adds** (`tpm_devid: add subject attribute selectors by OID
  (#7312)`, 2026-10-08): `tpm_devid:subject:oid:<OID>:<value>` for any
  string-valued attribute of the subject, standard or custom, so O, OU and
  `serialNumber` become possible. **It is in no release tag.** `v1.15.4`
  (published 2026-10-15) does not contain it, and the commit is 82 commits
  ahead of that tag. It can only be relied on once a release ships it. The
  deployment walkthrough is also on `main` only.
- **A readable identity comes from a node alias, not from the certificate.**
  The walkthrough's "Node Aliases" section: `spire-server entry create
  -spiffeID spiffe://example.org/node/primary -node -selector
  tpm_devid:subject:cn:node-01.example.org` creates an additional identity,
  "shared by every node matching the given selectors", to use as the stable
  `parentID` of workload entries. Without an alias, workload entries must be
  parented to each node's fingerprint-based ID individually. This works today
  on `v1.15.4` with `CN=<vmid>` and needs **no change to this service**: the
  operator maps a VM ID to a readable alias in SPIRE.
- **The trust statement is SPIRE's own:** "The trusted DevID issuer must
  validate subject attribute claims before signing. Certificate validation and
  TPM proofs do not independently establish the accuracy or uniqueness of
  those claims." Here the issuer is this service's DevID CA, so any field it
  puts into a certificate is a claim this service vouches for.
- **Measured on the running server** (`spire-server agent show` for VM 399's
  agent, run by the maintainer as root on hogan, 2026-10-17):
  ```
  SPIFFE ID        : spiffe://bgl.dembach.org/spire/agent/tpm_devid/3a165dc4…f285
  Attestation type : tpm_devid
  Selectors        : tpm_devid:ca:fingerprint:d59ccc98b3d6025fa86e0ba1d5cd707479c0ed74
  Selectors        : tpm_devid:issuer:cn:BGL Proxmox DevID CA
  Selectors        : tpm_devid:subject:cn:vtpm-pilot
  ```
  Three things follow.
  1. The CA-chain selector is `tpm_devid:ca:fingerprint:<sha1>`, as the
     walkthrough says. The server plugin page's table, which writes
     `tpm_devid:fingerprint:`, is the one that disagrees with this server.
  2. **`subject:cn` is `vtpm-pilot`, which is VM 399's name, not `399`.** The
     living `devid-enrollment` spec says the subject is the bare VM ID, and the
     coordinator measured `CN=399` on a fresh enrollment on 2026-10-16 (Item 6,
     task 4.2). **Explained, by the coordinator on hogan on 2026-10-17:** the
     SPIRE agent on VM 399 uses `/var/lib/spire/agent/devid.crt.pem`, issued on
     2026-09-14 with `CN=vtpm-pilot` and SHA-1 `3A:16:5D:C4…F2:85`, which matches
     the agent ID. That is **before** `enforce-authenticated-ldevid-subject`
     (2026-10-03): the guest chose the CN in its CSR then, and since the fix the
     daemon sets it to the authenticated VM ID. VM 399 has `tpmstate0` only and
     no `unusedN`, so `fresh-tpm` never ran and the original vTPM and its blobs
     from 2026-09-14 are still in use. The verification runs never replaced the
     SPIRE files. The readable name seen today is therefore exactly the one that
     was closed as a security gap, because the guest could choose it. "Readable"
     and "authenticated by the host" can go together if the name comes from the
     host side (Proxmox's `name:` or tags) instead of the CSR. That is the core
     of D8. Any SPIRE registration that selects on `subject:cn:vtpm-pilot` stops
     matching once the VM is re-enrolled with `CN=399`, and a new certificate
     also gives the agent a new SPIFFE ID, because the ID is the certificate's
     fingerprint.
  3. This means D8 is not only about the future: a change to what the
     certificate says can orphan existing registration entries.
- **The deployed SPIRE is `1.14.0`** (`spire-server --version`, read by the
  coordinator on hogan on 2026-10-17). The server plugin page at tag `v1.14.0`
  lists the same three selector rows as `v1.15.4`: `subject:cn`, `issuer:cn`
  and the chain fingerprint, with no `subject:oid`. So on the deployed version
  only `subject:cn` is usable for a readable name, and `subject:oid` would need
  a release that does not exist yet, plus an upgrade from 1.14.
- **Not checked:** whether any registration entries or node aliases exist
  today (`spire-server entry show` needs the admin socket, which belongs to
  `spire`), and the attestor's source code.

### Option a, shown end to end in the cloud lab (2026-10-17)

The node-alias route (D8 option 1) was run by `agent-cursor-vtpm-22e0` on the
cloud lab with SPIRE `1.14.0` (`spire-server --version` → `1.14.0`, trust
domain `lab.local`, `tpm_devid` against the lab DevID and EK CAs), from
`origin/main` at `b77b259`, built from source rather than from a `.deb`. No
product code was changed.

1. A freshly attested agent gets selectors
   `tpm_devid:ca:fingerprint:ff923f7e…`, `tpm_devid:issuer:cn:… Lab DevID CA`
   and `tpm_devid:subject:cn:100`. `CN=100` is the inventory VM ID, as the
   enforced subject says it should be. The agent ID is
   `spiffe://lab.local/spire/agent/tpm_devid/b23124b6…ecde`.
2. A node alias was created with `entry create -node -spiffeID
   spiffe://lab.local/node/lab-guest -selector tpm_devid:subject:cn:100`, and a
   workload entry with `-parentID` on that alias and `-selector unix:uid:1000`.
   `spire-agent api fetch x509` in the guest returned
   `spiffe://lab.local/workload/lab-guest`.
3. **Counter-test, re-enrolling:** the agent ID changed from `b23124b6…` to
   `4e5306bf…`, the CN stayed `100`, the alias still matched, and the fetch
   returned the same workload ID.

So option 1 works on the deployed SPIRE version, needs no change to this
service, and survives re-enrollment, because the alias hangs on the CN and not
on the fingerprint. It does not by itself fix VM 399, whose certificate still
has the guest-chosen `CN=vtpm-pilot`.

Not checked: a `.deb` build, a real Proxmox host (the lab uses the YAML
inventory), and the agent ran in the guest over SSH and a 9p share.

**A side finding that is not part of this record.** In the same lab run, a
guest enrolled by the stock `devid-enroll` could not be loaded by SPIRE
`1.14.0`: it loads the DevID under `SRKTemplateHighRSA`, while `devid-enroll`
creates under `SRKTemplateRSA`, and `tpm2.Load` fails with `integrity check
failed (0x1f)`. That is its own defect, with its own change
(`create-devid-under-the-srk-spire-loads`), and it is the reason the lab run
needed a lab-only binary.

### D8. Where the readable name comes from

1. **A SPIRE node alias keyed on `CN=<vmid>`.** It needs nothing from us and
   works on the released version. The mapping from VM ID to name lives in
   SPIRE, maintained by whoever runs it. Today's selector is `subject:cn:
   vtpm-pilot`, so an alias would first have to be reconciled with the CN
   change above, and with the fact that this name was guest-chosen.
2. **A descriptive name in the certificate**, for example as `CN`. That changes
   `devid-enrollment`, whose archived change `enforce-authenticated-ldevid-
   subject` pins `CN=<vmid>`, and it breaks the existing registrations that
   select on `subject:cn`.
3. **`CN=<vmid>` stays, and a second subject attribute carries the name or
   labels** (for example in OU or a custom OID), matched through
   `subject:oid`. This needs a SPIRE release that contains #7312, and a
   subject carrying values from Proxmox tags.

**Decided by the maintainer on 2026-10-17: option 1 (a node alias keyed on
`CN=<vmid>`) now, and option 3 (the VM ID stays in the CN, a name or tags go in
a second subject attribute) noted for later.** The CN stays the VM ID: unique in
the cluster, settable only by whoever creates the VM, and no code. The readable
name lives in SPIRE as one node alias per VM, which the lab showed end to end,
re-enrollment included. Option 3 waits for a SPIRE release that contains
`subject:oid` (#7312, on `main` only; the deployed SPIRE is 1.14.0). Option 2, a
descriptive name as the CN, is not chosen. Proxmox VM names are not unique and
can be changed by anyone with `VM.Config.Options`.

One consequence stays: VM 399's SPIRE agent still presents a certificate with
`CN=vtpm-pilot`, so an alias keyed on `CN=399` only matches after VM 399 is
re-enrolled, which changes its agent ID. That is decided in
`create-devid-under-the-srk-spire-loads`, task 3.4, not here.

### D9. Trust of values that reach the certificate

A tag or a VM name is only as trustworthy as the Proxmox privilege needed to
change it, whereas a VM ID is assigned only by whoever creates VMs. Anything
from option 3 above is therefore a statement a lower-privileged operator can
influence. D7 applies: it must come from the operator or the host, never from
the guest, and the question is which Proxmox role is allowed to edit it.

**Researched 2026-10-18** from the Proxmox sources (assignment of the
coordinator, #33). The pages were read through a web fetch tool that summarises
a page, so the quoted code is what the tool returned for the named file at the
`HEAD` of the repository on that day, not a diff against a checkout; the
installed version on hogan was not compared (see "Not established").

Sources, all `https://git.proxmox.com/?p=<repo>.git;a=blob_plain;f=<file>;hb=HEAD`:
`pve-access-control` `src/PVE/AccessControl.pm` (roles and privilege groups),
`pve-guest-common` `src/PVE/GuestHelpers.pm` (tag permissions),
`pve-cluster` `src/PVE/DataCenterConfig.pm` (`registered-tags`,
`user-tag-access`, `tag-style`), `pve-common` `src/PVE/JSONSchema.pm` (tag
format), `qemu-server` `src/PVE/API2/Qemu.pm` (create and update permissions),
`pve-manager` `PVE/API2/NodeConfig.pm` (node configuration).

**What each source needs to change it**

| Source | Privilege to set or change it | Where |
|---|---|---|
| VM ID | `VM.Allocate` on `/vms/{vmid}` or on the VM pool `/pool/{pool}`, at creation | `Qemu.pm`, `create_vm` permissions text |
| VM name | `VM.Config.Options` (`name` is in that key list, with `description`, `onboot`, ...) | `Qemu.pm`, `$check_vm_modify_config_perm` |
| VM tag | `VM.Config.Options` on the VM, **and** the tag's own rule (below) | `Qemu.pm` (`tags` branch), `GuestHelpers.pm` `assert_tag_permissions` |
| Node description | `Sys.Modify` on `/` (`PUT` of the node config) | `NodeConfig.pm`, `check => ['perm', '/', ['Sys.Modify']]` |
| Node name, cluster name | root on the host (join or create a cluster) | not read from a source file; the host's own configuration |

**The tag rule.** `assert_tag_permissions` first requires `VM.Config.Options`,
then lets a user with `Sys.Modify` on `/` through unconditionally. Everyone else
is checked per tag that changes (added or removed): it raises
`/, Sys.Modify for modifying tag '<tag>'` when
`(!$allowed_tags->{$tag} && !$freeform) || $privileged_tags->{$tag}`.
`$privileged_tags` is the exact list in `datacenter.cfg` `registered-tags`, a
`;`-separated list of tags, matched **by hash lookup, not by prefix**.
`user-tag-access` has `user-allow` with `none`, `list`, `existing` or `free`,
and **the default is `free`**: without a setting, any user holding
`VM.Config.Options` may set any tag that is not registered. With `none`, no
user without `Sys.Modify` can change any tag at all.

**Who has these privileges in the standard roles**, from `create_roles` in
`AccessControl.pm` (admin, user and audit groups feed `PVE<cat>Admin` and
`PVEAdmin`; the `root` group is only in `Administrator`):

| Role | `VM.Allocate` | `VM.Config.Options` | `Sys.Modify` |
|---|---|---|---|
| `Administrator` | yes | yes | yes |
| `PVEAdmin` | yes | yes | **no** (`Sys.Modify` is in the `root` group of `Sys`) |
| `PVEVMAdmin` | yes | yes | no |
| `PVEVMUser` | no | no | no |
| `PVESysAdmin` | no | no | **no** (gets only `Sys.Console`, `Sys.Syslog`, `Sys.Audit`) |
| `PVEAuditor` | no | no | no |

So among the standard roles only `Administrator` holds `Sys.Modify`, and a
holder of `PVEVMAdmin` or `PVEAdmin` can change the VM name and, with the default
`free`, set any unregistered tag such as `mds.zone.<anything>`.

**Confirmed on the installed version** (coordinator, hogan, 2026-10-18,
`pve-manager 9.2.3`, `qemu-server 9.1.16`, `libpve-access-control 9.1.1`, source
under `/usr/share/perl5/PVE` read without root): the same privileges for the
VM ID (`VM.Allocate` on `/vms/{vmid}`, `API2/Qemu.pm` around line 2736), the VM
name (`VM.Config.Options` through `$generaloptions`, around lines 796 and 983),
tags (`VM.Config.Options` plus `assert_tag_permissions`) and the node
description (`PUT /nodes/{node}/config` checks `Sys.Modify` on `/`,
`API2/NodeConfig.pm` around line 81, not on the node). One consequence the
coordinator drew: `VM.Config.Options` granted on a single VM allows renaming and
tagging it, but never a new VM ID, so per-VM ACLs can separate the ID from the
name and tags even though the standard roles do not.

**Matrix, source by purpose.** *Read* is serving the value through the metadata
service to the guest itself; *certificate* is a value that ends up in the
LDevID and is matched by a SPIRE selector.

| Source | Who changes it (standard roles) | Read (IMDS) | Certificate |
|---|---|---|---|
| VM ID | `VM.Allocate`: `PVEVMAdmin`, `PVEAdmin`, `Administrator`; fixed after creation | yes | yes (D8 a, today's CN) |
| VM name | `VM.Config.Options`: `PVEVMAdmin`, `PVEAdmin`, `Administrator`; no way to restrict it apart from the tags | yes | **no** |
| VM tag `mds.*`, default settings | same, and anyone with `VM.Config.Options` may set it | yes, with the caveat below | **no** |
| VM tag, `user-allow: none` | only `Sys.Modify` on `/`, so `Administrator` | yes | acceptable, at the price of all user tags |
| VM tag listed in `registered-tags` | only `Sys.Modify` on `/`, **for exactly the listed tags** | yes | acceptable for a finite known set |
| Node description | `Sys.Modify` on `/`, so `Administrator` (one path for all nodes) | yes | acceptable |
| Node name, cluster name | root on the host | yes | acceptable |
| YAML inventory | whoever writes the service's file as root | yes | acceptable |

**Findings that change the earlier advice.**

1. **M2's hint does not work for `mds.zone.<name>`.** M2 advised to allow these
   tags only to admins "through `registered-tags`". That list is exact tags, so
   every possible `mds.zone.<name>` and `mds.region.<name>` would have to be
   listed one by one; there is no prefix rule. For a fixed set of zones this is
   practicable, for free-form names it is not. The blanket alternative is
   `user-allow: none`, which also stops users from setting their own harmless
   tags.
2. **The node description is the strongest of the label sources**, and the
   privilege is the same as for a registered tag (`Sys.Modify` on `/`), not a
   weaker one, but it is set once per node rather than per VM. It cannot tell two
   VMs on one node apart, which is why it serves as a default and not as a carrier
   for per-VM values.
3. **`/` is cluster-wide.** The node configuration is protected by `Sys.Modify`
   on `/`, not on `/nodes/<node>`; a holder can edit the description of every node.
4. **A certificate is a snapshot.** Name and tags can change after enrollment;
   the metadata service answers with the current value, the certificate does not
   change until the VM enrolls again. Whatever lands in a certificate must be a
   value that does not change for the life of the VM, or the mismatch is a defect
   of its own. The VM ID is the only one that qualifies; D8 a already keeps the
   certificate to it.

**Recommendation.**

- **Read through the metadata service:** keep the order decided in M3 and the
  later decision (tag, node description, node and cluster name). The tag is the
  weakest of the three but it is what the maintainer chose for per-VM values, and
  the guest only reads it. Document that with the default `user-tag-access` a VM
  administrator sets any zone, and that `user-allow: none` or `registered-tags`
  closes it where that matters. No code change.
- **Into the certificate:** no VM name, no tag, no zone or region, with the
  default settings. Where a label has to reach a certificate, take it from the
  node description or a registered tag (both `Sys.Modify` on `/`) or from a file
  the operator owns (D2), and expect the staleness in finding 4. D8 option 3
  (name or tags as an extra subject attribute) stays out until that is settled.
- **Left open, needs the real stack:** whether the operator wants
  `registered-tags` or `user-allow: none` on a data-centre stack. That needs the
  real `datacenter.cfg` and roles of that stack.

**Not established.**

- The tag format. In `JSONSchema.pm` at `HEAD` it is
  `[a-z0-9_][a-z0-9_\-\+\.]*` (so `-` is allowed), yet hogan refused a tag with
  `-` on 2026-10-17 (`invalid characters in tag`). The installed privileges
  agree with `HEAD` (above), so the roles part stands; the installed
  `PVE_TAG_RE` was not read and the refusal is unexplained.
- The real settings on hogan: `datacenter.cfg` (`registered-tags`,
  `user-tag-access`) and `user.cfg` need root; the coordinator asked the
  maintainer for the lines.
- Pool roles and ACL inheritance down `/pool/{pool}` and `/vms/{vmid}`
  (`PVEPoolAdmin`, propagation), and the check that a chosen VM ID is free.
- The VM name's format check, and what `qm` does as root (root is not subject to
  these checks).

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
| Own file, keyed by VM ID | no, by design | no | rejected at load (the daemon still starts, see D5) | loader, merge, config key, packaging; drift when a VM ID is reused |
| YAML inventory field | n/a | n/a | rejected at load (exists in `main`; the daemon still starts, see D5) | none; Proxmox hosts would also need a YAML overlay |
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

- **Files we own** (YAML, our own file) are rejected as a whole at load: the
  load returns an error and produces no inventory.
  **What the daemon then does is not "refuse to start".** Measured on the lab
  daemon built from `main@2dd0335` (cloud lab agent, 2026-10-17): a malformed
  `public_ipv4` logs `Failed to load VM configs … is not an IPv4 address` and
  `Warning: Failed to load VM config cache on startup`, then the service starts
  its listener and answers `/health` with 200, running with an empty cache.
  That is what `vm-inventory`, "Load The Cache At Startup And Refresh It On
  SIGHUP", specifies: a startup load failure "SHALL NOT prevent the service
  from starting; the service then runs with an empty cache, so every caller is
  unbound". Unbound callers are refused under the default
  `require_vm_identity: true`, so the effect is closed but not loud. An earlier
  version of this record, and #29's tasks, assumed the service would stop.
  Whether a bad inventory should stop the service is its own decision, with
  its own change, and not part of this record.

  **The combination is the problem, not either part.** #29's delta makes a
  malformed `public_ipv4` fail the *whole* load, and the startup rule lets the
  service run on with an empty cache. Together, a typo in one optional field of
  one VM leaves **every** VM unbound and every identity-bearing path refused,
  while `/health` still answers 200. That is the "broken but reported healthy"
  state `fail-fast-on-unusable-configuration` was written against. The
  coordinator named it on 2026-10-17. The choice to fail the whole load was
  made in #29, not required by the spec. Two ways out, neither decided here:
  1. **Drop only the bad field**, log it loudly, and keep the rest of the
     entry and every other VM. Availability over strictness.
  2. **Refuse to start** on any inventory error. That also changes the
     existing startup requirement.

  The YAML inventory is the lab's backend; hogan reads Proxmox. The damage is
  therefore confined to the lab today, which argues for settling it with the
  rest of this category instead of squeezing it in now.
- **Data in Proxmox's files** (tags, Notes) is logged and treated as absent.
  Failing the whole inventory over one VM's tag would deny metadata to every
  VM on the host. #29 already wrote this asymmetry down.

### D7. Trust source per key

This was raised by the coordinator after the measurements above. The question
is no longer only *where* a value is stored, but *whose statement* it is.
There are three possible sources:
- **the operator**, through a declaration;
- **the host**, by derivation, as with the node and the cluster;
- **the guest**, through the agent.

The answer may differ per key. If a guest-asserted value is served at all,
for example the guest's own view of its network address (b), it must say so
in its **name**, not only in this document. It must never appear in the
instance identity document. The default for every key is "operator or host,
never guest".

### D6. The consumer

**Researched on 2026-10-17** by `agent-mac-vtpm-mds-687cc7` at the coordinator's
assignment, from the pages named below. The working assumption held in its main
point, with one correction and several things not read.

**Findings.**
- **Core Kubernetes reads no metadata service for zone or region.** The
  Kubernetes page on well-known labels says of `topology.kubernetes.io/zone`:
  "On Node: The `kubelet` or the external `cloud-controller-manager` populates
  this with the information from the cloud provider. This will be set only if you
  are using a cloud provider. However, you can consider setting this on nodes if
  it makes sense in your topology."
  (<https://kubernetes.io/docs/reference/labels-annotations-taints/>)
- **The in-tree providers are gone.** From Kubernetes 1.31 the kubelet and
  kube-controller-manager accept only an empty `--cloud-provider` or `external`;
  a kubelet in `external` mode registers a Node with the taint
  `node.cloudprovider.kubernetes.io/uninitialized`, and the provider's
  cloud-controller-manager sets provider ID, topology labels and addresses and
  removes it. The compiled-in AWS and OpenStack integrations went in 1.27 and
  1.26. Source for this paragraph: a web search of 2026-10-17 returning a
  DevClass report on 1.31 and Kubernetes' own blog post "cloud-provider
  integration changes" (<https://v1-33.docs.kubernetes.io/blog/2023/12/14/cloud-provider-integration-changes>);
  the pages were not opened in full.
- **The cloud-controller-manager's node controller** annotates and labels the
  Node "with cloud-specific information, such as the region the node is deployed
  into" (<https://kubernetes.io/docs/concepts/architecture/cloud-controller/>).
  That page does not say where a given provider gets the value.
- **The AWS provider takes the zone from the EC2 API, not from the metadata
  service** — the correction. In `cloud-provider-aws`
  (`pkg/providers/v1/aws.go`, `master`, read 2026-10-17), `GetZone` returns the
  availability zone cached from `DescribeInstances`; the metadata service
  supplies the instance ID, and the region is resolved at start-up through
  `getRegionFromMetadata`. **Which metadata path that function reads is not
  established**: the file was fetched through a summarising tool, truncated at
  100,000 characters, and the function body was not in the part read.
- **The EBS CSI driver's node plugin does read the metadata service**, falling
  back to the Kubernetes API when it cannot, and adds its own
  `topology.ebs.csi.aws.com/zone` label. Seen only in search-result summaries
  (a driver pull request and an AWS knowledge-center page), not in the driver's
  source.
- **A Proxmox cloud-controller-manager exists and does not use the metadata
  service.** `sergelogvinov/proxmox-cloud-controller-manager` says: "kubernetes
  label `topology.kubernetes.io/region` is a Proxmox cluster
  `clusters[].region`" and "kubernetes label `topology.kubernetes.io/zone` is a
  hypervisor host machine name", both obtained through the Proxmox API
  (<https://github.com/sergelogvinov/proxmox-cloud-controller-manager>). Its
  zone is the node name, its region a configured cluster name, which is what
  D4's derived defaults propose.

**What it means for this service.**
- **Nothing in Kubernetes core reads `placement/availability-zone` or
  `placement/region` from an IMDS.** Whatever fills those two paths is read by
  software that speaks the EC2 metadata shape: AWS-derived components, cloud-init
  datasource code, and scripts an operator writes, for instance a bootstrap that
  passes `--node-labels`. Whether the kubelet accepts `topology.kubernetes.io/*`
  in `--node-labels` was **not verified here**.
- **So serving real values at those two paths is enough for any consumer of the
  EC2 shape**, and the maintainer's steer, region and zone first, needs nothing
  more from Kubernetes. A Kubernetes node on these guests gets its topology
  labels either from a script reading the paths, or from the Proxmox CCM, which
  would ignore this service altogether.
- **Not checked:** which of the AWS-derived components a deployment will really
  run, the kubelet's `--node-labels` allow-list, k3s and RKE2 behaviour, and the
  metadata path in `getRegionFromMetadata`. If a consumer needs anything beyond
  the two paths, the scope changes before any delta is written.

## Decided, 2026-10-17 (M1 to M5)

**By the maintainer**, on the agent bus, as "M1–M5 alle wie empfohlen", answering
the coordinator's decision sheet. The sheet's labels map to the decisions above:

| Sheet | Decision | Outcome |
|---|---|---|
| M1 | D1, one mechanism or two | **Two.** Labels (zone, region, later others) are Proxmox tags; values such as addresses go to their own file when D2 is taken up. |
| M2 | D3, carrier for labels | **Tags** `mds.zone.<name>` and `mds.region.<name>` (Proxmox tags allow `.`, not `=` or `:`). Two zone tags on one VM: log and ignore both, so the default applies. Operators are advised to allow these tags only to admins through `registered-tags` (`Sys.Modify`). |
| M3 | D4, derived defaults | **Zone is the node name, region the cluster name.** Precedence: VM tag over default. |
| M4 | D7, who may declare | **Operator (tag or file) or host (derivation), never the guest.** |
| M5 | D5, a broken or unknown tag | **Log it and treat it as not set;** every other VM is untouched. The YAML inventory gets `zone` and `region` with the same meaning. |

**Still open:** D2 (carrier for values, `public-ipv4`), deferred by the maintainer;
D9 (trust of values that reach the certificate; researched 2026-10-18, see D9, decision open). D6 is researched above, not a
decision; D8 is decided above.

**Where it goes next:** the change `serve-zone-and-region-per-vm` (#44) carries
M1 to M5 into deltas and tasks. This exploration stays as the evidence. The
question why a typo in the YAML inventory empties the whole inventory is not part
of it and becomes its own change.

## Interim step, under every decision

**Done in `main` on 2026-10-17** by `read-guest-configs-from-this-node` (#40, archived
in #41); kept below for the reasoning.

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
