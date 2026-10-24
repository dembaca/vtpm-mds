# Exploration: operating the DevID CA

Status: draft for the maintainer, 2026-10-19. Not a change record. Nothing here
is decided; the last section proposes a decision to accept or change.

Sources read: `internal/devid/{ca,auth,client,http}.go`, `cmd/devid-enroll/main.go`,
`openspec/specs/devid-enrollment/spec.md`, `debian/vtpm-mds.service`,
`debian/README.Debian`, `README.md` (SPIRE sections), `AGENTS.md`,
`scratchpad/d9.md`; SPIRE `main` and tags `v1.14.0`/`v1.15.4` on GitHub;
`qemu-server` from git.proxmox.com; the Proxmox `datacenter.cfg(5)` page.
Items marked **[unverified]** were not checked against a primary source or a
running system.

## Where things stand

| Fact | Where |
|---|---|
| Every LDevID has `notAfter` 9999-12-31T23:59:59Z, labelled "IEEE 802.1AR / TCG far-future NotAfter" | `internal/devid/ca.go:21-22`, `:163` |
| The spec makes that a requirement and forbids any revocation mechanism ("SHALL NOT publish a revocation list") | `openspec/specs/devid-enrollment/spec.md:583-612` |
| CN is the authenticated VM ID; other subject RDNs come from the guest's request unchanged | `ca.go:145-147`, spec `:589-595` |
| The serial is SHA-1(CA cert, subject, public key): same key and subject give the same serial | `ca.go:114-131` |
| The SAN carries the SHA-256 of the EK public key (PermanentIdentifier) | `ca.go:178-190`, spec `:599-605` |
| Each `devid-enroll` run makes a **new** DevID key and writes a new cert to `-out` | `internal/devid/client.go:23-35`, `cmd/devid-enroll/main.go:19` |
| The VM ID comes from the inventory entry the caller's MAC resolves to; an optional `ek_sha256` pin exists | `internal/devid/auth.go:71`, `:97`, `:108` |
| The CA key file is `root:root 0600`, read by systemd and handed over as credential `devid_ca.key` | `debian/vtpm-mds.service:31`, `debian/README.Debian:45-54` |
| Every node holds the CA key; a compromised node can issue for any VM ID | `AGENTS.md:273-274` |
| D8: CN stays the VM ID; a readable SPIFFE ID comes from a SPIRE node alias on `tpm_devid:subject:cn:<vmid>` | `AGENTS.md:337-339` |
| `vtpm-mds` does not read `smbios1` or `vmgenid` from the guest config | `grep` over `internal/proxmox`, `internal/inventory`: no hits |

How SPIRE `tpm_devid` uses the certificate (source on `main`, behaviour the same
in `v1.14.0` where noted):

