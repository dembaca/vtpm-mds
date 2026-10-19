# Design

## Context

The subject is built in `IssueDevID`:

```go
var subj pkix.Name
subj.FillFromRDNSequence(&sr.PlatformIdentity)
subj.CommonName = subjectCN
```

`archive/2026-10-03-enforce-authenticated-ldevid-subject` made the common name
the authenticated VM ID and kept the rest of the platform identity, reasoning
that the other attributes assert nothing the service established. That is true
of the service's intent but not of what a relying party sees: every attribute in
a certificate is signed by the same CA, and nothing in the certificate marks
which ones were checked.

`devid-enroll` puts only a common name into the platform identity
(`internal/devid/csr.go`, `platformCN`), so the shipped client never sent
anything this change drops.

## Decisions

- **D-a. Subject is `pkix.Name{CommonName: subjectCN}`, nothing else.** The
  platform identity is not read by `IssueDevID` at all.
- **D-b. Drop silently, do not reject** (the maintainer's E2.5). A `400` for a
  platform identity with attributes other than `CN` would break any third-party
  client that fills one, for no gain: the attributes cannot reach the
  certificate either way. The undecodable-platform-identity `400` in request
  parsing stays, because that is a malformed request, not an unwanted attribute.
- **D-c. No log line for dropped attributes.** The issuance log (`devid issued
  serial=… vm=… ek=…`) stays exactly one line with exactly those fields;
  logging guest-chosen strings would also reopen the log-forging surface the
  access line was quoted against.
- **D-d. Serial construction unchanged.** It still hashes the subject; the
  subject is now a function of the VM ID only.

## Non-Goals

- Lifetime, `notAfter`, renewal (`limit-the-ldevid-lifetime-and-renew-it`).
- An intermediate CA per cluster.
- The SAN, key usage, EKU, basic constraints.
- Re-issuing certificates already issued; they change on their next enrolment.

## Risks

- A consumer outside this repository relies on `O`/`OU` in the LDevID. None is
  known: SPIRE uses `subject:cn`, Teleport's TPM join uses the EK. If one
  appears, the attribute it needs belongs in a service-controlled source (a VM
  tag is explicitly excluded from certificates by the maintainer's earlier
  decision), not in the guest's request.
