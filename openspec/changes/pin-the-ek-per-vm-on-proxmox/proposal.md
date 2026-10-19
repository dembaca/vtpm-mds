# Proposal

## Why

On `vmbr_imds`, a guest with root can set another VM's MAC and be bound as that
VM. Enrollment then issues an LDevID whose CN is the spoofed VMID: the EK CA
chain accepts the attacker's own genuine vTPM. Today's optional inventory
`ek_sha256` pin would stop that, but the Proxmox backend never populates it
and operators do not set it by hand. Maintainer decision E1 (2026-10-19): pin
the EK per VM first, then bind callers to their bridge port.

## What Changes

- After a **first successful** DevID enrollment for a VMID, the service
  **persists** the EK certificate fingerprint under `/var/lib/…` (TOFU).
- Later enroll legs for that VMID **require** the presented EK fingerprint to
  match the stored pin when the inventory `ek_sha256` is empty (inventory pin
  still wins when set).
- A mismatch is `401` with a clear body; resetting a pin is an **operator**
  action (remove the pin file), not automatic on `fresh-tpm` / failed enroll.
- **Record only in this PR.** Code after the coordinator's ACK of the record.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: add the TOFU pin store, when it is written, how it is
  enforced relative to inventory `ek_sha256`, and how an operator resets it.

The living `vm-inventory` rule that the Proxmox backend does not populate
`ek_sha256` stays true; the pin store is outside inventory and needs no
inventory delta.

## Impact

- **Code, after ACK:** `internal/devid/` (auth + write-on-success), small
  helpers possibly under `internal/proxmox/` only if reading host state is
  chosen later (this record prefers TOFU and does **not** require it). New
  files under `/var/lib/…/ek-pins/`. Unit + lab enroll tests. **Not**
  `debian/` packaging logic beyond ensuring the state dir exists if already
  created by the package; **not** `internal/server/server.go` (hogan #62).
- **Operators:** first enroll of a VM locks its EK; replacing a vTPM needs a
  documented pin delete. No change to guest client wire format.
- **Not in scope:** nft bridge MAC binding (`bind-callers-to-their-bridge-port`),
  Proxmox `macfilter` on VM 399 (E1a, hogan), CA hierarchy / SPIRE (E2).
