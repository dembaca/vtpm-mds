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
| **TPM attestation** — verify VM identity using vTPM (swtpm) quotes | endpoints present, quote verification not implemented (unspecified) |
| **Short-lived identity documents (JWT/JWS)** | endpoints present, real signing keys not implemented (unspecified) |

## Quick Start

### Build

```bash
make build
# also builds aliases bin/qemu-mds, bin/prox-mds and guest client bin/devid-enroll
```

### Debian packages

One source package, two binary packages, built and released together from the same changelog version:

| Package | Install on | Contents |
|---|---|---|
| `vtpm-mds` | hypervisor / lab host | `/usr/sbin/vtpm-mds` (aliases `prox-mds`, `qemu-mds`), `vtpm-mds.service`, `/etc/vtpm-mds/config.yaml`, `/var/lib/vtpm-mds` |
| `devid-enroll` | guest VM images | `/usr/bin/devid-enroll` and its man page — no daemon, no systemd unit, no `/etc/vtpm-mds` |

**Breaking in 0.2.0:** the host package no longer ships `/usr/bin/devid-enroll` — upgrading `vtpm-mds` removes that path from the hypervisor. The client is its own package now; install `devid-enroll` where you actually need it (guest images, not hosts). It declares `Breaks`/`Replaces: vtpm-mds (<< 0.2.0)` so it can take over the moved path.

Requires Go 1.24+ and `debhelper`. `make deb` runs `dpkg-buildpackage` and copies both artifacts to `dist/`:

```bash
make deb
# artifacts: dist/vtpm-mds_<version>_<arch>.deb       (currently dist/vtpm-mds_0.2.0_amd64.deb)
#            dist/devid-enroll_<version>_<arch>.deb   (currently dist/devid-enroll_0.2.0_amd64.deb)
```

Host — daemon, unit and config:

```bash
sudo dpkg -i dist/vtpm-mds_*.deb
sudo systemctl start vtpm-mds   # enabled on install, not auto-started
```

Guest — client only, nothing to start:

```bash
sudo dpkg -i dist/devid-enroll_*.deb
devid-enroll -version
```

CI builds both amd64 `.deb`s on Linux (`ubuntu-24.04`, not Darwin) and fails if either is missing. A GitHub Release is created when you push tag `v<debian-changelog-version>` (for example `v0.2.0`) or run **Actions → Debian package → Run workflow** with **Publish GitHub Release**; both artifacts are attached to that tag.

Ansible / lab-host download — hosts fetch the daemon package (private repo: GitHub auth required):

```bash
gh release download v0.2.0 --repo dembaca/vtpm-mds --pattern 'vtpm-mds_*_amd64.deb'
# or
# https://github.com/dembaca/vtpm-mds/releases/download/v0.2.0/vtpm-mds_0.2.0_amd64.deb
```

Guest image builds fetch the client package as well (same tag, same version):

```bash
gh release download v0.2.0 --repo dembaca/vtpm-mds --pattern 'devid-enroll_*_amd64.deb'
# or
# https://github.com/dembaca/vtpm-mds/releases/download/v0.2.0/devid-enroll_0.2.0_amd64.deb
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
- [`debian-packaging`](openspec/specs/debian-packaging/spec.md) — the host and guest `.deb` split
- [`operability`](openspec/specs/operability/spec.md) — `-version` reporting and service supervision

These specs describe what the code does **today**, including where it is weaker than
the trust model in `ARCHITECTURE.md` implies. Read `devid-enrollment` before relying on
enrollment for tenant isolation.

DevID enroll routes are registered when `devid_ca_cert` / `devid_ca_key` are set and
TPM attestation is enabled. The guest client is `bin/devid-enroll` (a cloud-init
oneshot in the QEMU lab, and the `devid-enroll` package on real VM images); it
writes `devid.crt.pem`, `devid.priv.blob` and `devid.pub.blob` for SPIRE.

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
Copy-paste checks for IMDS / DevID / TPM / SPIRE: [E2E verification cheat sheet](#e2e-verification-cheat-sheet).

### Spec-driven workflow

This project uses [OpenSpec](https://openspec.dev). `openspec/specs/` is the living
description of current behaviour; `openspec/changes/` holds in-flight work, each with a
proposal, design and task breakdown. When a change lands, its delta moves into
`openspec/specs/` and the change moves to `openspec/changes/archive/`, so the specs
never drift from what shipped.

```bash
openspec list            # active changes
openspec list --specs    # capability inventory
openspec validate --all

