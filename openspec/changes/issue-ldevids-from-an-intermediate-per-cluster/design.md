# Design

## Context

- Host package loads a single DevID CA cert (`devid_ca_cert`) and key
  (`devid_ca_key` or systemd credential `devid_ca.key`). That material is
  today’s **root**.
- Finish returns one PEM in `devid_cert_pem` (leaf only). Guests write it to
  the path SPIRE agent `devid_cert_path` names.
- Lab SPIRE server (`/var/lib/mds-lab/spire/conf/server/server.conf`):

  ```hcl
  NodeAttestor "tpm_devid" {
      plugin_data {
          devid_ca_path = ".../devid-ca.pem"
          endorsement_ca_path = ".../endorsement-ca.pem"
      }
  }
  ```

- Parallel work: #70 CN-only subject, #71 finite lifetime + renew. This design
  assumes those land first for code-go; the record itself is based on
  `main@da0083a`.

## SPIRE 1.14.0 facts (source + lab)

Checked against tag `v1.14.0`
(`pkg/server/plugin/nodeattestor/tpmdevid/{devid.go,selectors.go,devid_test.go}`):

| Side | Behaviour |
|---|---|
| Server `devid_ca_path` | `util.LoadCertPool` → **roots** only (`devIDRoots`) |
| Attestation payload `DevIDCert[0]` | Leaf |
| Attestation payload `DevIDCert[1:]` | **Intermediates** (parsed into `Intermediates` pool) |
| Verify | `x509.Verify` with `Roots=devIDRoots`, `Intermediates=…` |
| Agent `devid_cert_path` | `util.LoadCertificates` → **one or more** PEMs; all go into `DevIDCert` in order |
| Selectors | `subject:cn:<leaf CN>`, `issuer:cn:<leaf issuer CN>`, and for **every** cert in each verified chain **except the leaf**: `ca:fingerprint:<hex>` |
| Fingerprint | `sha1.Sum(cert.Raw)` then hex — SHA-1 over the certificate’s ASN.1 DER (`selectors.go` `Fingerprint`) |
| Root vs intermediate | With an intermediate, tests expect **both** `ca:fingerprint:<intermediate>` and `ca:fingerprint:<root>`. With no intermediate, only the root fingerprint appears |
| Multi-selector entries | `spire-server entry create -selector` “can be used more than once, to specify multiple selectors that **must** be satisfied” (AND). A node alias with `subject:cn` **and** `ca:fingerprint:<cluster intermediate>` matches only when both are present |

Lab (single-tier today): agent
`spiffe://lab.local/spire/agent/tpm_devid/6c525395…` shows
`tpm_devid:subject:cn:100`, `tpm_devid:issuer:cn:… Lab DevID CA`, and
`tpm_devid:ca:fingerprint:ff923f7e3604e117041492c13554936a510d90ac`, which
equals `sha1(DER)` of `/var/lib/mds-lab/spire/conf/server/devid-ca.pem`.
Two-tier lab mint was not run in this pass; intermediate+root dual
`ca:fingerprint` selectors are covered by SPIRE’s own unit tests on
`v1.14.0`.

With a real intermediate, the **agent file must contain leaf +
intermediate**; the **server file must contain the offline root** (not the
intermediate alone — trusting the intermediate as a root is variant A below
and defeats cluster-scoped revocation).

## Open questions for dembaca (with recommendations)

### Q0 — What does the intermediate protect against?

**Ask:** Today a node holds the root and can mint an LDevID for any VM ID.
After cut-over, SPIRE still trusts the offline root in `devid_ca_path`. If
registrations / node aliases select **only** on `tpm_devid:subject:cn:<vmid>`
(as D8 / the lab entry do today), a compromised node that holds its
**cluster intermediate** can still mint an LDevID for any VM ID — including
IDs that belong to another cluster — and SPIRE will accept it. Without a CRL,
revoking a leaked intermediate means rotating the **root** (or every alias
that trusted that path). So relative to today, intermediate-only packaging
buys “root private key off the hypervisors”, and little else for
cross-cluster forgery.

