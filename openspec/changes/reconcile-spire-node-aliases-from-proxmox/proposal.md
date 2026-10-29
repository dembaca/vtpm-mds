# Proposal

## Why

SPIRE registrations for guests today select `tpm_devid:subject:cn:<vmid>`
AND `tpm_devid:ca:fingerprint:<cluster intermediate>`. Two gaps follow:

- **Policies can name only a VM ID.** The maintainer wants rights that follow
  attributes supplied from the hypervisor side, so that roles and groups of a
  VM come from its pool and its tags rather than from a list of VM IDs, and so
  that a reused VM ID does not inherit the old VM's rights (exploration
  `docs/explorations/2026-10-25-attribute-based-vm-identity.md`, PR #91).
- **Agents of destroyed VMs stay attested** (Issue #89). Their agent keeps its
  SVID, and once a new VM with the same ID enrolls, both share every entry
  under `cn:<vmid>`. Today that is cleaned up by hand with
  `spire-server agent evict`.

The maintainer decided on 2026-10-29 (bus asks `ko3taid5…`, `yrukihan…`,
`s3mznmom…`, `sxxft5bm…`, `pkscik3y…`):

- **A-1.** The Proxmox pool is the admin scope. A tag grants only together
  with the VM's pool; a VM without a pool gets nothing from its tags. The
  documentation SHALL derive this rule so that an operator can follow it.
- **A-2.** Free tags stay out of the LDevID certificate (answer 4 of
  2026-10-18 stands). Rights from tags reach SPIRE as node aliases.
- **A-3.** The reconciler is a binary of this repository. It runs as one
  instance beside the SPIRE server, not on Proxmox nodes. Its deployment and
  credentials belong to the operator.
- **A-4.** SPIFFE IDs `spiffe://<td>/pve/<cluster>/vm/<vmid>`,
  `…/pve/<cluster>/pool/<pool>` and `…/pve/<cluster>/pool/<pool>/tag/<key>/<value>`;
  keys and values lowercased; a tag with a character a SPIFFE path segment
  cannot hold grants nothing and is logged.
- **A-5/A-6** (OID arc, waiting for SPIRE #7312 or writing a node attestor)
  are deferred until a SPIRE release contains #7312.

## What Changes

- A new binary, `vtpm-mds-reconcile`, reads VMs, their pools and their
  `vtpm_mds.io.<key>.<value>` tags from the Proxmox API of each configured
  cluster and makes the SPIRE server's node aliases under
  `spiffe://<td>/pve/<cluster>/` match them. Each alias selects the released
  `tpm_devid` selectors, `subject:cn:<vmid>` AND `ca:fingerprint:<sha1>` of a
  configured intermediate of that cluster. It creates missing aliases and
  deletes aliases under that prefix that no VM justifies.
- It bans `tpm_devid` agents of a configured cluster whose VM ID no longer
  exists there (the destroyed-VM half of #89).
- `devid issued` gains `leaf=<sha1>`, the SHA-1 of the issued certificate's
  DER, which is the suffix of the agent ID SPIRE gives that certificate.
  Whatever later tells incarnations of one VM ID apart (design Q-1) needs it.
- A man page `vtpm-mds-reconcile(8)` explains, with a worked example per case,
  why a tag needs a pool, which ID a VM receives and what an owner must do for
  a tag to take effect (A-1's condition).

## Impact

- New capability `spire-reconciler`. Modified: `devid-enrollment` (the
  issuance line).
- Code: `cmd/vtpm-mds-reconcile/`, `internal/reconcile/`, one line in
  `internal/devid/http.go` (`logIssuance`) and its test. New dependency on the
  SPIRE server API client (`github.com/spiffe/spire-api-sdk`).
- Docs: `vtpm-mds-reconcile(8)`; `README.md` (status table, one paragraph);
  `ARCHITECTURE.md` (the integration row for SPIRE).
- **Not** in this change: packaging of the new binary (design Q-2), its
  deployment and credentials (the operator's infrastructure repository), the
  reused-VM-ID half of #89 (design Q-1), subject attributes in the certificate
  (option (c), A-5/A-6), a custom node attestor (option (b1)), deleting EK
  pins.
- SPIRE: works with released `tpm_devid` (1.14.x); no server or agent plugin
  changes.
