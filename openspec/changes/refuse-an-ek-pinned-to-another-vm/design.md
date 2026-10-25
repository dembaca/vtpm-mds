# Design

## Context

See proposal.md. Pins live in `$STATE_DIRECTORY/ek-pins/<vmid>`, one
lowercase hex SHA-256 of the EK certificate DER per file, created on the
first successful `enroll/finish` (`pin-the-ek-per-vm-on-proxmox`, archived
2026-10-19). Both legs already enforce the caller's **own** effective pin in
`authenticateEnrollLeg` (`internal/devid/auth.go`).

## Goals / Non-Goals

**Goals:** an EK enrolled for one VM ID is never accepted for another on the
same node; the refusal is visible in the host journal.

**Non-Goals:** cluster-wide pins (P1, a separate change with the bgl-infra
hook); deleting stale pins; detecting a clone before its first enrollment;
any guest-side change (`renew-when-the-devid-key-is-not-this-tpms`).

## Decisions

- **D-a. Where the check sits.** In `authenticateEnrollLeg`, after the EK
  certificate is trusted and bound and after the caller's own pin check, so
  a caller with its own matching pin is never slowed down by it, and before
  the session and request checks. Both legs, like the own-pin check, so a
  clone is refused before it costs a credential challenge.
- **D-b. What counts as "pinned to another VM".** Any pin file in the store
  whose name is a valid VM ID other than the caller's and whose content
  equals the caller's EK fingerprint (case-insensitive), and any other VM in
  the current inventory whose non-empty `ek_sha256` equals it. Files whose
  name is not a valid VM ID, and empty files, are ignored, as the own-pin
  path already treats them.
- **D-c. Answer and log.** `401` `EK certificate is pinned to another VM`.
  The other VM's ID is not in the body, so a guest cannot probe which VM
  holds an EK; the host log line does name it:
  `devid refused ek pinned to another vm vm=<caller> pinned=<other>`.
- **D-d. An unreadable store fails closed.** If the pin directory cannot be
  listed, the leg answers `500` `pin store unavailable`, as finish already
  does when it cannot create a pin. A missing directory is an empty store.
- **D-e. A legitimate move is the operator's.** Restoring a VM under a new
  ID keeps its vTPM; the old ID's pin must be deleted first. The service
  never decides that a pin is stale.

## Risks / Trade-offs

- Per node only (proposal Impact). A clone migrated to or first enrolled on
  another node is refused only after P1.
- Every leg lists the pin directory. Linear in the VMs enrolled on this node
  (tens to hundreds); measured on hogan in task 2.x before calling it fine.
- A VM ID that was destroyed and whose pin was not removed blocks a later VM
  that legitimately inherits its vTPM (D-e): visible as the log line, fixed
  by deleting one file.
