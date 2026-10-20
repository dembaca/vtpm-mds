# Proposal

## Why

Today every hypervisor node that enrolls guests holds the **DevID root** private
key (`LoadCredential=devid_ca.key` → site path such as
`/etc/ssl/private/proxmox_devid_ca.key`; see archived
`2026-10-18-run-the-daemon-as-its-own-user`). A compromised node can therefore
mint an LDevID for any VM ID in any cluster that trusts that root. Maintainer
beta scope (dembaca, 2026-10-19): move to an **intermediate CA per cluster** so
nodes hold only their cluster intermediate; the root stays offline.

**Decided (dembaca 2026-10-20):** Q0 **(B)** — SPIRE aliases also select
`tpm_devid:ca:fingerprint:<cluster intermediate>` (AND with CN); Q1 **option
A** — offline root on a CA host, Ansible distributes intermediates; Q3 —
intermediate **1y** with overlap from ~9 months, `pathLen=0`,
`keyCertSign` only, no EKU / Name Constraints. Q2/Q4/Q5 confirmed as the
prior recommendations (leaf+intermediate PEM, root in `devid_ca_path`,
`devid_ca_readable` = signing CA). See `design.md`.

Code-Go applied (base #71). Product code, lab two-tier mode, and packaging
comments are in this PR; Fable should-fix addressed before ready.

## What Changes

- **Threat model (Q0 B):** D8 / registrations use CN **and** cluster
  intermediate `ca:fingerprint`.
- **Issuing CA:** `IssueDevID` signs with a cluster intermediate, not the root.
- **Trust chain to SPIRE:** guests present leaf+intermediate; server
  `devid_ca_path` holds the offline **root**.
- **Config / packaging / minting (Q1):** Ansible + in-repo
  `gen-devid-intermediate.sh` / two-tier lab; credential layout for
  intermediate; `/health` `devid_ca_readable` = signing material.
- **Intermediate profile (Q3):** 1y lifetime, pathLen 0, keyCertSign only.
- **Migration:** coexistence with root-issued LDevIDs; #71 renew / serial.

Credential name stays `devid_ca.key` (content = intermediate key); settled at
code-go.

## Apply order

Branch from `main@da0083a`. Prefer **code-go only after #70 and #71 are
archived** so the living issuance requirement already has CN-only subject and
finite lifetime; this change then only swaps the signing CA and the PEM shape.
If code-go must start earlier, rebase onto the post-#71 `main` before
implementation and retarget the REMOVED/ADDED titles to whatever issuance
requirement then lives in `openspec/specs/devid-enrollment/`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: issuing CA is a cluster intermediate; returned PEM is a
  chain SPIRE can consume; config/credential layout for intermediate vs root.
- `instance-metadata` (health): only if `devid_ca_readable` semantics change
  (open question Q4).
- `debian-packaging`: credential/config comments for intermediate + root
  bundle paths (host package).

## Impact

- **Code (after code-go):** `internal/devid/ca.go` (issuer cert+key), enroll
  finish PEM encoding, `internal/config` / `loadDevIDEnroller`, packaged
  `debian/config.yaml` + unit `LoadCredential=`, guest materials path unchanged
  in shape if the chain is written into the existing `devid.crt.pem`.
- **SPIRE 1.14.0 (lab + source):** server `devid_ca_path` = root pool; agent
  `devid_cert_path` may be a multi-PEM file (leaf then intermediates). Verified
  against `github.com/spiffe/spire` tag `v1.14.0`
  (`pkg/server/plugin/nodeattestor/tpmdevid/devid.go`,
  `pkg/agent/plugin/nodeattestor/tpmdevid/devid.go`).
- **Ops / Ansible:** root generation stays out of the guest path; intermediate
  minting and distribution need a playbook or offline procedure (Q1).
- **Not in scope:** E2.4 reconciler; #70 subject rules; enroll JSON field
  renames unless dembaca rejects PEM concatenation; Teleport/SPIRE selector
  docs beyond a short migration note.
