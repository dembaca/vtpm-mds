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
  days (`2160h`). Omitting the config key keeps that default.
- **Config:** new `mds.devid_lifetime` (Go `time.ParseDuration` string). An
  unparseable or non-positive value fails `config.Load` the same way a bad
  `token_ttl` does.
- **Guest client:** the `devid-enroll` package gains a oneshot + daily timer
  that runs before `spire-agent`, renews when remaining lifetime is under
  one third (or the cert is missing/expired), and **keeps the existing TPM
  DevID key** across renewals (new key only when no usable blobs exist).
- **Operators (docs in this record, not a reconciler):** SPIRE agent cleanup
  for the beta stays the E2.4 manual path — `spire-server agent evict
  -spiffeID …` when a VM or pin is deleted; `agent purge` exists in SPIRE
  1.14.0 for expired-SVID garbage collection. No automatic reconciler.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `devid-enrollment`: replace the non-expiring LDevID requirement with a
  configurable lifetime; add guest renewal behaviour (when to renew, key
  retention, order relative to SPIRE).
- `debian-packaging`: the `devid-enroll` package may ship systemd units/timer
  for renewal (today it ships only the binary and man page).

## Impact

- **Code, after coordinator ACK only:** `internal/devid/ca.go` (+ tests),
  `internal/config`, packaged `debian/config.yaml` comment/default,
  `cmd/devid-enroll` renewal logic, new unit/timer under `debian/` for the
  guest package, man page. Host daemon packaging unchanged beyond the config
  key.
- **SPIRE:** each new LDevID certificate gets a **new** `tpm_devid` agent
  SPIFFE ID (SHA-1 of cert DER), even when the TPM key is reused. Registration
  entries must select on `tpm_devid:subject:cn:<vmid>` (or equivalent), not on
  the fingerprint path. Stale agent records are cleaned by the E2.4 playbook.
- **Not in scope:** intermediate CA per cluster; LDevID subject strip to
  CN-only (E2.5 — separate change); automatic SPIRE agent reconciler;
  changing enroll wire format; host package systemd.
