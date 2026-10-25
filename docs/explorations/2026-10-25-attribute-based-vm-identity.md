# Exploration: attribute-based VM identity

Status: draft for the maintainer, 2026-10-25. Not a change record. Nothing here
is decided; the last sections propose a direction and list the decisions.

Goal, from the maintainer (2026-10-25, translated):

> I don't want to attach access policies to a SPIFFE ID that contains only the
> VMID; that does not scale. I want identities that contain attributes which
> can be supplied from the hypervisor side, and from which role or group
> relationships of a VM follow. That they are cryptographically bound to a
> specific VMID at runtime is fine. But when an EK cert is reissued and the
> VMID is possibly reused, these relationships are created anew.
>
> Free VM tags are fine too: the VM owner SHALL be able to grant the VM rights.
> Whoever controls the VM decides about its access.

Sources read: `internal/devid/ca.go`, `internal/proxmox/vmconfig.go`,
`openspec/specs/devid-enrollment/spec.md`, the archives named below, issue #89,
PR #90 (`design.md` on branch `read-cluster-ek-pins-from-pmxcfs`), the closed
exploration #64; SPIRE tags `v1.14.9`, `v1.15.5` and `main` on GitHub;
`spire-plugin-sdk`; the SPIFFE-ID standard; `pve-common`, `pve-guest-common`,
`pve-access-control`, `pve-manager` from git.proxmox.com (branch `HEAD`, read
2026-10-25, no commit pinned); the Proxmox `datacenter.cfg(5)` page. Items
marked **[unverified]** were not checked against a primary source or a running
system. Nothing in this document was run in a lab or on hogan.

## Where things stand

| Fact | Where |
|---|---|
| The LDevID subject is `CN=<vmid>` and nothing else; guest-supplied RDNs are dropped | `internal/devid/ca.go:179`; spec `:779` ("Issue A Time-Limited LDevID Whose Subject Is Only The VM ID"); archive `2026-10-19-issue-the-ldevid-subject-from-the-vm-id-only` |
| The SAN carries the SHA-256 of the EK (PermanentIdentifier) | `ca.go:181-183` (`buildDevIDSAN`) |
| Tags `vtpm_mds.io.<key>.<value>` are served only under `/latest/meta-data/tags/instance/<key>` | `internal/proxmox/vmconfig.go:106-158`; archive `2026-10-18-serve-vm-tags-as-instance-metadata` |
| Answer 4 of 2026-10-18: "Nothing into the certificate; the identity document does not carry tags." **This exploration revisits it.** | same archive, `design.md:34`, trust section `:82-93` |
| SPIRE registrations and node aliases select `tpm_devid:subject:cn:<vmid>` AND `tpm_devid:ca:fingerprint:<sha1 of the cluster intermediate>` | archive `2026-10-20-issue-ldevids-from-an-intermediate-per-cluster`, `design.md:38-41` |
| LDevID lifetime 90 days (`mds.devid_lifetime`); SPIRE checks the LDevID only at node attestation; a renewal changes the agent ID | archive `2026-10-20-limit-the-ldevid-lifetime-and-renew-it`; `AGENTS.md` |
| VM-ID reuse leaves the predecessor's agent attested under the same `cn:<vmid>`; eight agents under `cn:399` on 2026-10-25 | issue #89; PR #90 `design.md`, D-i |
| Cluster EK pins in `/etc/pve/vtpm-ek/<vmid>`, written only by a root hook (D-a); pins of destroyed VMs stop refusing (D-g, accepted 2026-10-25); destroying a VM deletes nothing (D-i) | PR #90 `design.md`, D-a, D-g, D-i (open PR, not merged) |

PR #90's D-g was accepted on the argument that "access policies will name
roles or groups rather than a VM ID". Its own D-i adds: "The roles argument
above holds only once role assignment follows the living VM (for example its
tags) rather than `cn:<vmid>`." This exploration is about that condition.

## 1. What SPIRE's `tpm_devid` emits