- **Agent ID** is `spiffe://<td>/spire/agent/tpm_devid/<SHA-1 of the leaf DER>`
  ([server doc](https://github.com/spiffe/spire/blob/main/doc/plugin_server_nodeattestor_tpm_devid.md)).
  A new certificate is a new agent ID, even for the same key.
- **Chain**: the agent sends every PEM block in `devid_cert_path`
  (`pkg/agent/plugin/nodeattestor/tpmdevid/devid.go:277-285`); the server uses
  blocks 2..n as intermediates and `devid_ca_path` as roots
  (`pkg/server/plugin/nodeattestor/tpmdevid/devid.go:143-153`). Intermediates
  therefore work without touching the server bundle.
- **Validity**: verification is plain Go `x509.Certificate.Verify` with roots,
  intermediates and `ExtKeyUsageAny`, no `CurrentTime` override, **no CRL or
  OCSP** (`server devid.go:261-271`). An expired leaf, intermediate or root fails.
- **Re-attestation**: the plugin returns `CanReattest: true` (`server devid.go:225`,
  also `v1.14.0:218`). The agent rotator, when its SVID is due for rotation,
  runs the **full attestation again** instead of a plain renewal
  (`pkg/agent/svid/rotator.go:211-219`). The DevID is re-checked at every agent
  SVID rotation (`agent_ttl`, default `default_x509_svid_ttl` = 1h,
  [server doc](https://github.com/spiffe/spire/blob/main/doc/spire_server.md)).
- **No renewal hook.** The agent reads the DevID files only in `Configure`
  (`agent devid.go:246`). `SIGUSR1` reconfigures only plugins whose
  `plugin_data_file` changed ([agent doc, "Reconfiguring plugins"](https://github.com/spiffe/spire/blob/main/doc/spire_agent.md)),
  so a cert rewritten in place is picked up by a restart.
- **Selectors** in released SPIRE: `subject:cn`, `issuer:cn`, and
  `fingerprint:<sha1>` for each **non-leaf** cert of the verified chain.
  Subject-attribute selectors `tpm_devid:subject:oid:<OID>:<value>` were merged
  on `main` on 2026-10-08 (spiffe/spire#7312) and are **not** in `v1.15.4`
  (2026-10-15).
- **Ban/evict**: `spire-server agent ban -spiffeID` stops an agent from
  re-attesting; `agent evict` deletes the record, and a reattestable agent may
  simply attest again ([server doc](https://github.com/spiffe/spire/blob/main/doc/spire_server.md)).
  A ban targets one certificate's agent ID, not a CN.

## 1. LDevID lifetime and renewal

**Facts.**

- 802.1AR: an IEEE 802.1 contribution summarising 802.1AR-2018 for IEC/IEEE 60802
  gives the IDevID validity as "Should be eternal (notAfter=99991231235959Z)"
  and the LDevID validity as "Arbitrary value tuple with notBefore < notAfter"
  ([Pfaff et al., 2021, IDevID and LDevID field tables](https://grouper.ieee.org/groups/802/1/files/public/docs2021/60802-Pfaff-et-al-Background-for-802-1AR-Adoption-1121-v01.pdf)).
  So 9999 is the IDevID convention. 802.1AR allows it for an LDevID but does
  not call for it. **[unverified against the standard's own text, which is paywalled]**
- Because SPIRE re-attests at every SVID rotation, the DevID's `notAfter` works
  as a hard deadline. At most one `agent_ttl` after expiry the agent can no
  longer rotate and loses its SVID (`rotator.go:102-105`). **[unverified in a lab]**
- With today's 9999 the only ways to withdraw a node are `agent ban` or
  removing the CA from `devid_ca_path`, and removing the CA withdraws every node.
- The CA certificate's own `notAfter` caps every LDevID, because Go checks the
  whole chain. Its value on hogan was not read. **[verify on hogan]**
- Renewal through `devid-enroll` means a re-run, a new TPM key, a new cert and
  a **new agent ID**, then a `spire-agent` restart. Anything registered against
  the old agent ID (parent ID of workload entries) stops matching; aliases
  keyed on selectors keep matching.
- Renewing **the same key** would need a new client mode. It would also give
  the same serial for a different certificate (`ca.go:114-131`), against the
  RFC 5280 rule that serials are unique per issuer, unless `notBefore` is added
  to the hash.

**Options.**

| | Option | For | Against |
|---|---|---|---|
| L1 | Keep 9999 | No guest tooling; matches today's spec | Withdrawal only by ban, per cert; a lost vTPM backup is valid forever |
| L2 | Finite lifetime, configurable (`devid.cert_ttl`), renewal by a guest timer that runs `devid-enroll` into a new directory, swaps atomically and restarts `spire-agent` | Expiry bounds every other failure (ID reuse, stolen state, node compromise); no SPIRE change | Guest timer plus agent restart; agent ID changes per renewal; the guest has to be bound and the node up at renewal time |
| L3 | Short lifetime (hours or days) and renewal before each restart | Close to revocation | Tight coupling with IMDS availability; an outage of a node's IMDS stops SPIRE on its guests |

**Trade-off.** A finite lifetime is the only revocation SPIRE `tpm_devid`
honours on its own, since it reads no CRL. Its cost is guest-side renewal.
802.1AR does not stand in the way.

**Recommendation.** L2. Make the lifetime a config key, ship `devid-enroll`
renewal (an `-out` swap plus a systemd timer in the guest package) and require
aliases on selectors, never on agent IDs. Choose the default lifetime when the
CA design below is settled; the order of magnitude to start from is 90 days,
renewed at two thirds of it **[a judgement, not measured]**.

**To verify.** Lab: agent behaviour across expiry (expect failure within one
`agent_ttl`), renewal by swap plus restart, alias continuity across a new agent
ID, and old agent records pruned by `prune_attested_nodes_expired_for`.
hogan: the CA cert's `notAfter`, and VM 399 renewing against the installed
package without losing workload SVIDs.

## 2. VM ID reuse

**Facts.**

- Proxmox hands out VM IDs from `/cluster/nextid`, within the datacenter option
  `next-id: lower=<int>,upper=<int>` (defaults 100 and 1000000, "free next-id
  API range", [datacenter.cfg(5)](https://pve.proxmox.com/pve-docs/datacenter.cfg.5.html)).
  The docs do not say whether freed IDs come back. That `nextid` returns the
  lowest free ID, so a deleted VM's ID is reused, is the commonly observed
  behaviour. **[unverified: check on hogan by reading `pvesh get /cluster/nextid`
  around a delete, or in `PVE/API2/Cluster.pm`]** A lower bound only moves the
  range; nothing prevents reuse.
- Hookscripts run only `pre-start`, `post-start`, `pre-stop` and `post-stop`
  (`PVE/QemuServer.pm` in qemu-server, the four `exec_hookscript` calls). There
  is **no destroy hook**. `qm destroy --purge` cleans backup, replication and HA
  configs and calls nothing external ([qm(1)](https://pve.proxmox.com/pve-docs/qm.1.html)).
- A deleted VM's certificate is only usable while its TPM state exists. Deleting
  the VM deletes `tpmstate0`, but copies survive in vzdump backups, in volumes
  detached as `unusedN` (this is what `hogan-lab qm fresh-tpm` does on purpose),
  and in clones.
- Clone: `qemu-server` writes a **new** `smbios1` UUID and, when one is set, a
  new `vmgenid` (`PVE/API2/Qemu.pm`, clone path, "auto generate a new uuid").
  The clone gets a copy of the tpmstate, so the same EK and the same DevID key.
- Restore: `vmgenid` is always regenerated; `smbios1` UUID only with `--unique`
  (`QemuServer.pm`, `restore_update_config_line`).
- With an alias on `subject:cn:<vmid>`, any agent whose cert carries that CN
  receives the alias. The new VM with a reused ID gets it as soon as it enrolls.
  The old cert (CN the same, 9999) gets it too, if its TPM state comes back
  (restoring a backup of the deleted VM under another ID is enough).
- **One CA, several clusters:** VM IDs are only unique per cluster. If the same
  CA key is installed on nodes of two clusters, `CN=100` from both match the
  same alias. Today nothing in the cert says which cluster issued it.
  **[open: is the CA shared across clusters? The CA's name is site-wide]**
- The guest controls every subject RDN except CN (spec `:592-595`). Once SPIRE
  releases subject-OID selectors, those RDNs become selectors a guest can set
  itself; SPIRE's doc says the issuer "must validate subject attribute claims
  before signing". That applies whichever option is chosen.

**Options.**

| | Option | Reopens D8? | Notes |
|---|---|---|---|
| R1 | Add an incarnation value to the subject, e.g. `serialNumber` (2.5.4.5) = EK SHA-256 or `smbios1` UUID, overriding any guest value; alias on CN + that OID | No for the CN, **yes** for "alias on `subject:cn`" | Needs a SPIRE release with #7312. EK hash is TPM-bound but equal for clones; smbios UUID differs for clones but survives a plain restore |
| R2 | Alias on CN **and** `issuer:cn` of a per-cluster intermediate (see 3) | Extends the alias, CN unchanged | Fixes cross-cluster collision, not reuse within a cluster |
| R3 | Reconciler: a periodic job that lists `tpm_devid` agents and bans those whose CN has no VM, or a VM whose EK hash differs from the cert's SAN | No | No delete hook exists, so it must poll (Proxmox API or `/etc/pve`) and run with SPIRE server admin rights |
| R4 | Finite lifetime (L2) | No | Bounds the window; does not close it |
| R5 | `ek_sha256` pin per VM in the inventory (exists for YAML; on the readiness list for Proxmox) | No | Stops a reused ID from enrolling with a foreign TPM; a fresh VM still needs a pin written by someone |
| R6 | CRL from the CA | No | Useless for SPIRE: `tpm_devid` never reads one |

**Trade-off.** R1 is the right model (an identity per VM incarnation), but it
waits for SPIRE and changes the alias key D8 chose. R3 plus R4 work with
released SPIRE and leave D8 alone. R2 is needed anyway if more than one cluster
shares a CA.

**Recommendation.** Now: R4 (L2) and R3, with the reconciler matching CN
**and** the SAN's EK hash against the live VM, so a reused ID with a new vTPM is
told apart without changing the cert. Settle the multi-cluster question and use
R2 if the answer is yes. Later, once a SPIRE release ships #7312: R1 with the EK
hash in `serialNumber`, and strip guest-supplied `serialNumber` (better, every
guest RDN that SPIRE would turn into a selector). That part reopens D8's alias
half and needs the maintainer's word.

**To verify.** hogan: `nextid` after a delete; that `qm destroy` removes
`tpmstate0`; that a restored backup attests as the old CN (scratch VM, not 399).
Lab: `agent ban` survives re-attestation with the same cert; R1 selectors on a
SPIRE build from `main`.

## 3. Key custody

**Facts.** The CA key sits on every node (`AGENTS.md:273`), loaded through
`LoadCredential` (`debian/vtpm-mds.service:31`). The daemon runs as `vtpm-mds`
with the key only in its credential directory. A node compromised as root, or
through the daemon, can sign any CN, and so takes every alias. SPIRE accepts
intermediates from the agent's PEM file (above), and treats every cert in
`devid_ca_path` as a trust anchor, self-signed or not (Go `x509` semantics).

**Options.**

| | Option | Compromised node can impersonate | Withdrawal | Cost |
|---|---|---|---|---|
| K0 | Today: one CA key on all nodes | Any VM of any cluster sharing the CA | Replace the CA everywhere | none |
| K1 | Offline root; **one intermediate per node**; guest file holds leaf + intermediate; SPIRE trusts the root | Any VM ID, unless aliases also match `issuer:cn` or the intermediate's `fingerprint` | Remove trust only by rotating the root, or by listing the intermediates as anchors instead of the root | Per-node issuance ceremony; an alias tied to the issuing node breaks when a VM moves and re-enrolls on another node |
| K1b | As K1, but `devid_ca_path` lists the **node intermediates** as anchors | As K1 | Delete one intermediate from the bundle, restart or reload the server | Bundle edit for every new node |
| K2 | Offline root; **one intermediate per cluster**, the same key on each node of that cluster; alias on CN + `issuer:cn` | Any VM of **its own** cluster | Drop that cluster's intermediate | One ceremony per cluster; key still on every node of it. Matches VM ID scope exactly |
| K3 | Central issuer (e.g. a small signer, Vault PKI or step-ca) that nodes call with the authenticated VM ID and EK; no key on nodes | Whatever the issuer believes the node: any VM ID, unless the issuer checks the VM ID against the cluster config and the node it runs on | Central, and the issuer can log and limit | New service, network path and availability dependency; enrollment fails when it is down |
| K4 | K2 with the key in a node TPM or HSM (PKCS#11), not a file | As K2, while the node is compromised; the key does not leave | As K2 | Signing code path through PKCS#11; hardware per node **[not explored]** |

**Trade-off.** Under every option a compromised node can still issue for its
own guests, because it is the party that authenticates them. The options differ
in how far beyond that it reaches and how cheaply it is cut off. K2 reduces the
reach to the unit in which VM IDs are meaningful, and also fixes the
cross-cluster CN collision. K1b reduces it to one node, but VM IDs are
cluster-wide, so a node intermediate limits nothing unless the alias also pins
the issuing node, and that does not survive migration plus re-enrollment. K3 is
the only option that keeps keys off the nodes, and it is a new service with its
own availability problem.

**Recommendation.** K2 with an offline root. Ship the guest PEM as leaf +
cluster intermediate (`devid-enroll` writes both; the service returns the
chain). Put the **root** in SPIRE's `devid_ca_path` and make the alias match
`subject:cn:<vmid>` **and** `issuer:cn:<cluster intermediate CN>`. Withdrawing a
cluster then means rotating the root or switching that server to K1b-style
anchors, so document both. Pair with L2 so a stolen intermediate also expires
(intermediate lifetime of a few years, longer than every leaf). Revisit K3 only
if the node count or audit requirements grow.

**To verify.** Lab: SPIRE `v1.14.0` and current attest with leaf + intermediate
in `devid_cert_path` and only the root in `devid_ca_path`; `issuer:cn` and
`fingerprint:` selectors come out as documented; an alias on two selectors
matches. hogan: the service loads a chain PEM through the existing credential;
the `0600` and credential path is unchanged.

## Proposed decision

For the maintainer to accept or change:

1. **CA hierarchy:** offline root, one intermediate per Proxmox cluster (K2).
   Nodes hold only their cluster's intermediate key, as today through
   `LoadCredential`. The guest receives leaf + intermediate.
2. **Alias key:** a SPIRE node alias matches `subject:cn:<vmid>` **and**
   `issuer:cn:<cluster>`. D8 stays (CN is the VM ID); the alias gains the issuer.
3. **Lifetime:** LDevIDs get a finite, configurable lifetime (start from 90
   days), renewed by a guest timer through `devid-enroll`. The "non-expiring"
   requirement is replaced. The intermediate gets a lifetime longer than every leaf.
4. **ID reuse:** a reconciler bans agents whose CN or EK hash no longer matches
   a live VM, until SPIRE releases subject-OID selectors. Then decide
   separately whether to add the EK hash as `serialNumber` and alias on it (this
   reopens D8's alias half).
5. **Guest RDNs:** stop carrying guest-supplied subject attributes into the
   cert, or limit them to a list, before any SPIRE release with subject-OID
   selectors is deployed.

## Follow-up changes this would produce

- `issue-ldevids-under-a-cluster-intermediate`: chain in the response and the
  PEM, root/intermediate tooling and runbook, `README.md` SPIRE section
  (`devid_ca_path` = root, alias on two selectors).
- `give-ldevids-a-finite-lifetime`: config key, spec change to the
  "Non-Expiring" requirement, serial uniqueness across renewals (include
  `notBefore` or use random serials).
- `renew-the-ldevid-from-the-guest`: `devid-enroll` atomic swap, systemd timer
  and `spire-agent` restart in the guest package.
- `stop-carrying-guest-subject-attributes`: only CN (and later
  service-set attributes) in the subject.
- `reconcile-spire-agents-against-vms` (may live outside this repository):
  periodic ban of agents with no live VM or a changed EK.
- Later, gated on a SPIRE release with #7312: `add-an-incarnation-attribute`
  (reopens D8's alias half).

## Not verified here

- The text of IEEE 802.1AR-2018 itself (cited through an 802.1 contribution).
- That Proxmox `nextid` returns the lowest free ID, so deleted IDs come back.
- Whether one DevID CA is shared by more than one cluster today.
- The DevID CA certificate's `notAfter` on hogan.
- Every SPIRE behaviour above was read from source and docs, not run: expiry
  handling, agent ID change on renewal, chain acceptance, two-selector aliases.
- K4 (TPM/HSM-held CA key) was not explored.
