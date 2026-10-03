# Proposal

## Why

The identity in an issued LDevID is chosen by the guest, not by the service.

`subjectCNFromRequest` in `internal/devid/enroll.go` reads the common name out
of the signing request's platform identity and returns it whenever it is
non-empty, falling back to the authenticated VM ID only when the guest supplied
none. The VM ID is the one value the service actually established — through the
caller's MAC address and the inventory lookup — and it is the one that loses.

So a guest authenticated as VM `100` can obtain a certificate reading
`CN=prod-db-01`, or `CN=101`, simply by putting that name in its own request.
Anything downstream that authorises on the LDevID subject — which is what a
subject is for, and what SPIRE's `tpm_devid` attestor surfaces — is authorising
a string the guest wrote about itself.

The damage outlives the request. LDevIDs are issued with `notAfter`
`9999-12-31T23:59:59Z` and the service publishes no revocation list, so a
certificate obtained under a borrowed name is valid forever.

## What Changes

- **BREAKING** The issued certificate's common name is always the authenticated
  VM ID. A common name in the signing request's platform identity is ignored.
- Other relative distinguished names from the platform identity — organization,
  organizational unit and the rest — are kept. They carry no identity claim the
  service can contradict, and dropping them would remove information some
  deployments put there deliberately.
- Because the common name is now always set and the authenticated VM ID is
  never empty, the issued subject is never empty, so the TCG Subject
  Alternative Name extension is never marked critical. That criticality rule
  was conditional on an empty subject.
- No change to the enroll wire format, to the client, to the certificate's
  validity, key usage, extended key usage or SAN contents, or to how the serial
  number is derived.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: `Issue A Non-Expiring LDevID With TCG DevID Encoding` is
  replaced by `Issue A Non-Expiring LDevID Bound To The Authenticated VM`. Its
  subject rule and its scenario `Guest-supplied common name is used` specify
  the defect and cannot be carried forward.
  `Verify The Signing Request Before Issuing A Challenge` has a closing
  sentence describing the platform identity as the source of the subject, which
  is now only partly true.

## Impact

- `internal/devid/enroll.go`: `subjectCNFromRequest`, and what `Enroller.Start`
  stores as the session's subject.
- `internal/devid/ca.go`: `IssueDevID` builds the subject from the platform
  identity and the common name.
- `internal/devid/http.go`: `HandleStart` passes the VM ID twice today, once as
  `subjectCN` and once as `vmid`; that pair collapses.
- `internal/devid/devid_test.go`: gains the override case, which is the
  regression test for this defect.
- Anything that authorises on an LDevID subject, including SPIRE `tpm_devid`
  registration entries. A guest that previously received a chosen common name
  will receive its VM ID after re-enrolling, and entries keyed on the old name
  stop matching.
- Certificates already issued under a guest-chosen name remain valid and cannot
  be withdrawn, because there is no revocation list. Re-enrolling produces an
  additional certificate rather than replacing one; the serial is derived from
  the subject, so the two differ. See design.md — Migration Plan.