**Within one cluster**, every node that holds that cluster’s intermediate can
forge any VM ID of that cluster. That limit is true for **both** variants
below and is not fixed by this change.

Two variants — pick one; Q2/Q4/Q5 mostly follow:

| | **(A) Key hygiene only** | **(B) Cluster-scoped selectors** |
|---|---|---|
| Trust | Root in `devid_ca_path`; nodes hold intermediate | Same |
| Registration / node alias | Keep `tpm_devid:subject:cn:<vmid>` only | **Also** require `tpm_devid:ca:fingerprint:<sha1(intermediate DER)>` (AND with CN) |
| Cross-cluster forge | Still possible with a stolen intermediate | Blocked: forged leaf carries the wrong intermediate fingerprint |
| Revoke one cluster | Rotate root (or rebuild all trust) | Drop / replace aliases for that fingerprint; root stays |
| Affects | Packaging + PEM chain only | D8 node-alias shape, registration docs, ops playbooks |

**Recommendation: (B)**, because SPIRE 1.14 already emits
`tpm_devid:ca:fingerprint:` for the intermediate (and the root) and entry
selectors AND. Lab proof after code-go: mint two-tier materials, attest,
confirm both fingerprints, create a node alias with CN **and** intermediate
fingerprint, show a leaf minted under another intermediate does not match.

If dembaca picks (A), document explicitly that the threat model is only
“protect the offline root key”, not cross-cluster isolation.

### Q1 — Who mints the intermediate, and where does the root live?

**Ask:** Offline root on an HSM / air-gapped host vs Ansible CA role on a
bastion? What lands in this git repo (scripts, docs) vs site Ansible only?

**Recommendation:** Keep the root **offline** (existing Proxmox/swtpm CA
practice). Add an Ansible role (or extend `proxmox-swtpm`) that:

1. Holds the root only on the CA host (not on hypervisors).
2. Mints one intermediate per cluster (CSR on first node or on the CA host).
3. Distributes `cluster-devid-intermediate.crt` + `.key` to nodes in that
  cluster; distributes `devid-root.crt` (cert only) to SPIRE servers and as
  optional host documentation.

In-repo: a short `docs/` or `debian/README.Debian` procedure + a
`scripts/gen-devid-intermediate.sh` that wraps `certtool`/`openssl` the same
way `gen-lab-pki.sh` does for the lab — **not** the root private key. Lab
`scripts/qemu-lab/gen-lab-pki.sh` gains a two-tier mode for cloud verification.

### Q2 — How is the chain delivered, and what does SPIRE need?

**Ask:** Does `devid_cert_pem` become leaf+intermediate PEM concatenation?
Does SPIRE `devid_ca_path` get only the root, or a bundle?

**Recommendation (matches SPIRE 1.14 source above):**

- **SPIRE server `devid_ca_path`:** root certificate(s) only.
- **Guest `devid.crt.pem` / enroll `devid_cert_pem`:** leaf PEM followed by
  intermediate PEM (no root). Agent `LoadCertificates` already supports this;
  **no new JSON field** (stays inside ASSIGN non-goals).
- **Host `devid_ca_cert`:** the **intermediate** certificate (issuer of
  LDevIDs). Optional new key `mds.devid_root_cert` only if something other than
  SPIRE must validate on the host — not required for enroll itself.
- Lab proof: mint intermediate, point server `devid_ca_path` at root, agent
  cert file at leaf+intermediate, attest once; under Q0(B) also confirm
  `ca:fingerprint` for intermediate+root and a two-selector node alias.
- If Q0 picks (B), registration / D8 node aliases gain the intermediate
  fingerprint selector (AND with CN); that is ops/docs, not a new enroll
  JSON field.

### Q3 — Intermediate lifetime, pathLen, name constraints, key usage

