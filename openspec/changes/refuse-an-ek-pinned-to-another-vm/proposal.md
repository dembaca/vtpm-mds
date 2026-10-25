# Proposal

## Why

An ordinary `qm clone` copies the source VM's vTPM state, so the clone has
the source's endorsement key. vtpm-mds pins an EK per VM ID on first
enrollment, but only checks that a VM keeps **its own** EK: a clone with the
source's EK and a new VM ID passes as a first enrollment, gets its own pin
and an LDevID, and two VMs then prove possession of the same TPM. bgl-infra
TASK 2.3c (2026-10-25) showed the clone path; the maintainer decided on ask
`gox8yerp…` ("A") that the service must refuse it, with the guest-side
`CN` check in `renew-when-the-devid-key-is-not-this-tpms` as the other
half.

## What Changes

- Both enroll legs refuse an EK that is already pinned to a **different** VM
  ID, from the pin store on this node or from another VM's inventory
  `ek_sha256`: `401` with body `EK certificate is pinned to another VM`,
  before any challenge or issuance, and no pin is created for the caller.
- Each refusal writes one log line naming the caller's VM and the VM that
  holds the pin, never the fingerprint.
- Nothing is removed or rewritten automatically: a VM whose vTPM really
  moved to a new VM ID (restore under a new ID) needs the operator to delete
  the old VM's pin file, as a replaced vTPM already does.

## Impact

- `internal/devid/auth.go`, `internal/devid/pin.go` and their tests;
  `debian/vtpm-mds.8` for the operator
  note; `debian/changelog`. Host package only.
- Spec: `devid-enrollment` adds "Refuse An EK Pinned To Another VM".
- **Per node only** until pins are cluster-wide (P1, bgl-infra backlog): a
  clone enrolling on a node that never saw the source is not refused. The
  guest's `CN` check still renews it there, so it holds a certificate
  for its own VM ID, but on a second TPM-sharing VM.
- A clone that kept its source's vTPM state gets no DevID on a node where the
  source is pinned, until it has a new vTPM. That is the intended outcome;
  the bgl-infra template creates a new vTPM per VM anyway.
- Each leg reads the pin directory once (one small file per enrolled VM on
  the node); enrollment is rare and rate-limited.
