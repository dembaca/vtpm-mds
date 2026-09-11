# qemu-mds / prox-mds — QEMU Metadata & Attestation Service

A lightweight **metadata and attestation service** for QEMU-based hosting environments
(Proxmox VE is a first-class inventory backend). It provides cloud-style **Instance
Metadata Service (IMDS)** functionality and **TPM-anchored workload identity** for VMs,
including SPIRE-compatible **TPM DevID** enrollment.

Binary names: `prox-mds` (legacy) and `qemu-mds` (preferred alias). Module path remains
`github.com/dembaca/prox-mds`.

## Features

- **Cloud-compatible metadata API** — EC2-IMDSv2-compatible endpoints
- **TPM attestation** — Verify VM identity using vTPM (swtpm) quotes
- **TPM DevID enrollment** — Issue LDevID certs + TPM2B blobs for SPIRE `tpm_devid`
- **Dual enroll auth** — MAC → inventory **and** TPM EK certificate header
- **Short-lived identity documents (JWT/JWS)** — Signed identity for consumers
- **Inventory backends** — YAML inventory (plain QEMU lab) or Proxmox `/etc/pve`
- **Host-anchored trust** — EK CA chain + optional DevID CA

## Quick Start

### Build

```bash
make build
# also builds: bin/qemu-mds (copy/alias), bin/devid-enroll (guest client)
```

### Configuration

Create `/etc/prox-mds/config.yaml` (see `config.yaml` for the full template):

```yaml
mds:
  listen_addr: "169.254.169.1:80"
  jwks_path: "/var/lib/prox-mds/jwks.json"
  attestation_ca: "/etc/prox-mds/attestation-ca.pem"
  ek_ca_chain: "/etc/prox-mds/ek-chain.pem"
  devid_ca_cert: "/etc/prox-mds/devid-ca.pem"
  devid_ca_key: "/etc/prox-mds/devid-ca-key.pem"
  token_ttl: "60s"
  jwt_ttl: "5m"
  enable_ec2_compat: true
  enable_tpm_attestation: true
  # Prefer YAML on non-Proxmox hosts; empty → Proxmox /etc/pve
  inventory_path: "/var/lib/mds-lab/inventory/lab.yaml"
```

### Run

```bash
sudo ./bin/qemu-mds -config /etc/prox-mds/config.yaml
# or: sudo ./bin/prox-mds -config /etc/prox-mds/config.yaml
```

## API Endpoints

### IMDSv2 Compatible

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token |
| `/latest/meta-data/*` | GET | Instance metadata |
| `/latest/dynamic/instance-identity/document` | GET | EC2-style IID |
| `/latest/dynamic/instance-identity/signature` | GET | PKCS#7 signature |

### Attestation / Identity

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/attest/nonce` | GET | Request nonce for TPM quote |
| `/latest/attest` | POST | Submit TPM quote for validation |
| `/latest/identity` | GET | Retrieve signed JWT/JWS identity |
| `/.well-known/jwks.json` | GET | Public JWKS for verifiers |

### DevID Enrollment (SPIRE `tpm_devid`)

Registered when `devid_ca_cert` / `devid_ca_key` are set and TPM attestation is enabled.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/devid/enroll/start` | POST | Verify CSR + return EK credential challenge |
| `/latest/devid/enroll/finish` | POST | Verify challenge + issue LDevID PEM |

**Authentication (both calls):**

1. **MAC → inventory** — caller identified via ARP / ConnContext (same as metadata)
2. **`X-qemu-mds-ek-cert`** — base64(DER) of the guest TPM Endorsement Key certificate;
   must chain to `ek_ca_chain`, match the CSR EK on `start`, and match the session on `finish`

Optional inventory pin: `ek_sha256` (hex SHA-256 of EK cert DER).

Guest client: `bin/devid-enroll` (cloud-init oneshot in the QEMU lab). Outputs:

- `devid.crt.pem`
- `devid.priv.blob` / `devid.pub.blob` (TPM2B, SPIRE paths)

## Development

```bash
make deps
make build
make test

# QEMU/netns lab (no Proxmox required)
./scripts/qemu-lab/cloud-install.sh
./scripts/qemu-lab/cloud-start.sh
sudo ./bin/qemu-mds -config /etc/prox-mds/config.lab.yaml &
make lab-e2e          # fast IMDS netns smoke
make lab-devid-e2e    # full guest vTPM DevID enrollment
```

See [`scripts/qemu-lab/README.md`](scripts/qemu-lab/README.md) for the nested-guest lab.

Architecture notes: [`prox-mds_README.md`](prox-mds_README.md).  
Remote/Proxmox workflows: [`DEVELOPMENT.md`](DEVELOPMENT.md), [`SETUP_REMOTE.md`](SETUP_REMOTE.md).

## Testing

```bash
go test ./...
go test ./internal/devid/ ./internal/inventory/ -count=1
```

## Roadmap

- [x] Basic IMDSv2 API
- [x] TPM attestation endpoints
- [x] YAML inventory + QEMU lab (Proxmox optional)
- [x] TPM DevID enrollment (go-tpm) + cloud-init client
- [x] Dual auth: MAC inventory + EK certificate
- [ ] TPM quote verification hardening
- [ ] JWT signing with host TPM-sealed keys
- [ ] Proxmox hook integration
- [ ] Full SPIRE/Teleport/Vault demos

## License

Apache 2.0 — See LICENSE file

## Maintainers

DG-i Platform Engineering — `andreas.dembach@dg-i.net`
