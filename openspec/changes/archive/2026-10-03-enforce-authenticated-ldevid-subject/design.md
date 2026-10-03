# Design

## Context

See proposal.md — Why.

Two places decide the subject. `subjectCNFromRequest` picks the common name
and is called from `Enroller.Start`, which stores the result on the session as
`SubjectCN`. `CA.IssueDevID` then builds the certificate subject from the
signing request's platform identity and that stored common name.

`HandleStart` already passes the authenticated VM ID twice —
`e.Start(requestData, sig, vmid, vmid, ekFP)` — once as the subject fallback
and once as the session's VM identity. The two arguments have always carried
the same value from the HTTP path; only `subjectCNFromRequest` could make them
diverge, and only in the guest's favour.

The session also binds the VM ID, and `Enroller.Finish` checks that the caller
finishing is the caller that started. So the authenticated identity is
available at issuance without any new plumbing.

## Goals / Non-Goals

**Goals:**

- The common name of an issued LDevID is a value the service established.
- The guest can still carry non-identity subject attributes through.
- The change is visible in the certificate, so an operator can tell a
  re-enrolled guest from one that has not been re-enrolled yet.

**Non-Goals:**

- Discarding the platform identity altogether. Organization and organizational
  unit are used by deployments to group guests, assert no identity the service
  can contradict, and removing them would be a second breaking change hidden
  inside this one.
- Rejecting a request whose platform identity names another VM. See the
  decision below.
- Changing the serial derivation, the validity window, or the SAN contents.
- Adding revocation, or shortening LDevID validity. Both are real gaps — a
  certificate issued under a borrowed name cannot be withdrawn — but each is
  its own change, and neither is needed to stop issuing new ones.
- Verifying the platform identity against anything else, such as the guest's
  hostname.

## Decisions

### Override the common name; do not reject the request

A signing request whose platform identity carries a different common name is
accepted, and the name is ignored. The alternative — answering `400` — was
considered and rejected:

- The guest client offers a `-cn` flag and the lab scripts use it, so rejecting
  would break callers that are doing nothing wrong by their own lights.
- A request that names another VM is not evidence of an attack; the obvious
  case is a guest that was told its own hostname.
- The service is the authority on the identity, so contradicting the guest is
  the correct response, and it is the quieter one.

The consequence is that a guest cannot tell from the response that its name was
dropped. It can tell from the certificate it receives, which it already reads.

### The common name is the VM ID, unconditionally

Not "the VM ID unless the platform identity agrees with it". A conditional
would make the certificate's content depend on a guest-supplied value again,
and the two branches produce the same certificate anyway.

### Keep the rest of the platform identity

`CA.IssueDevID` continues to fill the subject from the platform identity RDN
sequence and then sets the common name from the session's VM ID, replacing any
common name the sequence carried. Filtering at the RDN level rather than
rebuilding the subject keeps the encoding of the other attributes exactly as
the guest sent it.

### The session stops carrying a separate subject common name

`Enroller.Start`'s `subjectCN` parameter and the session's `SubjectCN` field
exist only to pass the fallback. With the VM ID always winning, the session's
VM ID is the subject common name, and the duplicate is removed rather than left
as a field that must stay in sync with another one.

### The SAN criticality rule becomes unreachable, and says so

`san.go` marks the TCG SAN critical when the subject is empty — correct
behaviour for a certificate with no subject, per RFC 5280. Authentication
requires a non-empty VM ID and the VM ID is now always the common name, so the
subject is never empty. The branch is kept as a guard rather than deleted; the
spec states the observable outcome, which is that the extension is never
critical.

## Risks / Trade-offs

- **[Risk] A SPIRE registration entry keyed on a guest-chosen name stops
  matching after that guest re-enrols** → Mitigation: the migration note in the
  spec delta says to re-key entries to the VM ID first. The failure mode is a
  workload that cannot attest, which is visible and reversible, not a silent
  authorisation change.
- **[Risk] Certificates issued under a borrowed name remain valid forever** →
  Accepted here and called out in the proposal: there is no revocation list, so
  the only way to withdraw them is to rotate the DevID CA. That is out of scope
  for a change whose job is to stop minting new ones, and it is a gap worth its
  own change.
- **[Risk] Re-enrolling produces a second valid certificate rather than
  replacing the first, because the serial is derived from the subject** →
  Accepted, and inherent to a deterministic serial over a changing subject.
  Worth knowing when counting issued certificates.
- **[Trade-off] The guest is not told its name was ignored** → Accepted; see
  the decision above. The certificate is the answer.

## Migration Plan

1. Re-key any authorisation that uses an LDevID subject common name to the VM
   ID, before the affected guests re-enrol.
2. Deploy the package.
3. Re-enrol the guests. Each receives a certificate with `CN=<vm id>`.
4. The certificate a guest held before re-enrolling stays valid. If that
   matters for a given guest, the DevID CA must be rotated; this change does
   not provide a way to withdraw it.

Rollback is reinstalling the previous package, which restores the old
behaviour for certificates issued after the rollback. Certificates issued in
between keep the VM ID as common name.

## Open Questions

None.
