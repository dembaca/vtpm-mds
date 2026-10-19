# Proposal

## Why

Today every hypervisor node that enrolls guests holds the **DevID root** private
key (`LoadCredential=devid_ca.key` → site path such as
`/etc/ssl/private/proxmox_devid_ca.key`; see archived
`2026-10-18-run-the-daemon-as-its-own-user`). A compromised node can therefore
mint an LDevID for any VM ID in any cluster that trusts that root. Maintainer
beta scope (dembaca, 2026-10-19): move to an **intermediate CA per cluster** so
nodes hold only their cluster intermediate; the root stays offline.

This change is a **record only** until dembaca answers the open questions
below. No code until coordinator code-go.

## What Changes

- **Issuing CA:** `IssueDevID` signs with a cluster intermediate, not the root.
- **Trust chain to SPIRE:** guests present a chain SPIRE 1.14 `tpm_devid` can
  verify; the server plugin's `devid_ca_path` holds the **root** (trust
  anchors), while intermediates travel with the leaf (see design D-spire).
- **Config / packaging:** how the intermediate cert+key and the root bundle
  are named, delivered (systemd credential vs files), and reflected in
  `/health` `devid_ca_readable`.
- **Migration:** coexistence of root-issued and intermediate-issued LDevIDs;
  interaction with finite lifetime (#71) and CN-only subject (#70).

Concrete wire/config names and lifetimes are **recommendations pending
dembaca**, not settled defaults.

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
