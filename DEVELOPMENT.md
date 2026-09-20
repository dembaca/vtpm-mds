# Development Setup

Entwicklungs- und Remote-Workflows für **vtpm-mds** (IMDS + TPM attestation + DevID).

Lokales QEMU-Lab ohne Proxmox: [`scripts/qemu-lab/README.md`](scripts/qemu-lab/README.md).
Architektur und Trust-Modell: [`ARCHITECTURE.md`](ARCHITECTURE.md).
Verhalten der Endpunkte: [`openspec/specs/`](openspec/specs/).

---

## Option 1: Cursor/VS Code Remote SSH (empfohlen)

**Vorteile:**
- Direkt auf dem Proxmox-Host entwickeln
- Kein Datei-Kopieren nötig
- Go-Tools laufen direkt auf dem Server
- TPM-Zugriff möglich

### Schritt 1: SSH-Konfiguration auf dem Mac

In `~/.ssh/config`:

```ssh-config
Host dev-host
    HostName 192.168.100.10
    User username
    IdentityFile ~/.ssh/id_rsa
    ForwardAgent yes
```

Verbindung testen:

```bash
ssh dev-host
```

### Schritt 2: Projekt auf dem Host vorbereiten

```bash
ssh dev-host

cd /opt
git clone https://github.com/dembaca/vtpm-mds.git
cd vtpm-mds

# Go installieren (falls nicht vorhanden)
apt-get update
apt-get install -y golang-go

make deps
```

### Schritt 3: Cursor Remote SSH einrichten

1. **Extension installieren:** `Cmd+Shift+X` → "Remote - SSH"
2. **Verbinden:** `Cmd+Shift+P` → "Remote-SSH: Connect to Host" → `dev-host`
3. **Workspace öffnen:** `File → Open Folder` → `/opt/vtpm-mds`
4. **Go-Extension** im Remote-Fenster installieren (`Cmd+Shift+X` → "Go") —
   wird auf dem Host installiert, nicht lokal

### Schritt 4: Testen

```bash
# Im Remote-Terminal (Ctrl+`)
make build
make test
sudo ./bin/vtpm-mds -config /etc/vtpm-mds/config.yaml
```

---

## Option 2: Git-basierter Workflow

```bash
# Auf Mac: Entwickeln
git add .
git commit -m "feature"
git push

# Auf dem Host: Pullen und testen
ssh dev-host
cd /opt/vtpm-mds
git pull
make build
make test
```

Einzeiler für den schnellen Zyklus:

```bash
ssh dev-host "cd /opt/vtpm-mds && git pull && make build && sudo ./bin/vtpm-mds"
```

Oder mit `rsync`, wenn Commits im Weg sind:

```bash
rsync -avz --exclude '.git' --exclude 'bin' ./ username@dev-host:/opt/vtpm-mds/
ssh dev-host "cd /opt/vtpm-mds && make build"
```

---

## Lokale Entwicklung (Mac)

```bash
brew install go
make deps

make build    # Binary bauen
make test     # Tests laufen lassen
make run      # Lokal starten (benötigt sudo)
```

Ohne TPM-Hardware lassen sich Unit-Tests und der IMDS-Pfad lokal ausführen;
für DevID/Attestation wird das QEMU-Lab oder der Proxmox-Host gebraucht.

---

## TPM-Entwicklung

Benötigt Zugriff auf:

- `/dev/tpm0` bzw. `/dev/tpmrm0` (Guest-/Host-TPM)
- Lab ohne Proxmox: `scripts/qemu-lab/` (swtpm + QEMU/TCG)

**DevID-Client (Guest):**

```bash
make build   # erzeugt auch bin/devid-enroll
# Guest-Oneshot (Lab) authentifiziert mit MAC + Header X-vtpm-mds-ek-cert
sudo ./scripts/qemu-lab/e2e-devid-guest.sh
```

**Wichtig:** Remote SSH funktioniert am besten direkt auf dem Proxmox-Host.
Cloud-Agent-Lab: nested KVM ist oft kaputt → TCG; IMDS-Smoke über `e2e-netns.sh`.

---

## Debugging

Remote Debugging mit Delve:

```bash
# Auf dem Host / Lab-Host
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug . --headless --listen=:2345 --api-version=2 -- -config /etc/vtpm-mds/config.yaml
```

---

## Troubleshooting

**"Host key verification failed"**

```bash
ssh-keygen -R 192.168.100.10
ssh dev-host  # Fingerprint akzeptieren
```

**Go nicht gefunden**

```bash
ssh dev-host
which go
apt-get install -y golang-go   # falls nicht vorhanden
```

**Permission denied**

```bash
ssh dev-host
whoami  # sollte "username" sein
```

---

## Empfohlene Extensions (Cursor/VS Code)

- Go (golang.go)
- Remote - SSH
- GitLens
- YAML
