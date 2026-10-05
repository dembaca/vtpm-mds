# Proposal

## Why

Reusing one guest vTPM across `scripts/qemu-lab/e2e-devid-guest.sh` runs ends in
`Certify: ... DA lockout mode`. After a single successful run the dictionary-attack
counter was reported as 1 of 3. That looked like enrollment burning an authorization
failure per run. It does not.

Investigation on the cloud lab host (standard `swtpm-localca`, `/usr/share/OVMF/`)
showed: while the guest is still running after a successful enroll,
`lockoutCounter` is **0**. After a graceful ACPI poweroff it stays **0**. After
`stop-guest.sh` — which SIGTERM/SIGKILLs QEMU while the guest swtpm is still
attached — the persisted state reads **1**. Enrollment never increments the
counter; hard teardown does. Three hard stops explain the lockout on reuse.
`internal/devid` needs no change.

## What Changes

- `stop-guest.sh` shuts the guest down **gracefully** (QMP `system_powerdown`
  and/or SSH `poweroff`) and waits for QEMU to exit before killing it.
- The guest swtpm is stopped **after** QEMU has released the ctrl socket, with
  SIGTERM (not an unordered kill paired with a live QEMU).
- `e2e-devid-guest.sh` / README stop describing enrollment as the DA cause; they
  document that measuring `lockoutCounter` after a hard kill is misleading.
- **No** `DictionaryAttackLockReset` in the lab scripts — that would hide a
  teardown bug.
- **No** product code under `internal/devid`, `imds/`, or `identity/`.

## Capabilities

### New Capabilities

None. Lab tooling only — `skip_specs: true`.

### Modified Capabilities

None.

## Impact

- `scripts/qemu-lab/stop-guest.sh` (and possibly a small helper shared with
  `start-guest.sh` for QMP).
- `scripts/qemu-lab/setup-guest-tpm.sh` or a dedicated stop-tpm helper if swtpm
  lifecycle moves out of ad-hoc kills.
- `scripts/qemu-lab/README.md` DA-lockout note.
- Verification is cloud-lab-only (this host). Hogan must not run the QEMU lab.
