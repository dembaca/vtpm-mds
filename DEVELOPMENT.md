# Development Setup

## Remote Development Options

### Option 1: Cursor/VS Code Remote SSH (Empfohlen)

**Vorteile:**
- Direkt auf Proxmox entwickeln
- Kein Datei-Kopieren nötig
- Go-Tools laufen direkt auf dem Server
- TPM-Zugriff möglich (wenn auf Proxmox-Host)

**Setup:**

1. **SSH-Konfiguration auf dem Mac** (`~/.ssh/config`):
```ssh-config
Host hogan
    HostName 10.7.10.5
    User root
    IdentityFile ~/.ssh/id_rsa
    ForwardAgent yes
```

2. **In Cursor/VS Code:**
   - Installiere Extension: "Remote - SSH" (falls nicht vorhanden)
   - `Cmd+Shift+P` → "Remote-SSH: Connect to Host"
   - Wähle `hogan`
   - Öffne den Workspace-Ordner auf dem Server

3. **Projekt auf Proxmox klonen:**
```bash
# Auf hogan (10.7.10.5)
ssh hogan
cd /opt
git clone https://github.com/dembaca/prox-mds.git
cd prox-mds
```

4. **Go installieren (falls nicht vorhanden):**
```bash
# Auf Proxmox (Debian/Ubuntu)
apt-get update
apt-get install -y golang-go
```

### Option 2: Git-basierter Workflow

**Workflow:**
```bash
# Auf Mac: Entwickeln
git add .
git commit -m "feature"
git push

# Auf Proxmox: Pullen und testen
ssh proxmox-dev
cd /opt/prox-mds
git pull
make build
make test
```

**Vorteile:**
- Einfach
- Versionierung automatisch
- Keine zusätzlichen Tools nötig

## Lokale Entwicklung (Mac)

### Voraussetzungen
```bash
# Go installieren
brew install go

# Dependencies
make deps
```

### Build & Test
```bash
make build    # Binary bauen
make test      # Tests laufen lassen
make run       # Lokal starten (benötigt sudo)
```

## Remote Testing Workflow

### Schneller Test-Zyklus

1. **Code auf Mac entwickeln**
2. **Via Git pushen**
3. **Auf Proxmox pullen und testen:**
```bash
ssh proxmox-dev "cd /opt/prox-mds && git pull && make build && sudo ./bin/prox-mds"
```

### Oder mit rsync (schneller für Tests):
```bash
# Von Mac aus
rsync -avz --exclude '.git' --exclude 'bin' ./ proxmox-dev:/opt/prox-mds/
ssh proxmox-dev "cd /opt/prox-mds && make build"
```

## TPM Development

Für TPM-Entwicklung benötigst du Zugriff auf:
- `/dev/tpm0` (VM TPM)
- `/dev/tpmrm0` (Host TPM)

**Wichtig:** Remote SSH funktioniert am besten, wenn du direkt auf dem Proxmox-Host arbeitest.

## Debugging

### Remote Debugging mit Delve
```bash
# Auf Proxmox
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/prox-mds --headless --listen=:2345 --api-version=2

# Auf Mac: VS Code/Cursor mit Go Debugger verbinden
```

## Empfohlene Extensions (Cursor/VS Code)

- Go (golang.go)
- Remote - SSH
- GitLens
- YAML

