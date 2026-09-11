# Remote Development Setup für hogan (10.7.10.5)

Projekt: **vtpm-mds** (IMDS + TPM attestation + DevID).  
Lokales QEMU-Lab ohne Proxmox: siehe `scripts/qemu-lab/README.md` und Root-`README.md`.

## Schritt 1: SSH-Konfiguration auf dem Mac

Füge folgende Zeilen zu `~/.ssh/config` hinzu:

```ssh-config
Host hogan
    HostName 10.7.10.5
    User root
    IdentityFile ~/.ssh/id_rsa
    ForwardAgent yes
```

**Teste die Verbindung:**
```bash
ssh hogan
```

## Schritt 2: Projekt auf hogan vorbereiten

```bash
# Auf hogan
ssh hogan

# Projekt klonen (oder von lokal pushen)
cd /opt
git clone https://github.com/dembaca/vtpm-mds.git
cd vtpm-mds

# Go installieren (falls nicht vorhanden)
apt-get update
apt-get install -y golang-go

# Dependencies installieren
make deps
```

## Schritt 3: Cursor Remote SSH einrichten

1. **Extension installieren:**
   - Öffne Cursor
   - `Cmd+Shift+X` → Suche nach "Remote - SSH"
   - Installiere die Extension

2. **Mit hogan verbinden:**
   - `Cmd+Shift+P` → "Remote-SSH: Connect to Host"
   - Wähle `hogan` aus der Liste
   - Cursor öffnet ein neues Fenster

3. **Workspace öffnen:**
   - Im Remote-Fenster: `File → Open Folder`
   - Wähle `/opt/vtpm-mds`
   - Fertig! 🎉

## Schritt 4: Go Extension installieren (im Remote-Fenster)

Im Remote-Fenster:
- `Cmd+Shift+X` → Suche nach "Go"
- Installiere die Extension (wird auf hogan installiert)

## Testen

```bash
# Im Remote-Terminal (Cursor)
make build
make test
```

## Workflow

1. **Entwickeln:** Direkt in Cursor auf hogan
2. **Testen:** `make test` im Terminal
3. **Builden:** `make build`
4. **Ausführen:** `sudo ./bin/vtpm-mds`

## Tipps

- **Git:** Funktioniert normal, alle Commits gehen direkt ins Repo
- **Terminal:** `Ctrl+`` öffnet integriertes Terminal
- **Go Tools:** Werden automatisch auf hogan installiert
- **TPM-Zugriff:** Direkt verfügbar, da du auf dem Proxmox-Host arbeitest

## Troubleshooting

**Problem: "Host key verification failed"**
```bash
ssh-keygen -R 10.7.10.5
ssh hogan  # Akzeptiere den Fingerprint
```

**Problem: Go nicht gefunden**
```bash
# Auf hogan
which go
# Falls nicht vorhanden:
apt-get install -y golang-go
```

**Problem: Permission denied**
```bash
# Stelle sicher, dass du als root verbunden bist
ssh hogan
whoami  # sollte "root" sein
```

