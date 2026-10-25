# Design

## Context

See proposal.md — Why. Found on 2026-10-25 in bgl-infra TASK 2.3c (clone of
VM 399 with an empty vTPM); the bgl-infra coordinator asked for a proposal,
not an implementation merged with it. Today `ClientEnrollOrRenew` returns
before it touches the TPM when the certificate is not due, and reuses the
blobs only once a renewal is already due (falling back to a new key on
`ErrDevIDLoad`).

## Goals / Non-Goals

**Goals:** a guest whose DevID materials do not belong to its TPM, or do not
belong together, renews on the next run of the oneshot or the timer, before
the SPIRE agent starts on boot; a guest whose materials fit keeps them.

**Non-Goals:** checking the certificate against the DevID CA or the subject
against this VM's ID (the client still does not verify what the service
issued; see Open Questions); the server and its EK pin; template cleanup in
bgl-infra; SPIRE configuration; the renewal threshold of one third.

## Decisions

- **D-a. Fit is part of "due".** The client renews when the lifetime rule
  says so **or** the materials do not fit. "Do not fit" means: a blob is
  missing or empty; the certificate's `SubjectPublicKeyInfo` differs from
  the public key decoded from `devid.pub.blob`; or `LoadDevID` fails with
  `ErrDevIDLoad` under the H-1 parent derived from this TPM. The order is
  cheapest first, so the TPM is touched only when the files agree.
- **D-b. Which key the renewal uses.** Unloadable blobs: a new key, exactly
  the existing fallback. Loadable blobs with a mismatched certificate: the
  loaded key, as a due renewal reuses it today. The check loads the key once
  and the signing request uses that handle, so a run never loads it twice.
- **D-c. Only a failed load counts.** A TPM error other than `ErrDevIDLoad`
  (device busy, deriving the parent fails) fails the run with that error and
  changes nothing. Renewing on such an error would replace a working key
  because of a transient fault, and the next timer run retries anyway.
- **D-d. Log the reason.** One line per renewal names why: `due`,
  `missing blob`, `certificate does not match key`, or `key does not load
  in this TPM`. A not-due run with fitting materials stays silent apart from
  the existing summary line.

## Risks / Trade-offs

- Daily TPM work on every guest (D-a). Deriving an RSA-2048 primary takes
  seconds on a vTPM; acceptable for a daily timer and one boot run.
- A guest whose TPM is unavailable at that moment now fails a run it used to
  pass silently when nothing was due (D-c). The unit shows `failed`, the
  agent still starts (no `Requires=`), and the timer retries.
- A clone that kept its source's **vTPM state** (full clone without a new
  `tpmstate`) has the same EK and loadable blobs: this change does not
  detect it, and the certificate keeps the source's CN until it is due.
  Covered only by the Open Question below and by template cleanup.

## Open Questions

- Should the client also renew when the certificate's `CN` is not this
  guest's VM ID, read from the service's `instance-id`? It would catch the
  case above, at the cost of a metadata request on every run. Left to the
  maintainer; not part of this change.
