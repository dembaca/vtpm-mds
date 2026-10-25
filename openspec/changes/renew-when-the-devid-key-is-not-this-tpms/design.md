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

**Non-Goals:** checking the certificate against the DevID CA (the client
still does not verify what the service issued); the server and its EK pin,
which `refuse-an-ek-pinned-to-another-vm` changes; template cleanup in
bgl-infra; SPIRE configuration; the renewal threshold of one third.

## Decisions

- **D-a. Fit is part of "due".** The client renews when the lifetime rule
  says so **or** the materials do not fit. "Do not fit" means: the certificate
  is missing or cannot be parsed (as today); a blob is missing or empty; the
  certificate's public key does not equal the key in `devid.pub.blob`
  (compared as keys, `cert.PublicKey` against `tpm2.Public.Key()`, not as
  raw DER, so an exponent of 0 reads as 65537); or the blobs are **foreign**
  to this TPM (D-c). The order is cheapest first, so the TPM is touched only
  when the files agree.
- **D-b. Which key the renewal uses, and when it is loaded.** Foreign blobs:
  a new key. Blobs that load with a mismatched certificate: the loaded key,
  as a due renewal reuses it today. The check derives the H-1 parent, loads
  the DevID key and flushes the parent (two objects at most). A not-due run
  flushes the DevID handle itself, because with `-tpm /dev/tpm0` closing the
  device flushes nothing. A renewal hands the loaded `KeyInfo` to a variant
  of `CreateSigningRequestWithDevID` that records it in its resources
  **before** the EK and AK steps, so the error path's `Flush` owns it. The
  sequence then is EK read and flush, `CreateAK` (DevID, parent, AK: three,
  parent flushed), certify, activate (EK, AK, DevID: three): the same peak as
  today, so "Four keys are never loaded at once" still holds. `CreateAK`
  derives the parent again, so a renewal derives it twice, as today; the
  check only avoids a third derivation and a second load. The `ErrDevIDLoad`
  retry in `ClientEnrollOrRenew` becomes unreachable and is removed.
- **D-c. Foreign is defined by the TPM's answer.** `LoadDevID` today wraps
  every `TPM2_Load` error in `ErrDevIDLoad` with `%v`, so a busy or
  out-of-memory TPM would read as "not this TPM" and a not-due run would
  replace a working key. A new sentinel `ErrDevIDForeign`, wrapping the
  cause with `%w`, covers exactly: a blob that does not decode, and a
  format-1 parameter error from `TPM2_Load` with code `TPM_RC_INTEGRITY`
  (the private area's HMAC fails under another parent), `TPM_RC_BINDING` or
  `TPM_RC_SENSITIVE`. Everything else, warnings such as `TPM_RC_RETRY` or
  `TPM_RC_OBJECT_MEMORY`, other TPM errors, transport errors and a failure to
  derive the parent, fails the run with that error and changes nothing; the
  next timer run retries. The order of all checks is: certificate present
  and parseable, lifetime, `CN` (D-e), blobs present, key equality, load.
- **D-e. The subject must name this VM.** Before touching the TPM, the
  client reads `/latest/meta-data/instance-id` from the service it enrolls
  with and renews when the certificate's subject `CN` is not the part after
  `i-`. The service issues `CN=<vmid>` and nothing else, so this compares
  like with like. When the read fails or the answer does not start with
  `i-`, the run fails and changes nothing, as in D-c; enrolling needs the
  service anyway. The metadata request goes through the session token like
  any other read, if the service requires one.
- **D-d. Log the reason.** One line per renewal names why: `no certificate`,
  `unreadable certificate` (the existing line), `due`, `missing blob`,
  `certificate is for another VM`, `certificate does not match key`, or
  `key does not load in this TPM`. A
  not-due run with fitting materials stays silent apart from the existing
  summary line.

## Risks / Trade-offs

- Daily TPM work on every guest (D-a). Deriving an RSA-2048 primary takes
  seconds on a vTPM; acceptable for a daily timer and one boot run.
- A guest whose TPM is unavailable at that moment now fails a run it used to
  pass silently when nothing was due (D-c). The unit shows `failed`, the
  agent still starts (no `Requires=`), and the timer retries.
- A clone that kept its source's **vTPM state** has the same EK and loadable
  blobs; D-e renews its certificate, and with
  `refuse-an-ek-pinned-to-another-vm` the service then refuses it on a node
  where the source is pinned. Such a clone gets no DevID until it has a new
  vTPM, which is the intended outcome.
- Every run now needs the metadata service, also when nothing is due (D-e).
  A guest that boots while the IMDS is down fails its oneshot; the agent
  still starts with its existing materials, and the timer retries.
- Found on the way, outside this scope: `isTPMRetry` in
  `internal/devid/csr.go` matches `*tpm2.Warning`, while go-tpm may return
  warnings as values. Check whether it ever matches before reusing the
  pattern for D-c; if it does not, that is a separate change.

## Decided

- The `CN` check (D-e) and the server-side refusal as a separate change:
  the maintainer's answer "A" to ask `gox8yerp…`, 2026-10-25.
