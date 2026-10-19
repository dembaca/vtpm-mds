# Proposal

## Why

LDevIDs are issued with `notAfter` `9999-12-31T23:59:59Z` (`NoExpiration` in
`internal/devid/ca.go`). There is no revocation list, so a certificate that
should stop being trusted never expires on its own. Maintainer decision E2.3
(2026-10-19): make the lifetime configurable (default 90 days) and renew from
the guest before SPIRE starts and on a daily check once less than one third of
the lifetime remains (or the cert is already expired).

## What Changes

- **BREAKING (issuance):** `IssueDevID` sets `notAfter` to `notBefore +
  devid_lifetime` instead of the far-future constant. Default lifetime is 90
  days (`2160h`). Omitting the config key keeps that default. Values above one
  year (`8760h`) are refused at config load.
- **BREAKING (serial):** the certificate serial SHALL change on every issuance,
  including renewals that reuse the same DevID key. Today's hash of CA +
  subject + public key alone would repeat the serial under RFC 5280
  §4.1.2.2 when only validity dates change; the hash also covers `notBefore`
  and `notAfter` (design D-g).
- **Config:** new `mds.devid_lifetime` (Go `time.ParseDuration` string).
- **Guest client:** the `devid-enroll` package gains a oneshot + daily timer
  that runs before `spire-agent`, renews when remaining lifetime is under
  one third (or the cert is missing/expired), **keeps the existing TPM DevID
  key**, and after a cert change clears the SPIRE agent SVID cache then
  `try-restart`s `spire-agent` (no hard dependency on SPIRE).
- **Operators:** E2.4 cleanup text lands in the `devid-enroll` man page (this
  change). A short pointer in the root README is left to the coordinator at
  archive time. No automatic reconciler.

## Apply order

**`issue-the-ldevid-subject-from-the-vm-id-only` (#70) archives first.** That
change REMOVEs `Issue A Non-Expiring LDevID Bound To The Authenticated VM` and
ADDs `Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID` (CN-only
subject; no surviving O/OU).

This change's delta is written against **that post-#70 requirement name**: it
REMOVEs `Issue A Non-Expiring LDevID Whose Subject Is Only The VM ID` and ADDs
`Issue A Time-Limited LDevID Whose Subject Is Only The VM ID`. Rebase this
branch onto `main` after #70 merges (and is archived) before code or before
merging #71. Do not archive both REMOVED deltas against the pre-#70 title.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: replace the non-expiring CN-only issuance requirement
  with a time-limited one; serial-on-renew; guest renewal + SPIRE cache clear;
  `mds.devid_lifetime`.
- `debian-packaging`: the `devid-enroll` package may ship systemd units/timer
  for renewal (today it ships only the binary and man page).

## Impact

- **Code, after coordinator code-go only:** `internal/devid/ca.go` (+ tests),
  `internal/config`, packaged `debian/config.yaml` comment/default,
  `cmd/devid-enroll` renewal logic, new unit/timer under `debian/` for the
  guest package, man page (incl. E2.4). Host daemon packaging unchanged beyond
  the config key.
- **SPIRE:** each new LDevID certificate gets a new `tpm_devid` agent SPIFFE
  ID. A restart that keeps the agent SVID cache does **not** re-read the
  LDevID (lab, 2026-10-19) — renew must clear that cache. Prefer registration
  on `tpm_devid:subject:cn:<vmid>`.
- **Not in scope:** intermediate CA; automatic SPIRE reconciler; enroll wire
  format; host package systemd; root README body (coordinator at archive).
