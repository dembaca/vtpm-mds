# prox-mds — Proxmox Metadata & Attestation Service

## 🧩 Overview

**prox-mds** is a lightweight **metadata and attestation service** for Proxmox VE environments.
It provides cloud-style **Instance Metadata Service (IMDS)** functionality and a **TPM-anchored workload identity** layer for virtual machines.

The goal is to enable **secure workload identity** (SPIFFE/SPIRE, Teleport, Vault, Kubernetes) inside DG-i’s private cloud without depending on external cloud infrastructure.

---

## 🎯 Key Goals

- **Cloud-compatible metadata API** — an EC2-IMDSv2-compatible endpoint for cloud-init and generic tooling.
- **TPM-anchored attestation** — verify VM identity and integrity using vTPM (swtpm) quotes and EKCerts.
- **Short-lived identity documents (JWT/JWS)** — signed by host TPM-sealed keys, consumable by SPIRE, Teleport, and Vault.
- **Secure multi-tenant isolation** — per-bridge IMDS interface with L2 isolation and iif-based nftables filters.
- **Host-anchored trust** — host TPM keys seal the attestation signing material.

---

## 🧠 Architecture Summary

```
+-------------------------------------------------------------+
|                     Control Plane / Consumers               |
|   SPIRE / Teleport / Vault / K8s Webhooks / Brokers         |
+-----------------------------▲-------------------------------+
                              │  verify JWT / JWS (JWKS)
                              │
+-----------------------------│-------------------------------+
|                Proxmox Host (pmx-mds daemon)                 |
|  - swtpm_localca (issues EKCerts via DG-i EK CA)             |
|  - prox-mds service:                                         |
|      • /latest/api/token (IMDSv2)                            |
|      • /latest/meta-data/* (EC2-compatible)                  |
|      • /latest/attest/* (TPM attestation)                    |
|      • /latest/identity (signed JWT/JWS)                     |
|  - Host key sealed to physical TPM (PCR policy)              |
|  - nftables: iif filter + DNAT 169.254.169.254 → 169.254.169.1 |
+-----------------------------▲-------------------------------+
                              │
                 Dedicated bridge: vmbr_imds
                              │
+-----------------------------│-------------------------------+
|                  Virtual Machines (with vTPM)                |
|  - swtpm-backed TPM 2.0 device                               |
|  - Agents: SPIRE / Teleport / Vault                          |
|  - Secondary NIC → vmbr_imds (no VLAN dependency)            |
|  - Calls prox-mds endpoints to get attested identity         |
+-------------------------------------------------------------+
```

---

## ⚙️ Key Endpoints

| Endpoint | Method | Description | Compatibility |
|-----------|---------|--------------|---------------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token | ✅ EC2-compatible |
| `/latest/meta-data/*` | GET | Instance metadata tree | ✅ EC2-compatible |
| `/latest/dynamic/instance-identity/document` | GET | EC2-style IID (shape-compatible) | ✅ partial |
| `/latest/dynamic/instance-identity/signature` | GET | PKCS#7 signature (DG-i Attestation CA) | ⚠️ AWS-incompatible |
| `/latest/attest/nonce` | GET | Request nonce for TPM quote | 🔒 attestation |
| `/latest/attest` | POST | Submit TPM quote + EKCert for validation | 🔒 attestation |
| `/latest/identity` | GET | Retrieve DG-i signed JWT/JWS identity | 🔒 attestation |
| `/.well-known/jwks.json` | GET | Public JWKS for verifiers | 🔒 attestation |

---

## 🔐 Trust Model

- **Cluster EK Root CA** → issues **EK Issuing CAs** for each host.
- Each **swtpm_localca** signs EKCerts for vTPMs on that host.
- **prox-mds** validates TPM quotes and signs identity JWTs with a key **sealed to the host’s physical TPM**.
- Consumers (Vault, SPIRE, Teleport) validate JWTs using the **Host Attestation Root CA** (JWKS).

---

## 🧰 Implementation Notes

### Languages / Components
- **Language:** Go (preferred)
- **API layer:** `net/http` or `echo` with clear route grouping.
- **Crypto:** Go `x509`, `crypto/tpm2`, `jose`.
- **Signing:** TPM2 policy-sealed keys (via `tpm2-tools` or `go-tpm`).
- **Logging/Tracing:** structured JSON logs for SIEM.

### Configuration
```yaml
mds:
  listen_addr: 169.254.169.1:80
  jwks_path: /var/lib/prox-mds/jwks.json
  attestation_ca: /etc/prox-mds/attestation-ca.pem
  ek_ca_chain: /etc/prox-mds/ek-chain.pem
  token_ttl: 60s
  jwt_ttl: 5m
  enable_ec2_compat: true
  enable_tpm_attestation: true
```

---

## 🧩 Integration Targets

| System | Purpose | Integration |
|---------|----------|-------------|
| **SPIRE** | Node attestation via JWT | trust JWKS from prox-mds |
| **Teleport** | Join via TPM join method | restrict to DG-i EK CA |
| **Vault** | `auth/jwt` with bound claims (`level`, `ek_hash`) | trust prox-mds JWKS |
| **Kubernetes** | Node attestation controller → labels | uses `/identity` JWTs |
| **Proxmox** | Hook scripts to create vTPMs and populate EKCerts | integrate at VM create |

---

## 🚀 Roadmap

| Phase | Description | Status |
|--------|--------------|--------|
| 1 | Minimal IMDSv2 API with EC2-compatible paths | ☐ |
| 2 | TPM attestation endpoints and quote verification | ☐ |
| 3 | JWT/JWS identity issuance + JWKS publishing | ☐ |
| 4 | TPM key sealing for host-level signing | ☐ |
| 5 | Proxmox hook integration (`swtpm_localca`) | ☐ |
| 6 | End-to-end SPIRE / Vault / Teleport integration demos | ☐ |

---

## 📚 Reference Links

- [SWTPM GitHub](https://github.com/stefanberger/swtpm)
- [Teleport TPM Join Docs](https://goteleport.com/docs/machine-workload-identity/machine-id/deployment/linux-tpm/)
- [SPIRE TPM DevID Plugin](https://github.com/spiffe/spire/blob/main/doc/plugin_server_nodeattestor_tpm_devid.md)
- [AWS IMDSv2 API Reference](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
- [HashiCorp Vault JWT Auth Method](https://developer.hashicorp.com/vault/docs/auth/jwt)

---

## 🧱 License & Maintainers

- **License:** Apache 2.0 (draft)
- **Maintainers:** DG-i Platform Engineering  
  Contact: `andreas.dembach@dg-i.net`

---

> _This file defines the architectural intent and API surface of the prox-mds service. Developers should treat it as the design anchor for implementation and integration with Proxmox, Vault, SPIRE, and Teleport._
