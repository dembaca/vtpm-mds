# prox-mds — Proxmox Metadata & Attestation Service

A lightweight **metadata and attestation service** for Proxmox VE environments, providing cloud-style **Instance Metadata Service (IMDS)** functionality and **TPM-anchored workload identity** for virtual machines.

## Features

- **Cloud-compatible metadata API** — EC2-IMDSv2-compatible endpoints
- **TPM attestation** — Verify VM identity and integrity using vTPM (swtpm) quotes
- **Short-lived identity documents (JWT/JWS)** — Signed by host TPM-sealed keys
- **Secure multi-tenant isolation** — Per-bridge IMDS interface
- **Host-anchored trust** — Host TPM keys seal the attestation signing material

## Quick Start

### Build

```bash
make build
```

### Configuration

Create `/etc/prox-mds/config.yaml`:

```yaml
mds:
  listen_addr: "169.254.169.1:80"
  jwks_path: "/var/lib/prox-mds/jwks.json"
  token_ttl: "60s"
  jwt_ttl: "5m"
  enable_ec2_compat: true
  enable_tpm_attestation: true
```

### Run

```bash
sudo ./bin/prox-mds
```

## API Endpoints

### IMDSv2 Compatible

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token |
| `/latest/meta-data/*` | GET | Instance metadata |
| `/latest/dynamic/instance-identity/document` | GET | EC2-style IID |
| `/latest/dynamic/instance-identity/signature` | GET | PKCS#7 signature |

### Attestation

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/attest/nonce` | GET | Request nonce for TPM quote |
| `/latest/attest` | POST | Submit TPM quote for validation |
| `/latest/identity` | GET | Retrieve signed JWT/JWS identity |
| `/.well-known/jwks.json` | GET | Public JWKS for verifiers |

## Development

```bash
# Install dependencies
make deps

# Build
make build

# Test
make test

# Debian Package
make deb          # Build package (auto-detects macOS/Linux)
make deb-native    # Build natively on Linux
make deb-docker    # Build in Docker container
make deb-clean     # Clean build artifacts
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/handlers
```

## Roadmap

- [x] Basic IMDSv2 API
- [x] TPM attestation endpoints
- [ ] TPM quote verification
- [ ] JWT signing with TPM keys
- [ ] Proxmox integration (hooks)
- [ ] SPIRE/Teleport/Vault integration demos

## License

Apache 2.0 — See LICENSE file

## Maintainers

DG-i Platform Engineering — `andreas.dembach@dg-i.net`