# Lab hosts have no node/npm, so the CLI cannot run there:
scripts/openspec-validate.py --strict
```

In an OpenSpec-aware agent: `/opsx:propose`, `/opsx:apply`, `/opsx:archive`,
`/opsx:explore`, `/opsx:sync` and `/opsx:update`. Cursor spells the same six
`/opsx-propose` and so on. Six further workflows, including `verify`, ship with
the CLI but not with the core profile; `openspec validate --all --strict` is the
verification this project runs.
Agents working in this repo must follow [`AGENTS.md`](AGENTS.md), which covers the scope
rule (behaviour changes need their own change record) and what to verify before a PR.

## Testing

```bash
go test ./...
go test ./internal/devid/ ./internal/inventory/ -count=1
```

## E2E verification cheat sheet

Human (and agent) crib for “did DevID + SPIRE actually work?”. Paths below are the **Hogan lab**: host = Proxmox hypervisor, guest = VM **399** (`vtpm-pilot`). Nested QEMU-lab materials instead live under `/var/lib/mds-lab/vms/guest100/shared/devid-out/`.

As `cursor-agent` on Hogan: `ssh vtpm-pilot` (or `qm guest exec 399 -- …`). Guest has **python3**, often **no curl**.

**Green** means all of: MDS `instance-id=i-399`; guest DevID PEM issued by `CN=BGL Proxmox DevID CA`; `devid.{priv,pub}.blob` non-empty; `spire-agent` healthy; `spire-server agent list` shows a `tpm_devid` agent. `spire-agent api fetch x509` may still say `no identity issued` until you create registration entries — that is SPIRE policy, not enroll failure.

### Host — package, MDS, IMDS DNAT

```bash
# What is running?
dpkg -l vtpm-mds
vtpm-mds -version          # must match dpkg Version
systemctl is-active vtpm-mds
curl -fsS http://169.254.169.1/health
journalctl -u vtpm-mds -n 50 --no-pager | grep -E 'version |DevID |enroll/'

# Guest IMDS NIC on the bridge
qm status 399; qm config 399 | grep -E 'net1|tpmstate'
ip neigh show dev vmbr_imds | grep 169.254.169.10

# DNAT 169.254.169.254 → :80 (use iifname, table inet vtpm_mds)
nft list table inet vtpm_mds
```

### Guest — IMDS (from inside the VM)

```bash
ssh vtpm-pilot
python3 - <<'PY'
import json, urllib.request
IMDS = "http://169.254.169.254"
print(urllib.request.urlopen(IMDS + "/health", timeout=5).read().decode())
req = urllib.request.Request(IMDS + "/latest/api/token", method="PUT",
    headers={"X-aws-ec2-metadata-token-ttl-seconds": "60"})
token = urllib.request.urlopen(req, timeout=5).read().decode()
h = {"X-aws-ec2-metadata-token": token}
iid = urllib.request.urlopen(urllib.request.Request(
    IMDS + "/latest/meta-data/instance-id", headers=h), timeout=5).read().decode()
print("instance-id", iid)          # expect i-399
print(urllib.request.urlopen(urllib.request.Request(
    IMDS + "/latest/dynamic/instance-identity/document", headers=h), timeout=5).read().decode())
PY
```

### Guest — TPM 2.0

```bash
ls -l /dev/tpm0 /dev/tpmrm0
tpm2_getcap properties-fixed | grep -E 'FAMILY|MANUFACTURER|VENDOR_STRING'
tpm2_getcap handles-persistent          # DevID/EK typically 0x81010001 / 0x81010016
tpm2_pcrread sha256:0,1,2,3,4,5,6,7

