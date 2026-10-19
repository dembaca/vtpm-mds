# Proposal

## Why

`IssueDevID` (`internal/devid/ca.go:145-147`) fills the LDevID subject from the
platform identity in the guest's signing request and then overwrites only the
common name with the authenticated VM ID. Every other attribute — `O`, `OU`,
`C`, `L`, `serialNumber`, anything an RDN sequence can carry — reaches a
certificate signed by the DevID CA exactly as a guest with root wrote it. The
living spec allows this on purpose ("they assert no identity the service has
established"), but a relying party cannot tell which attributes the CA checked:
a guest can obtain `O=Kunde A, OU=infra, CN=399` from our CA. SPIRE's
`tpm_devid` selects on `subject:cn` only, so SPIRE is not affected today; any
other verifier that looks at `O` or `OU` would trust the guest.

The maintainer decided on 2026-10-19 (E2.5): the subject consists of
`CN=<VMID>` only, guest-supplied subject attributes are dropped silently, as a
change of its own and part of the beta.

## What Changes

- The issued LDevID's subject is exactly `CN=<authenticated VM ID>`. The
  platform identity in the signing request is still parsed (the wire format and
  the `400` for an undecodable one are unchanged) but contributes nothing to the
  certificate.
- A request that carries a platform identity is **not** rejected: existing
  clients keep working, and the shipped `devid-enroll` sends only a common name,
  which was already ignored.
- The deterministic serial is unchanged in construction (CA certificate,
  subject, public key); because the subject no longer varies with the request, a
  re-enrolment of the same key now always yields the same serial.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: "Verify The Signing Request Before Issuing A Challenge"
  (`MODIFIED`: the platform identity contributes nothing). "Issue A
  Non-Expiring LDevID Bound To The Authenticated VM" is `REMOVED` and replaced
  by the `ADDED` "Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID"
  (subject is `CN=<VMID>` only; the scenario that kept `O`/`OU` becomes one
  that drops them). The replacement is needed because OpenSpec 1.14 refuses a
  `MODIFIED` block that drops a scenario.

## Impact

- **Code:** `internal/devid/ca.go` (`IssueDevID`), `internal/devid/devid_test.go`
  (`TestIssueDevIDPreservesOrganizationAndOU` becomes a test that they are
  dropped). Docs: `debian/devid-enroll.1` (`-cn` no longer claims to set the
  certificate's common name, stale since 2026-10-03) and the expected
  `openssl x509 -subject` output in the root `README.md`. No change to
  `cmd/devid-enroll` code, `imds/`, other `debian/` files or the wire format.
- **Operators:** certificates issued after the upgrade carry no attribute other
  than `CN`. Certificates already issued keep theirs until they are re-issued.
  SPIRE registrations on `tpm_devid:subject:cn:<vmid>` (D8) are unaffected.
- **Apply order:** `limit-the-ldevid-lifetime-and-renew-it` changes the same
  issuance requirement; after this change archives, its delta targets the new
  name. Whichever archives second rebuilds its `MODIFIED` block
  against the text the first one left (see AGENTS.md on the `public-ipv4`
  correction that nearly got reverted that way).
- **Not in scope:** certificate lifetime and renewal, an intermediate CA per
  cluster, any change to the SAN.
