# Tasks

Implemented in https://github.com/dembaca/vtpm-mds/pull/10 (commit `ffa7ee2`)
alongside `split-debian-host-and-guest-packages`. Written after the fact: the
work was done as recovery of lost uncommitted changes before it was recorded as
an OpenSpec change. Every verification below was run on the lab host.

## 1. Restore the flag on both binaries

- [x] 1.1 Add `-version` to `main.go`, branching before config load, and verify `vtpm-mds -version` prints `vtpm-mds <version>` and exits 0 with no config file present
- [x] 1.2 Add `Version` and `-version` to `cmd/devid-enroll/main.go`, branching before `tpm2.OpenTPM`, and verify `devid-enroll -version` exits 0 as an unprivileged user on a machine with no TPM
- [x] 1.3 Verify build-time injection still reaches both: `go build -ldflags "-X main.Version=0.2.0" ./cmd/devid-enroll` prints `devid-enroll 0.2.0`
- [x] 1.4 Verify the restored code matches the binary already deployed: diff the `-version` hunks against the staging copy of the build that produced the installed package

## 2. Document the flag

- [x] 2.1 Add `-version` to the SYNOPSIS and OPTIONS of `debian/vtpm-mds.8` and verify the page parses (`groff -man -ww -z`)
- [x] 2.2 Add `-version` to the SYNOPSIS and OPTIONS of `debian/devid-enroll.1` and verify the page parses, and that the shipped `.gz` in the guest package contains the option

## 3. Fix the unit's start limit

- [x] 3.1 Move `StartLimitIntervalSec` and `StartLimitBurst` from `[Service]` to `[Unit]` in `debian/vtpm-mds.service` and verify `systemd-analyze verify` reports no ignored directive (it previously said `Unknown key 'StartLimitIntervalSec' in section [Service], ignoring`)

## 4. Verify against the built packages

- [x] 4.1 Extract both `.deb`s and verify `usr/sbin/vtpm-mds -version` and `usr/bin/devid-enroll -version` both print `0.2.0`, matching `dpkg-deb -f … Version`
- [x] 4.2 Verify the `prox-mds` alias symlink prints `vtpm-mds 0.2.0` (same binary, reports the implementation name)
- [x] 4.3 Run `go vet ./...` and `go test ./...` and verify both pass
