# Design

## Context

- Host package loads a single DevID CA cert (`devid_ca_cert`) and key
  (`devid_ca_key` or systemd credential `devid_ca.key`). That material is
  today’s **root**.
- Finish returns leaf then intermediate PEM in `devid_cert_pem` (after
  cut-over; pre-cut-over the second block is still the signing root). Guests
  write it to the path SPIRE agent `devid_cert_path` names.
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

Lab (historical single-tier): agent
`spiffe://lab.local/spire/agent/tpm_devid/6c525395…` showed
`tpm_devid:subject:cn:100`, `tpm_devid:issuer:cn:… Lab DevID CA`, and
`tpm_devid:ca:fingerprint:ff923f7e…` = sha1(DER) of the then single-tier CA.

Lab (two-tier, Code-Go): `MDS_LAB_DEVID_TIER=two`; agent
`…/tpm_devid/b90bb8f2…` selectors include `subject:cn:100`,
`ca:fingerprint:<intermediate>`, and `ca:fingerprint:<root>`; node alias
CN+intermediate matches, CN+second-intermediate does not.

With a real intermediate, the **agent file must contain leaf +
intermediate**; the **server file must contain the offline root** (not the
intermediate alone — trusting the intermediate as a root defeats
cluster-scoped revocation under Q0(B)).

## Decisions (dembaca 2026-10-20)

Recorded by the interim coordinator from dembaca’s answers (Q0 19:08, Q1 👍
19:08, Q3 👍 19:09). Q2/Q4/Q5 were not asked separately; they follow and are
**confirmed as the prior recommendation** unless noted.

### Q0 — What does the intermediate protect against? → **(B)**

**Decision:** variant **B** — cluster-scoped selectors. SPIRE node aliases /
registrations SHALL select both:

- `tpm_devid:subject:cn:<vmid>` **and**
- `tpm_devid:ca:fingerprint:<sha1(cluster intermediate DER)>`

(AND; SPIRE 1.14 `entry create -selector` semantics). A compromised node that
holds only its cluster intermediate cannot mint an accepted LDevID for another
cluster’s VM IDs. Revoking one cluster means updating/removing aliases for that
fingerprint; the offline root stays.

**D8 node-alias shape:** today’s lab/D8 alias that is CN-only must gain the
cluster intermediate fingerprint selector (and any registration docs /
playbooks that create those aliases). That is ops/docs work at code-go, not a
new enroll JSON field.

**Still true:** within one cluster, every node with that intermediate can forge
any VM ID of that cluster. Not fixed by this change.

### Q1 — Who mints the intermediate, and where does the root live? → **option A**

**Decision:** root stays **offline** on a CA host outside the hypervisors (same
pattern as the swtpm site CA). An **Ansible role** mints one intermediate per
cluster and distributes cert+key to that cluster’s nodes, and the root
**certificate** (not key) to SPIRE servers. No HSM for the beta.

**In this git repo:** `scripts/gen-devid-intermediate.sh`, short procedure docs,
and a two-tier mode for `scripts/qemu-lab/gen-lab-pki.sh`. No private keys in
the repo.

**Hogan deployment fact (Fable/hogan, read-only, 2026-10-20):** today’s
`/etc/ssl/certs/proxmox_devid_ca.crt` is **already** a `pathLen=0` intermediate
of the site root (`CA:TRUE`, keyUsage includes certSign). It passes the new
`IsCA` load check and can sign LDevID leaves, but **cannot** mint a further
per-cluster intermediate — those must be issued by the site root. So “the node
holds the root today” is not literally true on hogan; the cut-over is from
this shared pathLen-0 site intermediate to a **per-cluster** intermediate
still chained to that offline root.

### Q2 — Chain delivery / SPIRE needs → **confirmed (recommendation)**

Not asked separately; follows Q0(B) + SPIRE 1.14 facts:

- SPIRE server `devid_ca_path`: **root** certificate(s) only.
- Guest `devid.crt.pem` / enroll `devid_cert_pem`: leaf then intermediate PEM
  (no root); **no new JSON field**.
- Host `devid_ca_cert` / credential: the **intermediate** (signing CA).
- Lab at code-go: two-tier mint + attest + two-selector node alias proof.

**Settled at code-go:** systemd credential name stays `devid_ca.key`; only the
file content is the per-cluster intermediate key (coordinator condition).

### Q3 — Intermediate lifetime / constraints → **decided**

| Field | Decision |
|---|---|
| Intermediate `notAfter` | **1 year**; renew with overlap from ~**9 months** |
| Root lifetime | Multi-year offline (site policy) |
| `pathLenConstraint` | **0** |
| KeyUsage | **`keyCertSign` only** (no `crlSign` until CRLs exist) |
| Extended KeyUsage | omit on intermediate |
| Name Constraints | **omit for beta** |

LDevID lifetime (#71) stays on the leaf; intermediate rotation is ops, not a
guest renew trigger.

### Q4 — Migration and `/health` `devid_ca_readable` → **confirmed (recommendation)**

Not asked separately; follows Q0(B)/Q2:

- SPIRE keeps the offline **root**; leaves issued by today’s shared site
  intermediate and by a new per-cluster intermediate coexist during transition
  (both chain to that root).
- Hosts switch the signing credential to the **per-cluster** intermediate;
  guests renew into the new chain on the #71 schedule. On hogan the current
  material is already a pathLen-0 intermediate (see Q1), so the switch is
  intermediate→intermediate, not root→intermediate.
- `devid_ca_readable` remains “signing CA cert+key readable” (whatever the
  node signs with). No root-readable bit for beta.
- Finish always appends the **signing** CA PEM after the leaf. The ADDED
  requirement’s “SHALL NOT include the root” means the offline root PEM is
  never added; the second block is always the configured signing CA.

### Q5 — Effect on #71 renew and serial → **confirmed (recommendation)**

Not asked separately:

- Renew decision logic unchanged; post-cut-over renews are intermediate-issued;
  SVID cache clear still required on PEM change.
- Serial hash uses the issuing CA DER (now the intermediate) plus validity;
  root-era serials are not comparable — fine.

## Non-goals

- Automatic SPIRE `agent evict` / `purge` reconciler (E2.4).
- Changing CN-only subject rules (#70).
- New enroll JSON fields if PEM concatenation suffices.
- Putting the root private key on hypervisors or in this git repo.
- Name Constraints that encode Proxmox cluster identity (Q3 decided: omit).
- HSM for the beta (Q1).

## Risks

- **Ops / D8:** every node alias and registration that today is CN-only must
  gain `tpm_devid:ca:fingerprint:<cluster intermediate>`; rotating an
  intermediate requires updating those selectors before guests renew onto the
  new chain. Missing the fingerprint selector silently regresses to Q0(A).
- Shipping the intermediate in SPIRE `devid_ca_path` (as a trust anchor)
  defeats Q0(B) — server must keep the offline **root** only.
- Forgetting the intermediate in the guest PEM breaks attest while root-only
  leaves still work — confusing during migration.
- **Intra-cluster:** any node with the cluster intermediate can forge any VM
  ID in that cluster. Not in scope.
