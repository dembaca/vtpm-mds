# CLAUDE.md

Instructions for Claude Code when working with this repository.

## Project

prox-mds is an EC2-IMDSv2-compatible Instance Metadata Service for Proxmox VE. It provides metadata endpoints that allow VMs to retrieve instance information using AWS SDK patterns.

## Build & Test

```bash
make deps      # Download dependencies
make build     # Build to bin/prox-mds
make test      # Run all tests
```

Single test: `go test -v ./imds -run TestTokenStore`

## Code Structure

```
cmd/prox-mds/main.go        Entry point, config loading, graceful shutdown
internal/config/config.go   YAML config parsing
internal/server/server.go   HTTP server, route registration, middleware
imds/                       Metadata handlers (token.go, handlers.go)
```

## Key Components

- **TokenStore** (`imds/token.go`): IMDSv2 session token management with TTL
- **Server** (`internal/server/server.go`): Route registration and middleware

## API Flow

1. `PUT /latest/api/token` - get session token
2. `GET /latest/meta-data/*` - fetch metadata (requires `X-Aws-Ec2-Metadata-Token` header)

## Configuration

File: `/etc/prox-mds/config.yaml`

```yaml
mds:
  listen_addr: "169.254.169.1:80"
  token_ttl: "60s"
  enable_ec2_compat: true
```
