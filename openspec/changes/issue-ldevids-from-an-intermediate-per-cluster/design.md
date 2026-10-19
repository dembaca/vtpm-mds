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

Checked against tag `v1.14.0`:

| Side | Behaviour |
|---|---|
| Server `devid_ca_path` | `util.LoadCertPool` → **roots** only (`devIDRoots`) |
| Attestation payload `DevIDCert[0]` | Leaf |
| Attestation payload `DevIDCert[1:]` | **Intermediates** (parsed into `Intermediates` pool) |
| Verify | `x509.Verify` with `Roots=devIDRoots`, `Intermediates=…` |
| Agent `devid_cert_path` | `util.LoadCertificates` → **one or more** PEMs; all go into `DevIDCert` in order |

Lab today uses a single-cert `devid-ca.pem` (root = issuing CA) and a
single-cert agent `devid.crt.pem`. That matches “no intermediates” tests in
SPIRE. With a real intermediate, the **agent file must contain leaf +
intermediate**; the **server file must contain the offline root** (not the
intermediate alone, or Verify fails unless the intermediate is also trusted as
a root — which defeats the point).

Selectors include `ca:fingerprint:` of intermediates when present (server
plugin tests). Prefer continuing `tpm_devid:subject:cn:<vmid>` registration.

## Open questions for dembaca (with recommendations)

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
  cert file at leaf+intermediate, attest once.

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

- Shipping only the intermediate in SPIRE `devid_ca_path` makes the
  intermediate a trust anchor — a stolen intermediate still enrolls forever
  from SPIRE’s point of view until the path is rotated; the offline root does
  not help unless SPIRE trusts the root and nodes only hold the intermediate.
- Forgetting to put the intermediate in the guest PEM breaks attest with
  “unable to verify DevID signature” while root-only leaves still work —
  confusing during migration.
- Lab `gen-lab-pki.sh` today is single-tier; without a two-tier lab mode,
  cloud verification of this change is weak.
