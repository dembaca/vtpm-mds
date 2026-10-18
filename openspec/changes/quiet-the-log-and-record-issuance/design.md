# Design

## Context

Assigned by the coordinator on 2026-10-18 from his list of what is missing for
production readiness (points 1 and 4), for `agent-mac-vtpm-mds-3502f6`.

### Evidence read in this repository (2026-10-18, `main@407bfca`)

- `main.go` has the `-debug` flag and uses it once: to dump the loaded
  configuration. It does not reach the server.
- `internal/server/server.go` writes `[DEBUG …]` lines unconditionally from
  `connContext`, `extractVMConfigByRouting`, `getMACFromARPFile`, `getMACFromARP`,
  `findVMConfigByMAC` and `RefreshVMConfigCache`. `connContext` and the lookups run
  once for each connection, and hogan counted about ten lines per request because
  a request there usually opens one. `RefreshVMConfigCache` ends with
  `[DEBUG RefreshVMConfigCache] Cache refresh complete (…): N VMs, M MAC addresses
  cached`, the only line that says how many VMs a start or a reload loaded.
- `loggingMiddleware` logs `%s %s %d %v` for method, `r.URL.Path`, status and
  duration. The catch-all handler logs `Unmatched route: %s %s from %s` with
  `r.RemoteAddr`, so an unmatched path already names its peer and the matched ones
  do not.
- `internal/devid/http.go`, `HandleFinish`, logs `devid enroll/finish: %v` for an
  error and nothing for a success. `Enroller.Finish` returns the PEM and has the
  VM ID and the EK fingerprint in hand but not the serial, which the CA computes.
- `inventory.GetVMConfigFromRequest(r)` gives the bound record to a handler or a
  middleware; `loggingMiddleware` sits inside `vmConfigMiddleware`, so the
  record is in the request's context when the line is written.

## Decided by this record's author, for the maintainer to overrule

- **D-a. `-debug` switches the `[DEBUG` lines, and only them.** Without it none is
  written; with it they are unchanged. The config dump stays as it is. There is no
  second level: the flag is on or off.
- **D-b. The flag reaches the server as a value, not a global.** The server is told
  at construction, so a test can start one with debug on and off. How is an
  implementation note.
- **D-c. The access line.** `METHOD PATH STATUS DURATION PEER VM`, appended to the
  present order, so that a reader or a script that took the first four fields still
  does. `PEER` is the IP address of the connection, without the port (the port is
  ephemeral and says nothing), taken from the connection and never from
  `X-Forwarded-For`; `VM` is the bound VM ID, `-` when unbound. An unparseable peer
  is written as `-`.
- **D-d. The path is quoted** (`%q`). The path is what the guest chose, URL-decoded:
  a `%0a` in it is a line break in the journal. The unmatched-route line gets the
  same quoting. It stays, because it is the only line a path that no handler claims
  would otherwise have; with the access line it says the same thing twice, and
  removing it is a decision for the maintainer.
- **D-e. The issuance line.** `devid issued serial=<hex> vm=<id> ek=<sha256 hex>`:
  the certificate's serial number in hexadecimal, the VM ID, and the EK certificate
  fingerprint the enrolment was bound to (the one `Finish` already compares). The
  serial is deterministic from the CA, subject and key; a re-enrolment with the same
  key and subject therefore logs the same serial again, which is information.
  Nothing else: not the PEM, not the DevID public key, not the nonce, not the session.
- **D-f. Refusals are not added.** A failed finish already logs its error; this
  change adds no line for an authentication failure. That belongs with the item
  that stops the error text from reaching the guest, and should decide what a
  refusal logs and what it tells the caller.
- **D-h. The cache summary stops being debug output.** The line that ends a load
  and says `N VMs, M MAC addresses` reports a start or a reload, which an operator
  checks after every `systemctl reload`. It is written whatever the flag says, with
  its `[DEBUG RefreshVMConfigCache]` prefix replaced by an ordinary one; the lines
  that announce the load and its source stay debug. Without this the quiet journal
  would not say whether a reload worked.
- **D-g. Where the line is written.** In `HandleFinish`, after `Finish` returns,
  using the certificate it returned: the handler has the VM ID and the fingerprint,
  and parsing the returned PEM gives the serial. `Finish` and the CA stay as they are.

## Non-Goals

- **The `connectionMap` leak**, its own change (`drop-the-connection-map`).
- **Throttling**, `throttle-guest-requests`.
- **Structured or levelled logging**, a log shipper, rotating files: the journal is
  the sink and stays so.
- **What enrolment says to the guest on an error.**

## Risks

- **A quieter journal hides what the debug lines showed on a fault.** The ARP and
  MAC lookup lines are the ones that explained why a caller was unbound. An operator
  who needs them starts the service with `-debug` and reloads; the access line now
  says `-` for the VM, which is the symptom.
- **A longer access line.** Two more fields; nothing the repository reads.
- **Peer and VM in the journal are personal data of no one**: they are a guest's
  address and a VM ID, as the old unmatched-route line already wrote.
