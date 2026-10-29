# Design

## Context

The analysis is in `docs/explorations/2026-10-25-attribute-based-vm-identity.md`
(PR #91). It is not repeated here. The facts this design rests on:

- Released SPIRE `tpm_devid` (1.14.x, unchanged in 1.15.x) emits only
  `subject:cn`, `issuer:cn` and `ca:fingerprint:<sha1 of each non-leaf DER>`.
  The agent ID is `spiffe://<td>/spire/agent/tpm_devid/<sha1 of the leaf DER>`.
- A node alias is a registration entry whose parent is
  `spiffe://<td>/spire/server`; its selectors are ANDed; several entries may
  share one SPIFFE ID with different selectors. Workload entries name an alias
  as their parent.
- `tpm_devid` is reattestable, so a node's selectors and the aliases it
  matches are refreshed at each agent SVID rotation. A banned agent ID cannot
  attest again; a new LDevID gives a new agent ID.
- Proxmox tags can be set by anyone with `VM.Config.Options` on the VM, which
  is the VM owner. Pool membership needs `Pool.Allocate` on the pool.

Option (b2) of the exploration is the direction the maintainer chose: nothing
changes in the certificate; a process outside SPIRE writes node aliases whose
selectors are the released, cryptographically bound ones and whose SPIFFE IDs
carry the scope and the tag.

**What is cryptographic and what is not.** That an agent is VM `<vmid>` of the
cluster whose intermediate it chains to is proved by the LDevID and the TPM.
That this VM is in pool P and carries tag T is the reconciler's assertion,
from an authenticated Proxmox API read, written through the SPIRE admin API.
It is not signed and a relying party cannot check it afterwards; it is
trusted as far as anyone holding SPIRE admin rights. Putting it in the
certificate would sign the same Proxmox read at issuance and freeze it until
`notAfter`, which is why A-2 keeps tags out.

## Goals / Non-Goals

**Goals:**

- Node aliases under `spiffe://<td>/pve/<cluster>/` match the live VMs, pools
  and tags of each configured cluster within one reconcile interval.
- A removed tag, a pool change or a destroyed VM takes its aliases away
  without any action in the guest and without reissuing a certificate.
- Agents of destroyed VMs are banned (#89, first half).
- An operator can derive from the man page which IDs a VM receives and why.

**Non-Goals:**

- Telling two incarnations of one VM ID apart (#89, second half): Q-1.
- Packaging and deployment of the binary: Q-2 and the operator.
- Subject attributes in the LDevID (option (c)) and a custom node attestor
  (option (b1)); A-5/A-6 wait for a SPIRE release with #7312.
- Workload entries. The reconciler writes node aliases only; which workloads
  run under an alias stays the operator's.
- LXC guests; deleting EK pins; anything on Proxmox nodes.

## Decisions

- **D-a. One writer per prefix.** The reconciler owns every node alias whose
  SPIFFE ID starts with `spiffe://<td>/pve/<cluster>/` for a configured
  `<cluster>`, and nothing else. It creates and deletes only there. An entry
  someone wrote by hand under that prefix is deleted on the next pass; the man
  page says so.
- **D-b. Three ID shapes (A-4).** For a VM `<vmid>` of cluster `<cluster>`:
  `…/pve/<cluster>/vm/<vmid>` always; `…/pve/<cluster>/pool/<pool>` when the VM
  is in a pool; `…/pve/<cluster>/pool/<pool>/tag/<key>/<value>` for each of its
  instance tags when it is in a pool. Tags are parsed exactly as the
  `vm-inventory` capability parses them for metadata (`vtpm_mds.io.<key>.<value>`,
  prefix case-insensitive), then key and value are lowercased. A pool, key or
  value that is not a valid SPIFFE path segment (`[a-z0-9._-]` after
  lowercasing) yields no alias and one log line per pass naming the VM and the
  tag; `+` is the case Proxmox allows. A VM without a pool gets only its
  `vm/` alias, and its tags grant nothing (A-1).
- **D-c. Selectors.** Every alias for VM `<vmid>` of cluster `<cluster>` has
  exactly `tpm_devid:subject:cn:<vmid>` and
  `tpm_devid:ca:fingerprint:<sha1>`, with `<sha1>` the lowercase hex SHA-1 of
  the DER of one configured intermediate of that cluster. With two
  intermediates configured (rotation), every alias exists once per
  intermediate. The cluster name in the ID comes from the configuration, not
  from Proxmox, so two clusters with the same `cluster_name` stay apart.
- **D-d. Never delete on a doubtful read.** A pass applies a cluster's diff
  only when the read of that cluster succeeded completely. A failed request, a
  TLS or authentication error, or a read with **no qemu guest at all** while
  aliases exist under that cluster's prefix, skips that cluster for the pass
  and logs once. Proxmox filters `/cluster/resources` by the token's rights
  and answers an under-privileged token with a short or empty list instead of
  `403`; at start the reconciler therefore checks the token's
  `/access/permissions` for `VM.Audit` on `/vms` and `Pool.Audit` on `/pool`
  (propagated) and refuses to start without them.
- **D-e. Bans for destroyed VMs.** A `tpm_devid` agent whose selectors include
  `ca:fingerprint` of a configured intermediate of cluster C and whose
  `subject:cn` names no qemu guest of C in **two consecutive** successful
  reads is banned, with one log line naming the agent ID and the VM ID. Two
  reads, so that a VM briefly missing from one listing (an API answering
  during a migration or a restore) is not banned. Banned, not evicted: an
  evicted agent could attest again with the same certificate, for example from
  a backup.
- **D-f. The issuance line carries the agent ID.** `devid issued` gains
  `leaf=<sha1>`, so the agent ID SPIRE will give a certificate is in the
  issuer's log. Nothing consumes it in this change; it costs one field and
  every answer to Q-1 needs it.
- **D-g. Configuration.** One YAML file, decoded strictly like the daemon's:
  the trust domain, the SPIRE admin socket path, the interval (default 60 s),
  and per cluster its name, the API URL, a CA file for TLS, the token file and
  the intermediate certificate files. Secrets are read from files (systemd
  credentials where available), never from the YAML itself.
- **D-h. Documentation (A-1's condition).** `vtpm-mds-reconcile(8)` derives
  the rule: who can set a tag, why a tag alone is worth only as much as the VM
  owner, why the pool is the scope, which IDs a VM receives, and what an owner
  does for a tag to take effect. One worked example per case: a VM with pool
  and tags, a VM without a pool, a tag with `+`, a removed tag, a destroyed
  VM.

## Open questions for the maintainer

- **Q-1. Reused VM IDs.** Between a destroy and a re-creation under the same
  ID inside two reads, or for a VM ID reused after the ban, the reconciler
  cannot tell the old agent from the new one: the attested node record holds
  no EK and no issuance time **[unverified: `ListAgents` output not
  inspected]**. Options: (1) ship the `leaf=` lines of every node to the
  reconciler (journal forwarding, the operator's); (2) compare against the
  guest config's `meta: ctime=`; (3) an issuance record per VM that the
  reconciler can read. Recommendation: (1), as a change of its own once this
  one runs.
- **Q-2. Packaging.** The binary runs beside the SPIRE server, not on a
  Proxmox node, so neither existing package fits. Recommendation: a third
  `.deb` and a static binary in a follow-up change; until then the operator
  builds it.

## Risks / Trade-offs

- **The attribute part is an assertion, not a proof** (Context). A compromised
  reconciler or SPIRE admin grants anything; that is no worse than a SPIRE
  admin today.
- **Removal latency** is one interval plus the agent's entry sync, and a
  workload holding an SVID keeps it until it expires. Granting is as fast.
- **Hand-written entries under `/pve/<cluster>/`** are deleted (D-a).
- **A restored VM after a ban** cannot attest with its old certificate; the
  guest must enroll again. The man page says so.
- **Every node holds the cluster intermediate**, so a compromised node can
  issue for any VM ID of its cluster and so reach every alias of that cluster.
  Unchanged by this design; the selector on the intermediate limits it to one
  cluster.
- **Proxmox API facts read from source, not run**: that `/cluster/resources`
  returns `pool` and `tags` per guest and that `PVEAuditor` suffices. Task 2.1
  checks both against a real cluster before any code.
