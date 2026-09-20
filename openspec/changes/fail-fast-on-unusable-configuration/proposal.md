# Proposal

## Why

Two defects in configuration loading turn a typo into a service that starts,
reports healthy, and answers nothing useful.

`config.Load` unmarshals the file into a zero-valued `Config` and never merges
the built-in defaults. A configuration file that omits `enable_ec2_compat`
therefore leaves it `false` and the entire `/latest/meta-data/` tree is never
registered — while `GET /health` still answers `{"status":"ok"}`, because it
reports only that the listener is up. The same applies to every other setting
the file does not mention.

`NewTokenStore` discards the error from `time.ParseDuration(cfg.MDS.TokenTTL)`.
A `token_ttl` of `60` or `sixty` instead of `60s` yields a zero lifetime, so
every minted token is expired the instant it is returned: `PUT /latest/api/token`
answers `200` and every metadata read that follows answers `401`.

In both cases the daemon looks healthy from the outside and the configuration
file looks reasonable to a reader. The failure surfaces in the guest, far from
its cause.

## What Changes

- **BREAKING** `config.Load` starts from the built-in defaults and overlays the
  file on top, so a setting the file omits keeps its default instead of its zero
  value. A configuration file that does not mention `enable_ec2_compat` now
  leaves the metadata tree registered.
- **BREAKING** A `token_ttl` that does not parse as a Go duration is a startup
  error. The service refuses to start and names the setting and the offending
  value, instead of starting with a zero lifetime.
- An omitted `token_ttl` takes the built-in `60s` rather than being empty, which
  follows from the defaults merge.
- `listen_addr` stays validated. It can no longer be empty by omission, so the
  existing "listen_addr is required" error now reports only an explicitly empty
  value.
- No change to the token wire format, to the TTL a caller may request (still
  ignored), to the metadata endpoints themselves, or to `/health`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `instance-metadata`: two requirements change outcome, not just wording, so
  each is removed and replaced under a name that states the new contract.
  `Expire Session Tokens On The Configured TTL` becomes
  `Expire Session Tokens On A Validated TTL`, and
  `Gate The EC2 Metadata Surface On Configuration` becomes
  `Gate The EC2 Metadata Surface On The Effective Configuration`. In both cases
  a scenario that specifies the defect — `Unparseable TTL makes every token
  dead on arrival` and `Omitting the setting from a config file disables the
  tree` — describes behaviour that will no longer exist and cannot be carried
  forward; everything else is kept verbatim.

## Impact

- `internal/config/config.go`: `Load` merges onto `DefaultConfig()` and
  validates `token_ttl`.
- `imds/token.go`: `NewTokenStore` may assume a parsed duration; the discarded
  error moves to load time.
- `main.go`: a `config.Load` failure already aborts startup, so the new error
  needs no new handling — only a message an operator can act on.
- `internal/config/config_test.go` asserts today that a file omitting
  `enable_ec2_compat` yields `false`. That assertion encodes the defect and is
  replaced.
- The `devid-enrollment` capability inherits the defaults merge: a file that
  omits `enable_tpm_attestation` or the DevID CA paths now gets the built-in
  values rather than empty ones. Its requirements are written against the
  *effective* values (`when enable_tpm_attestation is true`), so they stay
  accurate and need no delta. The shipped `/etc/vtpm-mds/config.yaml` sets all
  of these explicitly, so the packaged default configuration is unaffected.
- Structural note, deliberately not addressed here: the contract for how
  configuration is loaded currently lives inside an `instance-metadata`
  requirement, although it governs every capability. Extracting it into a
  capability of its own is worth a separate change.