# EK certificate in NV (swtpm may or may not implement this NV index)
tpm2_getekcertificate -o /tmp/ek.der && \
  openssl x509 -inform DER -in /tmp/ek.der -noout -subject -issuer -dates
```

### Guest — DevID materials (SPIRE `tpm_devid` files)

```bash
ls -l /var/lib/spire/agent/devid.crt.pem \
      /var/lib/spire/agent/devid.priv.blob \
      /var/lib/spire/agent/devid.pub.blob
wc -c /var/lib/spire/agent/devid.priv.blob /var/lib/spire/agent/devid.pub.blob
# Hogan today: priv=222 pub=280 bytes (TPM2B_PRIVATE / TPM2B_PUBLIC)

openssl x509 -in /var/lib/spire/agent/devid.crt.pem -noout -subject -issuer -dates
# subject=CN=vtpm-pilot   issuer=CN=BGL Proxmox DevID CA
openssl x509 -in /var/lib/spire/agent/devid.crt.pem -noout -text | less
devid-enroll -version
```

(Re)enroll against MDS — writes a **new** cert into `-out` (use `/tmp` so you do not clobber SPIRE’s files until you mean to):

```bash
# guest; default MDS URL is http://169.254.169.254
sudo devid-enroll -cn vtpm-pilot -tpm /dev/tpmrm0 -out /tmp/devid-e2e
openssl x509 -in /tmp/devid-e2e/devid.crt.pem -noout -subject -issuer -dates
```

Install a host-matching client after a `dpkg -i` on Hogan — the host package no longer carries `/usr/bin/devid-enroll`, so push the guest `.deb` of the same version instead:

```bash
# HOST as cursor-agent; from dist/ after `make deb`, or
# gh release download v0.2.0 --repo dembaca/vtpm-mds --pattern 'devid-enroll_*_amd64.deb'
scp dist/devid-enroll_*_amd64.deb vtpm-pilot:/tmp/
ssh vtpm-pilot 'sudo dpkg -i /tmp/devid-enroll_*_amd64.deb && devid-enroll -version'
```

### Host — verify the guest cert against the DevID CA

```bash
ssh vtpm-pilot cat /var/lib/spire/agent/devid.crt.pem > /tmp/devid.crt.pem
openssl x509 -in /etc/ssl/certs/proxmox_devid_ca.crt -noout -subject -issuer -dates
# issuer is the org root; -partial_chain if you only have the issuing CA file
openssl verify -partial_chain -CAfile /etc/ssl/certs/proxmox_devid_ca.crt /tmp/devid.crt.pem
# expect: /tmp/devid.crt.pem: OK
```

### Guest — SPIRE agent

```bash
systemctl is-active spire-agent
grep -A6 'NodeAttestor "tpm_devid"' /etc/spire/agent.conf
# devid_*_path should be /var/lib/spire/agent/devid.{crt.pem,priv.blob,pub.blob}

spire-agent healthcheck -socketPath /tmp/spire-agent/public/api.sock
# expect: Agent is healthy.

journalctl -u spire-agent -n 80 --no-pager | grep -Ei 'tpm_devid|attested|error'
```

### Host — SPIRE server (attested nodes)

```bash
systemctl is-active spire-server
grep -A6 'NodeAttestor "tpm_devid"' /etc/spire/server.conf
# devid_ca_path = /etc/ssl/certs/proxmox_devid_ca.crt
# endorsement_ca_path = /etc/ssl/certs/proxmox_tpm_ca.crt

spire-server agent list
# look for Attestation type: tpm_devid
# SPIFFE ID …/spire/agent/tpm_devid/<fingerprint>
# Can re-attest: true
```

MDS enroll logs on the host while a guest enroll runs:

```bash
journalctl -u vtpm-mds -f --no-pager | grep -E 'enroll/|VMID=399'
# POST /latest/devid/enroll/start 200
# POST /latest/devid/enroll/finish 200
```

## License

Apache 2.0 — See LICENSE file

## Maintainers

DG-i Platform Engineering — `andreas.dembach@dg-i.net`