**Ask:** Relative to LDevID default 90d (#71) and cap 8760h?

**Recommendation:**

| Field | Recommendation |
|---|---|
| Intermediate `notAfter` | ≥ 2× LDevID default (e.g. **180d–1y**), shorter than root; rotate with overlap |
| Root lifetime | Multi-year offline (site policy; not this package’s problem) |
| `pathLenConstraint` | **0** on the intermediate (must not mint further CAs) |
| KeyUsage | `keyCertSign` (+ `crlSign` only if you later add CRLs — out of scope) |
| Extended KeyUsage | omit on intermediate (EKU on leaf stays `tcg-cap-verifiedTPMFixed`) |
| Name Constraints | **omit for beta** (SPIRE selects on CN/`tpm_devid` fingerprint; DNS/email constraints do not bind VM IDs). Revisit if a cluster must be cryptographically unable to name another cluster’s VM IDs — that needs a different subject design than CN=`<vmid>` |

LDevID lifetime (#71) stays on the leaf; intermediate expiry is an ops
rotation problem, not a guest renew trigger (guest renew only watches the
leaf).

### Q4 — Migration and `/health` `devid_ca_readable`

**Ask:** How do existing root-issued LDevIDs coexist? Does health still mean
“signing material readable”?

**Recommendation:**

- **Transition:** SPIRE `devid_ca_path` keeps the **root**. Old leaves that
  chain directly to the root continue to verify (empty intermediates). New
  leaves chain through the intermediate. No dual SPIRE plugin needed.
- **Hosts:** replace node credential with the intermediate key; keep serving
  enroll. Guests renew (#71) into intermediate-issued leaves on their normal
  schedule; no mass re-enroll required for trust, only for moving off a
  compromised-root threat model.
- **`devid_ca_readable`:** keep meaning “the **signing** CA cert+key the
  enroller loads are readable” (now the intermediate). Do **not** overload it
  to mean “root reachable”. If operators want a root check, add a separate
  field later — not required for beta.

### Q5 — Effect on #71 renew and serial

**Ask:** Does intermediate change renew or serial rules?

**Recommendation:**

- **Renew (#71):** unchanged decision logic (missing / corrupt / expired /
  last third). After cut-over, a renew naturally gets an intermediate-issued
  leaf; SVID cache clear still required because the leaf DER (fingerprint)
  changes.
- **Serial:** today’s hash includes the **issuing** CA certificate DER
  (`uniqueSerial` over `ca.cert.Raw`). After cut-over that is the
  intermediate, so serials are not comparable to root-era serials — fine.
  Validity remains in the hash (#71 D-g). Same key + same intermediate + same
  validity window still collides (acceptable).

## Non-goals

- Automatic SPIRE `agent evict` / `purge` reconciler (E2.4).
- Changing CN-only subject rules (#70).
- New enroll JSON fields if PEM concatenation suffices.
- Putting the root private key on hypervisors or in this git repo.
- Name Constraints that encode Proxmox cluster identity (unless dembaca
  overrides Q3).

## Risks

- **Variant (A) only (key hygiene):** SPIRE trusts the root; aliases select
  only on `subject:cn`. A stolen cluster intermediate still mints any VM ID
  SPIRE will accept. Gains are limited to keeping the root private key off
  hypervisors. Revoking that intermediate without CRLs still means root
  rotation (or mass re-registration). This is the failure mode the earlier
  note “shipping only the intermediate in `devid_ca_path` …” was about when
  misread as trusting the intermediate as a root — same end-state for
  forgery if selectors ignore `ca:fingerprint`.
- **Variant (B):** ops must keep node aliases (and any registration docs) in
  sync with each cluster’s intermediate fingerprint; rotating an intermediate
  requires updating those selectors before guests renew onto the new chain.
- Forgetting to put the intermediate in the guest PEM breaks attest with
  “unable to verify DevID signature” while root-only leaves still work —
  confusing during migration.
- Lab `gen-lab-pki.sh` today is single-tier; without a two-tier lab mode,
  cloud verification of intermediate `ca:fingerprint` selectors is weak
  (source tests cover it; lab confirm is still owed at code-go).
- **Intra-cluster:** any node with the cluster intermediate can forge any VM
  ID in that cluster (A and B). Not in scope to fix here.
