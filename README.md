# prox-mds

A lightweight EC2-IMDSv2-compatible Instance Metadata Service (IMDS) for Proxmox VE environments.

## Overview

prox-mds provides cloud-style metadata endpoints for virtual machines running on Proxmox VE, enabling workloads to retrieve instance information using familiar AWS SDK patterns and cloud-init compatibility.

## Features

- **EC2-IMDSv2 compatible API** - Token-based authentication for metadata requests
- **Instance metadata** - Instance ID, hostname, IP addresses, availability zone
- **Identity documents** - EC2-style instance identity document endpoint
- **Link-local addressing** - Standard 169.254.169.254 metadata endpoint

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
  token_ttl: "60s"
  enable_ec2_compat: true
```

### Run

```bash
sudo ./bin/prox-mds
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/latest/api/token` | PUT | Issue short-lived IMDSv2 token |
| `/latest/meta-data/instance-id` | GET | Instance identifier |
| `/latest/meta-data/local-hostname` | GET | Instance hostname |
| `/latest/meta-data/local-ipv4` | GET | Private IP address |
| `/latest/meta-data/public-ipv4` | GET | Public IP address |
| `/latest/meta-data/placement/availability-zone` | GET | Availability zone |
| `/latest/meta-data/services/domain` | GET | Service domain |
| `/latest/dynamic/instance-identity/document` | GET | Instance identity document |
| `/health` | GET | Health check endpoint |

## Usage Example

```bash
# Get session token
TOKEN=$(curl -X PUT -H "X-aws-ec2-metadata-token-ttl-seconds: 60" \
  http://169.254.169.254/latest/api/token)

# Fetch metadata
curl -H "X-aws-ec2-metadata-token: $TOKEN" \
  http://169.254.169.254/latest/meta-data/instance-id
```

## License

Apache 2.0

## Maintainers

DG-i Platform Engineering - `andreas.dembach@dg-i.net`
