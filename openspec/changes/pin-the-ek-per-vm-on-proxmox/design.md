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

- **D-a. Pin store path.** One file per VMID:
  `/var/lib/<package>/ek-pins/<vmid>` containing a single line: lowercase hex
  SHA-256 of the EK certificate DER (same algorithm as `EKFingerprint` /
  inventory `ek_sha256`). Directory mode `0750`, files `0640`, owned by the
  service user. The package already creates `/var/lib/<package>`; this change
  ensures `ek-pins/` exists at enroll time (`mkdir` in code is enough).
- **D-b. When the pin is written.** On **successful** `enroll/finish` only
  (LDevID issued), and only if neither inventory `ek_sha256` nor a store file
  already pins that VMID. Failed finish / start never writes. Concurrent first
  finishes: create with `O_EXCL` (or write-temp + `link`); loser keeps the
  winner's pin and the next enroll must match it.
- **D-c. Enforcement order.** For both enroll legs, effective pin =
  1. inventory `ek_sha256` if non-empty, else
  2. store file for that VMID if present, else
  3. no pin (first enroll allowed).
  Mismatch → `401` body `EK certificate does not match inventory pin` when
  the pin came from inventory, and `EK certificate does not match stored pin`
  when it came from the store (so operators can tell which source to fix).
- **D-d. Reset / fresh-tpm / re-enroll.** Replacing a vTPM changes the EK.
  The service **SHALL NOT** auto-clear the pin. Operator removes
  `/var/lib/…/ek-pins/<vmid>` (documented in `debian/README.Debian` only if
  the coordinator wants packaging docs in a follow-up; this change's tasks
  document the path in the delta and in a short comment near the store). No
  HTTP admin API in this change.
- **D-e. Inventory layer stays dumb.** `ek_sha256` on the VM record remains
  YAML-only from inventory's point of view. The store is read inside
  `devid` auth, not merged into `VMConfig` by Proxmox parse. That keeps #62's
  fingerprint/reload work out of this change's inventory files except a
  one-line spec clarification.
- **D-f. Lab verification.** Cloud lab: enroll guest100 once → pin file
  appears; second enroll with same TPM succeeds; after deleting guest TPM
  state / using a different EK (or swapping the pin file to another digest),
  enroll fails with the stored-pin body. Hogan: same against VM 399 after
  code ACK (optional in tasks if lab covers it).

## Non-Goals

- **nft / bridge port MAC binding** — `bind-callers-to-their-bridge-port`.
- **Proxmox `macfilter` / firewall on VM 399** — E1(a), hogan read-only plan.
- **Deriving the pin from swtpm state on the host** — alternative B above.
- **HTTP or CLI to reset pins** — operator deletes the file.
- **`debian/` postinst changes, `internal/server/server.go`, `AGENTS.md`,
  root `README.md`** — coordinator / other assignees.
- **CA lifetime, SPIRE eviction, intermediate hierarchy** — E2 explore.

## Risks

- **TOFU race:** an attacker who spoofs MAC and enrolls first owns the pin.
  Acceptable only with E1(a)/(b) following soon; call that out in tasks as
  "pin alone is not enough".
- **Orphan pins** after VMID reuse (Proxmox recycles IDs): a new VM with an
  old ID inherits the old pin and cannot enroll until the file is removed —
  related to E2 VM-ID reuse; document under reset, do not auto-delete on
  missing VM config (a temporary inventory gap must not wipe pins).
- **Disk full / permission errors on write:** finish has already issued the
  cert; log and leave unpinned (retry write on next success) rather than
  failing the HTTP response after issuance — or fail finish before sign if
  the pin dir is unusable. **Prefer:** attempt pin write **before** signing
  the LDevID when no pin exists yet, so a broken state dir fails closed.
