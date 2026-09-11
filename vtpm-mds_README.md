# vtpm-mds — Architecture & Design Anchor

## Overview

**vtpm-mds** is a lightweight **metadata and attestation service**
for QEMU-based hosting. **Proxmox VE** is a special inventory/hosting subclass (parse
`/etc/pve`); plain QEMU labs use a YAML inventory.

It provides cloud-style **Instance Metadata Service (IMDS)** functionality and a
**TPM-anchored workload identity** layer for virtual machines—including issuance of
**IEEE 802.1AR / TCG LDevID** certificates for SPIRE’s `tpm_devid` node attestor.

The goal is secure workload identity (SPIFFE/SPIRE, Teleport, Vault, Kubernetes) in
private cloud without depending on external cloud IMDS.

---

## Key Goals

- **Cloud-compatible metadata API** — EC2-IMDSv2-compatible endpoint for cloud-init and tooling
- **TPM-anchored attestation** — verify VM identity using vTPM (swtpm) quotes and EKCerts
- **TPM DevID provisioning** — modern go-tpm reimplementation of HP-style LDevID enrollment over IMDS HTTP (not gRPC)
- **Short-lived identity documents (JWT/JWS)** — consumable by SPIRE, Teleport, and Vault
- **Caller binding** — MAC → inventory for tenant isolation; DevID enroll also requires EK cert auth
- **Host-anchored trust** — EK CA chain + DevID CA; host TPM sealing for JWT keys (roadmap)

---

## Architecture Summary

```
+-------------------------------------------------------------+
|                     Control Plane / Consumers               |
|   SPIRE (tpm_devid / JWT) / Teleport / Vault / K8s          |
+-----------------------------▲-------------------------------+
                              │  JWKS / DevID trust
                              │
+-----------------------------│-------------------------------+
|           QEMU host (Proxmox or plain QEMU lab)             |
|  - swtpm_localca (issues EKCerts)                           |
|  - vtpm-mds:                                     |
|      • /latest/api/token (IMDSv2)                           |
|      • /latest/meta-data/* (EC2-compatible)                 |
|      • /latest/attest/* (TPM attestation)                   |
|      • /latest/identity (signed JWT/JWS)                    |
|      • /latest/devid/enroll/{start,finish} (LDevID)         |
|  - Inventory: YAML path  -or-  Proxmox /etc/pve             |
|  - nftables/iptables: DNAT 169.254.169.254 → listen addr    |
+-----------------------------▲-------------------------------+
                              │
                 Dedicated bridge (br-imds / vmbr_imds)
                              │
+-----------------------------│-------------------------------+
|                  Virtual Machines (with vTPM)                |
|  - swtpm-backed TPM 2.0                                     |
|  - cloud-init oneshot: devid-enroll                         |
|      auth: MAC + X-vtpm-mds-ek-cert                         |
|  - SPIRE agent: tpm_devid materials                         |
|  - Secondary NIC on IMDS bridge                             |
+-------------------------------------------------------------+
```

---

## Key Endpoints

| Endpoint | Method | Description | Compatibility |
|-----------|---------|--------------|---------------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token | EC2-compatible |
| `/latest/meta-data/*` | GET | Instance metadata tree | EC2-compatible |
| `/latest/dynamic/instance-identity/document` | GET | EC2-style IID | partial |
| `/latest/dynamic/instance-identity/signature` | GET | PKCS#7 signature | AWS-incompatible |
| `/latest/attest/nonce` | GET | Request nonce for TPM quote | attestation |
| `/latest/attest` | POST | Submit TPM quote + EKCert | attestation |
| `/latest/identity` | GET | Retrieve signed JWT/JWS identity | attestation |
| `/.well-known/jwks.json` | GET | Public JWKS for verifiers | attestation |
| `/latest/devid/enroll/start` | POST | DevID CSR → credential challenge | SPIRE DevID |
| `/latest/devid/enroll/finish` | POST | Challenge response → LDevID PEM | SPIRE DevID |

### DevID enroll authentication

Both enroll calls require:

1. **MAC → inventory** VM identity (ConnContext / ARP), same binding as metadata
2. Header **`X-vtpm-mds-ek-cert`**: base64(DER) of the TPM EK certificate, trusted via
   `ek_ca_chain`, matching the CSR (start) and the enroll session (finish)

