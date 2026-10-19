# Design

## Context

Assigned 2026-10-19 in ASSIGNMENTS after dembaca's E1–E5 decisions:
`@agent-cursor-vtpm-22e0` records `pin-the-ek-per-vm-on-proxmox` (E1, first of
two spoofing changes). Branch from `main` at the tip that contains #61
(`report-readiness-honestly` archived). Code only after the coordinator's ACK
of this record. File split with hogan-d3e6 on #62: this change owns
`internal/devid/` and may touch `internal/proxmox/` only for pin-related
helpers; it must not edit `internal/server/server.go` or `debian/` product
logic.

### Evidence in this repository (2026-10-19)

- `AuthenticateEnrollCaller` (`internal/devid/auth.go`) enforces
  `vm.EKSHA256` when non-empty; otherwise any trusted EK for the bound VMID
  enrolls.
- `vm-inventory` Proxmox backend **SHALL NOT** populate `ek_sha256`
  (`openspec/specs/vm-inventory/spec.md`); YAML can set it, Proxmox guests
  never get a pin today.
- Enrollment already binds the EK certificate to the endorsement key and to
  the session fingerprint (`bind-ek-certificate-to-endorsement-key`); the gap
  is **cross-enroll under a spoofed MAC**, not certificate forgery.
- Issuance is already logged with VMID + EK fingerprint
  (`quiet-the-log-and-record-issuance`); the pin store makes that binding
  sticky.

### Alternatives weighed (required by the assignment)

| Approach | Pros | Cons |
|---|---|---|
| **A. TOFU:** write pin under `/var/lib/…/ek-pins/<vmid>` on first successful `enroll/finish` | Works for Proxmox and YAML; no root access to QEMU/swtpm state; same check path as inventory pin; clear reset (`rm` the file) | First enroll wins — a spoof that enrolls **before** the real guest locks the attacker in (mitigate: enroll soon after VM create; optional future: require inventory pin for high-value VMs) |
| **B. Derive from host vTPM state** (`tpmstate` volume + swtpm files under `/etc/pve` / storage) | Pin exists before first enroll; spoof cannot race | Path and format are site-specific; files are typically root-owned and not readable as the service user; EK cert may live in NV after guest boot, not as a static host file; coupling to Proxmox storage layout |

**Decision for this record: A (TOFU).** B stays a non-goal / future option if
operators need pre-enrollment pins without YAML; it would be its own change
once a readable, stable host-side EK source exists for this deployment.

## Decided by this record's author, for the maintainer to overrule

- **D-a. Pin store path.** One file per VMID under
  `$STATE_DIRECTORY/ek-pins/<vmid>` (systemd), falling back to
  `/var/lib/<package>/ek-pins/<vmid>` — not derived from `signing_key_path`
  (configurable; under `/etc` would break under `ProtectSystem=strict`).
  Content: lowercase hex SHA-256 of the EK certificate DER (same as
  `EKFingerprint` / inventory `ek_sha256`). Directory mode `0750`, files
  `0640`, owned by the service user. Create `ek-pins/` at enroll time
  (`mkdir` in code); operators seeding pins must `install -d` with the
  service owner first (see tasks 4.1).
- **D-b. Write the pin atomically before signing.** On `enroll/finish`,
  when neither inventory `ek_sha256` nor a store file pins the VMID yet, the
  service SHALL create `ek-pins/<vmid>` **before** `IssueDevID` by writing a
  temp file (`.${vmid}.tmp-<rand>`), `Sync()`, then `os.Link` onto the final
  name (atomic + exclusive; `EEXIST` = already pinned). An unusable state
  directory (or any create error other than "already exists") → HTTP `500`
  with body `pin store unavailable` (details only in the log), **no** LDevID
  PEM. Concurrent first finishes with the same EK: both may sign once the
  pin is linked. Start never writes. Failed finish after a successful pin
  create leaves the pin in place.
- **D-c. Enforcement order.** For both enroll legs, effective pin =
  1. inventory `ek_sha256` if non-empty, else
  2. store file for that VMID if present, else
  3. no pin (first enroll allowed).
  Mismatch → `401` body `EK certificate does not match inventory pin` when
  the pin came from inventory, and `EK certificate does not match stored pin`
  when it came from the store (so operators can tell which source to fix).
- **D-d. Reset / fresh-tpm / re-enroll.** Replacing a vTPM changes the EK.
  The service **SHALL NOT** auto-clear the pin. Operator removes
  `/var/lib/…/ek-pins/<vmid>`. No HTTP admin API in this change.
- **D-e. Inventory layer stays dumb.** `ek_sha256` on the VM record remains
  YAML-only from inventory's point of view. The store is read inside
  `devid` auth, not merged into `VMConfig` by Proxmox parse.
- **D-f. Lab verification.** Cloud lab: enroll guest100 once → pin file
  appears; second enroll with same TPM succeeds; mismatch / reset scenarios
  as in tasks. Hogan: seed (D-g) + enroll for VM 399 once dembaca gives the go.
- **D-g. Seed pins for VMs already enrolled (upgrade gap).** After this
  change ships, every existing VM has no store file; the first post-upgrade
  enroll wins (including a spoofer). The journal already logs
  `devid issued serial=… vm=<id> ek=<sha256>`
  (`quiet-the-log-and-record-issuance`). **Operators seed** before relying on
  the pin: extract those pairs into `ek-pins/<vmid>` (no new binary — a
  documented `journalctl`/`awk` one-liner in `tasks.md`). Hogan carries out
  the seed for VM 399 as a verification task once dembaca approves host work.
- **D-h. Both inventory backends.** The store applies whenever inventory
  `ek_sha256` is empty — **YAML and Proxmox**. The change title says
  "proxmox" because that is the production gap; behaviour is backend-agnostic.

## Non-Goals

- **nft / bridge port MAC binding** — `bind-callers-to-their-bridge-port`.
- **Proxmox `macfilter` / firewall on VM 399** — E1(a), hogan read-only plan.
- **Deriving the pin from swtpm state on the host** — alternative B above.
- **HTTP or CLI to reset pins** — operator deletes the file.
- **`debian/` postinst changes, `AGENTS.md`, root `README.md`** —
  coordinator / other assignees. (`ekPinDir` in `internal/server/server.go`
  is the only server.go touch: `$STATE_DIRECTORY/ek-pins` or the package
  state dir.)
- **CA lifetime, SPIRE eviction, intermediate hierarchy** — E2 explore.

## Risks

- **TOFU race:** an attacker who spoofs MAC and enrolls first owns the pin.
  Acceptable only with E1(a)/(b) following soon; call that out in tasks as
  "pin alone is not enough".
- **Orphan pins** after VMID reuse (Proxmox recycles IDs): a new VM with an
  old ID inherits the old pin and cannot enroll until the file is removed —
  related to E2 VM-ID reuse; document under reset, do not auto-delete on
  missing VM config (a temporary inventory gap must not wipe pins).
- **Disk full / permission errors on write:** fail finish with `500` before
  signing (D-b); never issue an unpinned LDevID on first enroll.
- **Upgrade without seeding (D-g):** until operators seed from the journal,
  the TOFU race window is open for every already-enrolled VM; E1(a)/(b) still
  matter.
