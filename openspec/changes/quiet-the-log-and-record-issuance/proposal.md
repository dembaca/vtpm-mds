# Proposal

## Why

**The journal is almost all debug output, and it does not say who asked.** On
hogan, running `0.2.0+git20261018.eaed1d1b54b6`, 594 of 699 journal lines in 24
hours were `[DEBUG …]` lines (coordinator, 2026-10-18), about ten per request
(`internal/server/server.go`, in `connContext`, `extractVMConfigByRouting`,
`getMACFromARPFile`, `findVMConfigByMAC` and `RefreshVMConfigCache`). The `-debug`
flag exists and only dumps the loaded configuration; the per-request lines are
written whatever the flag says. A guest that sends requests in a loop therefore
writes ten journal lines for each of them and can flood the journal.

**The one line that should be the audit trail leaves out the two facts that matter.**
`loggingMiddleware` writes `method path status duration`, and neither the peer
address nor the VM the request was bound to. Whoever reads the journal to find out
which guest asked for what has to join it with the debug lines that are about to go.

**An issued LDevID leaves no trace.** `Enroller.Finish` returns the certificate
and logs nothing on success; only a failure logs. The service issues a certificate
that never expires, and the journal cannot say that it did, for which VM or against
which endorsement key.

## What Changes

- **Debug lines are written only with `-debug`.** Every line that starts with
  `[DEBUG` is dropped from a normal run, and written, unchanged, with the flag.
  The flag is read once at start; nothing else changes about how it works. The
  one line that says how many VMs a load found is kept, as an ordinary line.
- **The access line carries the peer and the VM.** One line per request: method,
  path, status, duration, the connection's peer IP address and the ID of the bound
  VM, or `-` for none. The path is quoted so that a path with a line break in it
  cannot write a second journal line.
- **One line per issued LDevID.** On a successful finish: the certificate's serial
  number, the VM ID, and the fingerprint of the EK certificate it was bound to.
  Nothing secret and not the certificate itself.

## Capabilities

### Modified Capabilities

- `operability`: two requirements added, debug output only on request and an
  access line that names the peer and the VM.
- `devid-enrollment`: one requirement added, a log line for each issued LDevID.

## Impact

- **Code, after the coordinator's ACK:** `main.go` (hand the flag to the server),
  `internal/server/server.go` (a debug logger for the `[DEBUG` lines, the access
  line), `internal/devid/http.go` or `enroll.go` (the issuance line), and tests.
- **Behaviour visible to operators:** the journal is quieter by about nine tenths and
  its access lines are longer. Anything that parses the old line `METHOD PATH
  STATUS DURATION` will see two more fields.
- **Not in scope:** removing the `connectionMap` (its own change), throttling and
  rate limits (`throttle-guest-requests`), structured logging, log levels beyond
  on and off, and the error text that enrollment returns to the guest.