Optional YAML inventory field `ek_sha256` pins a specific EK certificate to a VM.

Guest client (`cmd/devid-enroll`) performs credential activation locally and writes:

| File | Use |
|------|-----|
| `devid.crt.pem` | LDevID certificate |
| `devid.priv.blob` | TPM2B_PRIVATE for SPIRE |
| `devid.pub.blob` | TPM2B_PUBLIC for SPIRE |

---

## Trust Model

- **Cluster EK Root CA** → issues **EK Issuing CAs** per host (lab: swtpm-localca chain).
- Each **swtpm_localca** signs EKCerts for vTPMs on that host.
- **vtpm-mds** verifies EKCerts on DevID enroll, runs TPM credential activation challenge,
  and issues LDevIDs from `devid_ca_*`.
- JWT path (separate): validates TPM quotes and signs identity JWTs; consumers trust JWKS.
- SPIRE `tpm_devid`: trusts the DevID CA and verifies residency via TPM blobs.

---

## Implementation Notes

### Languages / Components

- **Language:** Go
- **API:** `net/http` ServeMux
- **TPM:** `github.com/google/go-tpm` (legacy/tpm2 + credactivation) for SPIRE-compatible blobs
- **Inventory:** `internal/inventory` (YAML) / `internal/proxmox` (`/etc/pve`)

### Configuration

```yaml
mds:
  listen_addr: 169.254.169.1:80
  jwks_path: /var/lib/vtpm-mds/jwks.json
  attestation_ca: /etc/vtpm-mds/attestation-ca.pem
  ek_ca_chain: /etc/vtpm-mds/ek-chain.pem
  devid_ca_cert: /etc/vtpm-mds/devid-ca.pem
  devid_ca_key: /etc/vtpm-mds/devid-ca-key.pem
  token_ttl: 60s
  jwt_ttl: 5m
  enable_ec2_compat: true
  enable_tpm_attestation: true
  inventory_path: /var/lib/mds-lab/inventory/lab.yaml  # empty → Proxmox
```

See also root `config.yaml` and `scripts/qemu-lab/README.md`.

---

## Integration Targets

| System | Purpose | Integration |
|---------|----------|-------------|
| **SPIRE** | Node attestation via `tpm_devid` | DevID PEM + TPM2B blobs from enroll |
| **SPIRE** | JWT node/workload path | trust JWKS from vtpm-mds |
| **Teleport** | TPM join | restrict to EK CA |
| **Vault** | `auth/jwt` with bound claims | trust JWKS |
| **Kubernetes** | Node attestation labels | `/identity` JWTs |
| **Proxmox** | Inventory + hooks for vTPM/EKCert | `/etc/pve` fallback + future hooks |
| **Plain QEMU** | Lab / non-Proxmox hosts | YAML `inventory_path` |

---

## Roadmap

| Phase | Description | Status |
|--------|--------------|--------|
| 1 | Minimal IMDSv2 API with EC2-compatible paths | done |
| 2 | TPM attestation endpoints | done (hardening ongoing) |
| 3 | JWT/JWS identity + JWKS | partial |
| 4 | YAML inventory + QEMU lab; Proxmox as backend | done |
| 5 | TPM DevID enroll (go-tpm) + cloud-init client + EK/MAC auth | done |
| 6 | Host TPM key sealing for JWT signing | planned |
| 7 | Proxmox hook integration (`swtpm_localca`) | planned |
| 8 | End-to-end SPIRE / Vault / Teleport demos | planned |

---

## Reference Links

- [SWTPM](https://github.com/stefanberger/swtpm)
- [SPIRE TPM DevID Plugin](https://github.com/spiffe/spire/blob/main/doc/plugin_server_nodeattestor_tpm_devid.md)
- [AWS IMDSv2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
- [TCG DevID](https://trustedcomputinggroup.org/wp-content/uploads/TCG_IWG_DevID_v1r2_02dec2020.pdf)
- [HP DevID Provisioning Tool](https://github.com/HewlettPackard/devid-provisioning-tool/) (protocol inspiration; not vendored)

---

## License & Maintainers

- **License:** Apache 2.0
- **Maintainers:** DG-i Platform Engineering — `andreas.dembach@dg-i.net`

> Architectural intent and API surface for vtpm-mds. Treat as the design
> anchor for QEMU lab, Proxmox, Vault, SPIRE, and Teleport integration.