**Released (v1.14.9, and unchanged in v1.15.5).**
[`pkg/server/plugin/nodeattestor/tpmdevid/selectors.go#L9-L38` at v1.14.9](https://github.com/spiffe/spire/blob/dbfbd97c343cc93b73731c804e0c746160588ddf/pkg/server/plugin/nodeattestor/tpmdevid/selectors.go#L9-L38):

```go
if leaf.Subject.CommonName != "" {
    selectorValues = append(selectorValues, "subject:cn:"+leaf.Subject.CommonName)
}
if leaf.Issuer.CommonName != "" {
    selectorValues = append(selectorValues, "issuer:cn:"+leaf.Issuer.CommonName)
}
// for every cert of every verified chain except the leaf:
selectorValues = append(selectorValues, "ca:fingerprint:"+fp) // fp = hex SHA-1 of the DER
```

So the complete list, with the plugin prefix, is:

- `tpm_devid:subject:cn:<leaf CN>`
- `tpm_devid:issuer:cn:<leaf issuer CN>`
- `tpm_devid:ca:fingerprint:<sha1>` for each non-leaf certificate

The
[v1.14.9 server doc](https://github.com/spiffe/spire/blob/dbfbd97c343cc93b73731c804e0c746160588ddf/doc/plugin_server_nodeattestor_tpm_devid.md#selectors)
names the third one `tpm_devid:fingerprint:<sha1>`. The code emits
`ca:fingerprint:`, which is what this repository already uses. The doc is
wrong, not the archive.

`selectors.go` at `v1.15.5` (2026-10-20) has no OID handling (`grep -i oid`
finds only an unrelated comment). The agent ID stays
`spiffe://<td>/spire/agent/tpm_devid/<SHA-1 of the leaf DER>` (same doc,
"SPIFFE ID").

**Unreleased.** spiffe/spire#7312, merged into `main` on 2026-10-08 as
[`9dbd864`](https://github.com/spiffe/spire/blob/9dbd864e89239e1228246be486874eea69d3c81f/pkg/server/plugin/nodeattestor/tpmdevid/selectors.go),
adds `tpm_devid:subject:oid:<OID>:<value>` for every non-empty string-valued
subject attribute, standard or custom. Values keep case and whitespace;
repeated attributes with distinct values give separate selectors; a selector
longer than 255 characters (including `subject:oid:<OID>:`) is skipped with a
warning, because "MySQL stores selector values in varchar(255) columns". Its
doc warns: "The trusted DevID issuer must validate subject attribute claims
before signing." `compare v1.15.5...9dbd864` reports `diverged`, so no
released tag contains it as of 2026-10-25. Which release will is
**[unverified]**.

**Consequence.** With any released SPIRE, the only certificate field that
reaches a selector and that `vtpm-mds` controls per VM is the subject CN. An
attribute in an extension, a SAN or a non-CN RDN is invisible to SPIRE without
a custom plugin. Packing attributes into the CN breaks D8 and every existing
alias, and gives one selector, not one per attribute.

**Re-attestation refreshes selectors.** On each `AttestAgent` the server
replaces the node's selectors
([`pkg/server/api/agent/v1/service.go#L366` at v1.14.9](https://github.com/spiffe/spire/blob/dbfbd97c343cc93b73731c804e0c746160588ddf/pkg/server/api/agent/v1/service.go#L366)).
`tpm_devid` is reattestable; #64 found the agent re-attests at each agent SVID
rotation. The agent reads the DevID files only at plugin configure, so a new
certificate needs a `spire-agent` restart (#64; `devid-enroll -spire-data`
does that). **[unverified in a lab for attribute changes]**

## 2. Extension points SPIRE 1.14 still has

- **NodeResolver is gone.** Deprecated in 1.4.0 (#3272), removed in 1.5.0
  ("NodeResolver plugin type and `azure_msi` builtin NodeResolver plugin
  (#3470)"), per `CHANGELOG.md` at v1.14.9. The server plugin types at v1.14.9
  are `bundlepublisher`, `credentialcomposer`, `keymanager`, `nodeattestor`,
  `notifier`, `upstreamauthority` (`pkg/server/plugin/`).
- **NodeAttestor (external).** Returns `spiffe_id`, `selector_values` and
  `can_reattest`
  ([`nodeattestor.proto`](https://github.com/spiffe/spire-plugin-sdk/blob/main/proto/spire/plugin/server/nodeattestor/v1/nodeattestor.proto)).
  An external plugin can carry any selectors. The catalog refuses only an
  external override of `join_token`
  ([`pkg/server/catalog/catalog.go#L143-L144` at v1.14.9](https://github.com/spiffe/spire/blob/dbfbd97c343cc93b73731c804e0c746160588ddf/pkg/server/catalog/catalog.go#L143-L144)),
  which suggests an external plugin may be named `tpm_devid` and replace the
  builtin, so the agent side stays untouched. **[unverified: not run]**
- **CredentialComposer.** Composes CA, server, agent and workload SVIDs; no RPC
  returns selectors
  ([`credentialcomposer.proto`](https://github.com/spiffe/spire-plugin-sdk/blob/main/proto/spire/plugin/server/credentialcomposer/v1/credentialcomposer.proto)),
  so it cannot change which entries a node matches.
- **Entry API.** Node aliases and registration entries can be written by any
  client with admin rights on the server API. An entry's selectors are ANDed.

## 3. Options

Attributes come from the hypervisor: per VM its tags (owner) and pool
(admin, see 4); per cluster the intermediate (`ca:fingerprint`).

### (a) Attributes in the LDevID certificate

Put the scope and the tags as custom subject RDNs, one attribute per tag, under
an OID arc of this project (none exists yet, see decisions). Example selectors
once #7312 ships: `tpm_devid:subject:oid:<arc>.1:<pool>`,
`tpm_devid:subject:oid:<arc>.2:role=web`.

- **For.** No server-side component; the certificate is the whole truth;
  works offline from Proxmox at attestation time.
- **Against.**
  - Needs an unreleased SPIRE (#7312), or a custom plugin anyway.
  - An attribute change takes effect only on reissue. `devid-enroll` renews
    only when the certificate is missing, unreadable, expired or in its last
    third. A change trigger is needed (for example an attribute digest served
    by the IMDS and compared by the guest timer).
  - **Removal is not enforceable before `notAfter`.** The guest decides when
    it renews. A VM whose tag or pool was taken away keeps the old certificate,
    and the old relationships, for up to 90 days. Only a ban per agent ID
    stops it.
  - Every reissue is a new agent ID (#64); stale agents pile up (#89).
  - Reverses answer 4 of 2026-10-18 and reopens the subject requirement.
- **VM-ID reuse.** The new VM gets its own attributes in its own certificate.
  The old certificate keeps the old ones; it is usable only where its TPM
  state survives (backup, `unusedN`, clone). With an incarnation attribute
  (EK hash, for example `serialNumber`, OID 2.5.4.5) a policy could also
  exclude it.
- **Stale agents.** Unchanged: evict or ban by hand, or the #89 reconciler.

### (b1) Server side: a node attestor wrapping `tpm_devid`

An external plugin runs the `tpm_devid` verification (Apache-2.0 code, or by
delegation), then looks up the VM by cluster (intermediate fingerprint), VM ID
(CN) and EK hash (SAN), and emits extra selectors such as
`tpm_devid:vm:incarnation:<ek sha256>`, `tpm_devid:pve:pool:<pool>`,
`tpm_devid:pve:tag:<key>:<value>`.

- **For.** Attributes are as fresh as the last re-attestation (one agent SVID
  rotation). No certificate change, no guest change. It can **refuse**
  attestation when the EK hash is not the VM's current cluster pin, which
  closes #89 at the source: an old incarnation cannot attest.
- **Against.** Custom code inside the SPIRE trust path, maintained against the
  plugin SDK. The SPIRE server needs read access to every cluster's Proxmox API
  (or to the IMDS, which listens only on `169.254.169.1`). Attestation fails
  when that lookup fails. Whether the plugin can keep the name `tpm_devid` is
  **[unverified]**.
- **VM-ID reuse.** Relationships are recomputed at every attestation from the
  living VM; the old incarnation is refused.
- **Stale agents.** Still listed until pruned (`prune_attested_nodes_expired_for`,
  reattestable nodes only), but cannot re-attest.

### (b2) Server side: an external reconciler

A process outside SPIRE reads the Proxmox API (VMs, tags, pools) and writes
node aliases through the SPIRE server API. For each VM and each grant it
writes an alias whose selectors are the released ones,
`subject:cn:<vmid>` AND `ca:fingerprint:<intermediate>`, and whose SPIFFE ID
names the scope and the attribute (shape in 4). The same process is the #89
reconciler: it evicts or bans agents of destroyed VMs and of replaced EKs.

- **For.** Works with SPIRE 1.14 as deployed. No plugin in the trust path. A
  tag removal or pool change takes effect within one reconcile interval plus
  the agent's entry sync, without a reissue. Already planned for #89.
- **Against.** The binding is still `cn:<vmid>`: between a reuse and the next
  reconcile, and for any revived old TPM state, the old certificate matches
  the new VM's aliases. Separating incarnations needs the reconciler to know
  which agent belongs to which EK. Today the attested node record does not
  carry the EK hash **[unverified: not checked in `ListAgents` output]**; the
  `devid issued serial=… vm=… ek=…` line does not carry the leaf's SHA-1, which
  is the agent ID. Logging it is a small change in this repository.
- **VM-ID reuse.** Aliases follow the VM's current tags; after the reconciler
  has banned the predecessor's agent, nothing of the old VM remains.
- **Stale agents.** Banned by agent ID: every agent under `cn:<vmid>` whose
  DevID fingerprint is not the latest one issued to a live VM.

### (c) Hybrid

Slow, admin-controlled facts in the certificate; fast, owner-controlled facts
on the server.

- Certificate (once a release with #7312 is deployed): pool and EK hash as
  subject attributes. Both change rarely; a change of either means reissue.
- Reconciler: free tags to aliases, scoped by the pool selector, plus the #89
  bans. A pool change waits for reissue; a tag change does not.

| | Works on SPIRE 1.14 | Tag removal effective | Reuse/revived TPM | Code in trust path |
|---|---|---|---|---|
| (a) cert only | no (#7312) | at `notAfter` (≤ 90 d) | with EK attribute | none |
| (b1) attestor | yes | next re-attestation | refused | plugin |
| (b2) reconciler | yes | next reconcile | after ban | none |
| (c) hybrid | partly | next reconcile | with EK attribute | none |

## 4. Trust scoping of free tags

**Who can set a tag.** `assert_tag_permissions` first requires
`VM.Config.Options` on the VM, then applies the datacenter options
(`PVE/GuestHelpers.pm` in `pve-guest-common`, `HEAD`, lines 267-321):

- `user-tag-access: user-allow=` `free` (default), `existing`, `list`, `none`.
  "Users with the Sys.Modify privilege on / are always unrestricted"
  ([datacenter.cfg(5)](https://pve.proxmox.com/pve-docs/datacenter.cfg.5.html)).
- `registered-tags`: "A list of tags that require a Sys.Modify on / to set and
  delete." It protects only the tags listed exactly (D9, #33), so it cannot
  reserve a prefix such as `vtpm_mds.io.*`. **[unverified for wildcards: the
  code compares whole tags]**

So under the default, anyone with `VM.Config.Options` (in `PVEVMAdmin`) sets
any `vtpm_mds.io.<key>.<value>` tag. That matches the maintainer's goal: the
VM owner grants the VM rights. It also means **a tag alone is worth only as
much as the VM owner**. `role=db-admin` on a VM someone created in a sandbox
means nothing.

**Admin-controlled scopes available.**

| Scope | Set by | Reaches SPIRE today |
|---|---|---|
| Cluster | Whoever installs the intermediate | `tpm_devid:ca:fingerprint:<intermediate>` |
| Pool | `Pool.Allocate` on `/pool/<p>` (and on the old pool), plus `Permissions.Modify` or `VM.Allocate` on `/vms/<id>` (`PVE/API2/Pool.pm` lines 166, 333-349; `RPCEnvironment.pm` 442-456) | no; via (b) or later (a) |
| Registered tag | `Sys.Modify` on `/` | no; via (b) or later (a) |
| Node description | `Sys.Modify` on `/` | no (node-wide, not per VM) |

A pool fits best: it is per VM, it is what Proxmox already uses to delegate VM
administration, and moving a VM between pools needs rights on both pools.
Pool membership lives in `/etc/pve/user.cfg` (`pve-access-control`
`AccessControl.pm:1695`), not in the guest config, so `vtpm-mds` does not read
it today. Whether `www-data` may read `user.cfg` is **[unverified]**. That a
VM is in at most one pool is **[unverified]**.

**Rule proposed.** No grant on a tag alone. Every grant names the scope and the
tag, and the scope is part of the SPIFFE ID, so a downstream policy cannot
forget it:

```
spiffe://<td>/pve/<cluster>/pool/<pool>/tag/<key>/<value>   # per grant
spiffe://<td>/pve/<cluster>/pool/<pool>                     # scope only
spiffe://<td>/pve/<cluster>/vm/<vmid>                       # D8, per VM
```

Selectors for an alias under (b2), today:
`tpm_devid:subject:cn:<vmid>` + `tpm_devid:ca:fingerprint:<intermediate>`,
one alias per (VM, grant), written by the reconciler. Under (b1) or (c) the
alias is per grant, not per VM:
`<scope selector>:<pool>` + `<tag selector>:<key>:<value>` +
`tpm_devid:ca:fingerprint:<intermediate>`. A VM without a pool gets only the
per-VM ID; its tags grant nothing. Whether "no pool" should mean a default
scope is a decision.

## 5. VM-ID reuse with PR #90

- **D-a** (only a root hook writes the cluster pin, and replaces it on a vTPM's
  first initialisation): the pin always names the **current** incarnation of a
  VM ID. That is the fact (b1) checks at attestation and (b2) uses to decide
  which agent to ban.
- **D-g** (pins of destroyed VMs refuse nobody): a backup restored under a new
  ID enrols under the new ID. With attribute-based identity it gets the new
  VM's pool and tags, so it inherits nothing from the old VM by name.
  Its old certificate, still valid up to 90 days, carries `CN=<old vmid>` and
  matches whatever VM now holds the old ID. Only (b1) refuses it outright
  (its EK is not the old ID's current pin); (b2) bans it once seen; (a) and (c)
  exclude it only through an EK attribute in policies.
- **D-i** (destroying a VM deletes nothing): unchanged. The predecessor's agent
  stays until a reconciler or an operator removes it. Attribute-based
  identities make the reuse **less harmful**, because grants follow the
  living VM's pool and tags, but the per-VM ID `…/vm/<vmid>` and anything
  registered under it are still shared until the old agent is banned.

What remains in every option: the window before the first ban or re-attestation;
entries that people register by hand under `cn:<vmid>`; and any node of a
cluster can still issue for any VM of that cluster (#72).

## 6. Encoding limits

- **Proxmox tags.** `PVE_TAG_RE = qr/[a-z0-9_][a-z0-9_\-\+\.]*/i`
  (`pve-common` `src/PVE/JSONSchema.pm:890`, `HEAD`). No `/`, `:`, `=` or space.
  Case-insensitive in Proxmox (comment at `vmconfig.go:107`); `vtpm-mds`
  keeps the source's case for metadata. A length limit was not found
  **[unverified]**.
- **SPIFFE IDs.** "Individual path segments MUST contain only letters, numbers,
  dots, dashes, and underscores ([a-zA-Z0-9.-_])"
  ([SPIFFE-ID.md:82](https://github.com/spiffe/spiffe/blob/49a9cbbc018c39ec6643ad3301632f9f4549258b/standards/SPIFFE-ID.md)).
  `+` is a legal tag character but not a legal path character: a tag with `+`
  must be refused or escaped. IDs are case-sensitive, so keys and values
  should be lowercased for identity, unlike for metadata.
- **SPIRE selectors.** 255 characters per selector value on MySQL, enforced
  for OID selectors after #7312 (see 1). For other selectors the limit is the
  datastore's **[unverified for SQLite/Postgres]**.
- **Certificate.** Under (a), one RDN per tag. With tens of tags the
  certificate grows by a few hundred bytes; no limit of the TPM or of the
  attestation stream was found to matter **[unverified]**. A custom OID arc
  needs a registered root (an IANA Private Enterprise Number or the site's
  own arc).

## Proposed direction

1. **Now, on SPIRE 1.14:** (b2). Extend the #89 reconciler into the one
   writer of node aliases: per grant
   `spiffe://<td>/pve/<cluster>/pool/<pool>/tag/<key>/<value>`, plus the bans.
   Free tags stay **out of the certificate** (answer 4 stands for tags),
   because a certificate cannot take a right back before it expires.
2. **In this repository:** log the leaf's SHA-1 (the agent ID) in
   `devid issued`, so the reconciler can tell incarnations apart; nothing
   else changes in the daemon.
3. **Later, gated on a SPIRE release with #7312:** (c). Add the pool and the
   EK hash as subject attributes, so the scope and the incarnation are bound
   in the certificate, and aliases select them instead of `cn:<vmid>`.
4. (b1) stays the fallback if the reconciler's ban window proves too long. It
   is the only option that refuses an old incarnation at attestation.

## Decisions for the maintainer

- **A-1.** Scope: pool, registered tag, or something else? And does a VM
  without a pool get any grant?
- **A-2.** Free tags in the certificate: keep answer 4 (recommended) or
  revisit it?
- **A-3.** Where the reconciler lives (bgl-infra, beside the P1-w hook, or
  this repository) and which credentials it gets on SPIRE and Proxmox.
- **A-4.** SPIFFE ID shape and lowercase/escape rule for tags (section 4, 6).
- **A-5.** OID arc for later subject attributes (PEN or site arc).
- **A-6.** Whether to wait for #7312 in a release or to write (b1).

## Not verified here

- Which SPIRE release will contain #7312.
- That an external node attestor may replace the builtin `tpm_devid` by name.
- Re-attestation picking up changed selectors (read from source, not run).
- Whether `ListAgents` exposes anything that ties an agent to an EK.
- `www-data` reading `/etc/pve/user.cfg`; one pool per VM; a tag length
  limit; `registered-tags` matching exact tags only (D9, not re-read).
- Selector length limits on SQLite and Postgres datastores.
- All Proxmox sources were read at `HEAD`, not at the installed pve-manager
  9.2.3 / pve-guest-common versions on hogan.
