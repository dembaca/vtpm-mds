# Design

## Context

Both binaries already carry a `Version` variable set through
`-ldflags "-X main.Version=…"`; `debian/rules` passes
`dpkg-parsechangelog -S Version` and the `Makefile` passes `VERSION?=dev`.
The daemon logs that value at startup. What was missing is a way to read it
without starting the process — which is precisely what an operator needs on a
host where the daemon fails to bind, and the only option at all on a guest
where `devid-enroll` is a one-shot.

Recovered verbatim from the staging copy of the last build that contained it
(archived at `/home/cursor-agent/vtpm-recovered/` on the lab host), so the
restored code is the code that produced the currently installed package rather
than a fresh guess.

## Goals / Non-Goals

**Goals:**

- `-version` on both binaries, before any side effect.
- Packaged binaries report the Debian package version.
- systemd actually applies the restart rate limit.

**Non-Goals:**

- A `--version` long form, a `version` subcommand, or build metadata beyond
  the injected string (no commit, date or Go version).
- Changing how `Version` is injected, or the `Makefile` / `debian/rules`
  ldflags.
- Any change to enroll, metadata, attestation or config behaviour.

## Decisions

### Check the flag before opening the TPM

`devid-enroll` opens `/dev/tpm0` immediately after `flag.Parse()`. The
`-version` branch returns before that, so the flag works on a machine with no
TPM, no root, and no reachable metadata service. The daemon's branch likewise
returns before config loading, so `-version` works with no config file.

### Print `<name> <version>`, not a bare version

`vtpm-mds 0.2.0` and `devid-enroll 0.2.0`. The alias symlinks `prox-mds` and
`qemu-mds` are the same binary, so they print `vtpm-mds`; that is intentional
— it tells the operator which implementation answered, not which name was
typed.

### Start-limit directives belong to `[Unit]`

`StartLimitIntervalSec` and `StartLimitBurst` are unit-level in systemd 229+.
Leaving them under `[Service]` is silently ignored, which is worse than not
setting them: the unit looks rate-limited but is not.

## Risks / Trade-offs

- **[Risk] A future flag added after the `-version` branch would run TPM or
  config code on `-version`** → Mitigation: the branch is the first statement
  after `flag.Parse()` in both binaries.
- **[Risk] `-version` on an unstamped build prints `dev`** → Accepted: that
  is the signal that the binary did not come from a package, which is exactly
  what the lab procedure needs to catch.

## Migration Plan

None. Additive flag; the unit change takes effect on the next
`systemctl daemon-reload`, which the package's maintainer scripts already do.

## Open Questions

None.
