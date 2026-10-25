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
  certificate is trusted and bound and after the caller's own pin check, and
  before the session and request checks. The scan runs for every caller, as
  the spec requires: a VM whose own pin matches is refused only if another
  VM ID also holds that EK. Both legs, like the own-pin check, so a
  clone is refused before it costs a credential challenge.
- **D-a2. No race between two first enrollments.** The auth-time check is
  the cheap early refusal, but two VMs with one EK and no pins could both
  pass it and then each create a pin under its own name. `ensurePinBeforeIssue`
  therefore repeats the cross-VM scan under a package-level `sync.Mutex` that
  also covers the exclusive create; one process serves all legs, so that
  makes "never accepted for another VM" hold on this node.
- **D-b. What counts as "pinned to another VM".** Any pin file in the store
  whose name is a valid VM ID other than the caller's and whose content
  equals the caller's EK fingerprint (case-insensitive), and any other VM in
  the current inventory whose non-empty `ek_sha256` equals it. "Valid VM ID"
  is `validateVMID`, which also skips the `.<vmid>.tmp-*` files of an
  in-flight create; empty files are ignored. A pin file counts even where
  the same VM's inventory `ek_sha256` overrides it for that VM (D-e: the
  service never decides a pin is stale). The `Enroller` sees only the
  caller's record today, so it gets a seam
  `InventoryPins func() map[string]string`, wired from the inventory cache in
  `internal/server/server.go`. When several VMs match, `pinned=` names the
  first in store order, then inventory order.
- **D-c. Answer and log.** `401` `EK certificate is pinned to another VM`.
  The other VM's ID is not in the body, so a guest cannot probe which VM
  holds an EK; the host log does name it. The refusal carries `pinned=` in
  its error, so `writeEnrollAuthError` writes its one existing line as
  `devid enroll auth: EK certificate is pinned to another VM vm=<caller>
  pinned=<other>` instead of a second line.
- **D-d. An unreadable store fails closed.** If the pin directory cannot be
  listed, or a listed pin file cannot be read (as `readStoredPin` already
  treats it), the leg answers `500` `pin store unavailable`, as finish
  already does when it cannot create a pin. A missing directory is an empty
  store. The response body never carries the other VM ID: the refusal keeps
  it in a log-only detail that `writeEnrollAuthError` and `HandleFinish`
  strip from the body.
- **D-e. A legitimate move is the operator's.** Restoring a VM under a new
  ID keeps its vTPM; the old ID's pin must be deleted first. The service
  never decides that a pin is stale.

## Risks / Trade-offs

- Per node only (proposal Impact). A clone migrated to or first enrolled on
  another node is refused only after P1.
- Every leg lists the pin directory. Linear in the VMs enrolled on this node
  (tens to hundreds); measured on hogan in task 2.x before calling it fine.
- An oracle, bounded: any guest that holds a trusted EK **certificate**
  (public material) learns whether that EK is enrolled on this node, at six
  enrollments per 60 s. It learns no VM ID. Accepted.
- An unlistable store shows only in the journal, not in `/health`.
- A VM ID that was destroyed and whose pin was not removed blocks a later VM
  that legitimately inherits its vTPM (D-e): visible as the log line, fixed
  by deleting one file.
