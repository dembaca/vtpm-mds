# Development

## Prerequisites

- Go 1.21+
- Make

## Build Commands

```bash
make deps      # Download Go dependencies
make build     # Build binary to bin/prox-mds
make test      # Run all tests
make run       # Build and run (requires sudo)
make deb       # Build Debian package
```

## Running Tests

```bash
# All tests
make test

# Single package
go test -v ./imds

# Single test
go test -v ./imds -run TestTokenStore

# With coverage
go test -cover ./...
```

## Local Development (macOS)

```bash
brew install go
make deps
make build
make test
```

Note: Running the service locally requires sudo and network configuration for link-local addressing.

## Remote Development (Proxmox)

For testing on actual Proxmox infrastructure, remote development is recommended.

### SSH Configuration

Add to `~/.ssh/config`:

```ssh-config
Host proxmox-dev
    HostName <your-proxmox-ip>
    User root
    IdentityFile ~/.ssh/id_rsa
    ForwardAgent yes
```

### VS Code / Cursor Remote SSH

1. Install "Remote - SSH" extension
2. `Cmd+Shift+P` -> "Remote-SSH: Connect to Host"
3. Select your configured host
4. Open `/opt/prox-mds` folder

### Setup on Proxmox Host

```bash
ssh proxmox-dev
cd /opt
git clone https://github.com/dembaca/prox-mds.git
cd prox-mds

# Install Go if needed
apt-get update && apt-get install -y golang-go

make deps
make build
make test
```

### Sync Workflow

For quick iteration without commits:

```bash
# From local machine
rsync -avz --exclude '.git' --exclude 'bin' ./ proxmox-dev:/opt/prox-mds/
ssh proxmox-dev "cd /opt/prox-mds && make build && make test"
```

Or use the Makefile targets:

```bash
make sync         # rsync to remote
make remote-test  # Run tests on remote
```

## Debugging

### Remote Debugging with Delve

On Proxmox:
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/prox-mds --headless --listen=:2345 --api-version=2
```

Connect VS Code/Cursor debugger to port 2345.

## Project Structure

```
cmd/prox-mds/main.go     Entry point
internal/
  config/config.go       Configuration parsing
  server/server.go       HTTP server and routing
imds/                    Metadata endpoint handlers
```

## Recommended Extensions

- Go (golang.go)
- Remote - SSH
- GitLens
- YAML
