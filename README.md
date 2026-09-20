# vtpm-mds — QEMU Metadata & Attestation Service

A lightweight **metadata and attestation service** for QEMU-based hosting environments
(Proxmox VE is a first-class inventory backend). It provides cloud-style **Instance
Metadata Service (IMDS)** functionality and **TPM-anchored workload identity** for VMs,
including SPIRE-compatible **TPM DevID** enrollment.

Binary: `vtpm-mds` (aliases: `qemu-mds`, `prox-mds`). Module path: `github.com/dembaca/vtpm-mds`.

## Features

| Capability | Status |
|---|---|
| **Cloud-compatible metadata API** — EC2-IMDSv2-compatible endpoints | implemented |
| **TPM DevID enrollment** — LDevID certs + TPM2B blobs for SPIRE `tpm_devid` | implemented |
| **Dual enroll auth** — MAC → inventory **and** TPM EK certificate header | implemented |
| **Inventory backends** — YAML inventory (plain QEMU lab) or Proxmox `/etc/pve` | implemented |
| **Host-anchored trust** — EK CA chain + optional DevID CA | implemented |
| **TPM attestation** — verify VM identity using vTPM (swtpm) quotes | endpoints present, quote verification [in progress](openspec/changes/implement-tpm-quote-verification/) |
| **Short-lived identity documents (JWT/JWS)** | endpoints present, real signing keys [in progress](openspec/changes/sign-identity-documents-with-real-keys/) |

## Quick Start

### Build

```bash
make build
# also builds aliases bin/qemu-mds, bin/prox-mds and guest client bin/devid-enroll
```

### Debian package

Requires Go 1.24+ and `debhelper`. `make deb` runs `dpkg-buildpackage` and copies the artifact to `dist/`:

```bash
make deb
# artifact: dist/vtpm-mds_<version>_<arch>.deb   (currently dist/vtpm-mds_0.1.0_amd64.deb)
sudo dpkg -i dist/vtpm-mds_*.deb
sudo systemctl start vtpm-mds   # enabled on install, not auto-started
```

CI builds that same amd64 `.deb` on Linux (`ubuntu-24.04`, not Darwin). A GitHub Release is created when you push tag `v<debian-version>` (for example `v0.1.0`) or run **Actions → Debian package → Run workflow** with **Publish GitHub Release**.

Ansible / lab-host download (private repo: GitHub auth required):

```bash
gh release download v0.1.0 --repo dembaca/vtpm-mds --pattern 'vtpm-mds_*_amd64.deb'
# or
# https://github.com/dembaca/vtpm-mds/releases/download/v0.1.0/vtpm-mds_0.1.0_amd64.deb
```

The unit listens on `169.254.169.1:80` (IMDS bridge). Packaged config points at Ansible-managed PKI under `/etc/ssl`. See `debian/README.Debian`.

### Configuration

Create `/etc/vtpm-mds/config.yaml` (see `config.yaml` for the full template):

```yaml
mds:
  listen_addr: "169.254.169.1:80"
  jwks_path: "/var/lib/vtpm-mds/jwks.json"
  attestation_ca: "/etc/vtpm-mds/attestation-ca.pem"
  ek_ca_chain: "/etc/vtpm-mds/ek-chain.pem"
  devid_ca_cert: "/etc/vtpm-mds/devid-ca.pem"
  devid_ca_key: "/etc/vtpm-mds/devid-ca-key.pem"
  token_ttl: "60s"
  jwt_ttl: "5m"
  enable_ec2_compat: true
  enable_tpm_attestation: true
  # Prefer YAML on non-Proxmox hosts; empty → Proxmox /etc/pve
  inventory_path: "/var/lib/mds-lab/inventory/lab.yaml"
```

### Run

```bash
sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.yaml
```

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token |
| `/latest/meta-data/*` | GET | Instance metadata tree |
| `/latest/dynamic/instance-identity/document` | GET | EC2-style instance identity document |
| `/latest/dynamic/instance-identity/signature` | GET | Detached signature over the document |
| `/latest/attest/nonce` | GET | Request nonce for a TPM quote |
| `/latest/attest` | POST | Submit TPM quote for verification |
| `/latest/identity` | GET | Retrieve signed JWT/JWS identity document |
| `/.well-known/jwks.json` | GET | Public JWKS for verifiers |
| `/latest/devid/enroll/start` | POST | Verify CSR, return EK credential challenge |
| `/latest/devid/enroll/finish` | POST | Verify challenge, issue LDevID PEM |
| `/health` | GET | Liveness probe (no token required) |

**Exact behaviour — authentication, status codes, response shapes and edge cases — is
specified in [`openspec/specs/`](openspec/specs/):**

- [`instance-metadata`](openspec/specs/instance-metadata/spec.md) — IMDSv2 token flow and the metadata tree
- [`devid-enrollment`](openspec/specs/devid-enrollment/spec.md) — the two-leg enroll protocol and its dual-factor auth
- [`vm-inventory`](openspec/specs/vm-inventory/spec.md) — how callers are identified and inventory is loaded

DevID enroll routes are registered when `devid_ca_cert` / `devid_ca_key` are set and
TPM attestation is enabled. The guest client is `bin/devid-enroll` (a cloud-init
oneshot in the QEMU lab); it writes `devid.crt.pem`, `devid.priv.blob` and
`devid.pub.blob` for SPIRE.

## Development

```bash
make deps
make build
make test

# QEMU/netns lab (no Proxmox required)
./scripts/qemu-lab/cloud-install.sh
./scripts/qemu-lab/cloud-start.sh
sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.lab.yaml &
make lab-e2e          # fast IMDS netns smoke
make lab-devid-e2e    # full guest vTPM DevID enrollment
```

See [`scripts/qemu-lab/README.md`](scripts/qemu-lab/README.md) for the nested-guest lab.

Architecture and trust model: [`ARCHITECTURE.md`](ARCHITECTURE.md).
Remote/Proxmox development workflows: [`DEVELOPMENT.md`](DEVELOPMENT.md).

### Spec-driven workflow

This project uses [OpenSpec](https://openspec.dev). `openspec/specs/` is the living
description of current behaviour; `openspec/changes/` holds in-flight work, each with a
proposal, design and task breakdown. A change merges into the specs when it lands, so
the specs never drift from what shipped.

```bash
openspec list            # active changes
openspec list --specs    # capability inventory
openspec validate --all
```

In an OpenSpec-aware agent: `/opsx:propose`, `/opsx:apply`, `/opsx:verify`, `/opsx:archive`.

## Testing

```bash
go test ./...
go test ./internal/devid/ ./internal/inventory/ -count=1
```

## License

Apache 2.0 — See LICENSE file

## Maintainers

DG-i Platform Engineering — `andreas.dembach@dg-i.net`
