# Proposal

## Why

`systemctl stop vtpm-mds` and `systemctl restart vtpm-mds` leave the unit in
`failed`. `main.go` runs the listener in a goroutine and calls `log.Fatalf` on
whatever `srv.Start()` returns, but `http.Server.Serve` returns
`http.ErrServerClosed` as its *normal* outcome after `Shutdown`. Every clean
stop therefore races the graceful-shutdown path, exits 1, and is recorded as a
failure. Verified on 0.2.0 on the lab host.

An operator cannot tell a stop they asked for from a daemon that died, `Restart=on-failure`
treats an administrative stop as a crash on the next boot ordering, and any health
monitoring that reads unit state reports a permanently broken service on a host
where nothing is wrong.

## What Changes

- A shutdown requested by `SIGTERM` or `SIGINT` completes the graceful shutdown
  path and the process exits 0, so the unit ends in `inactive (dead)`.
- `http.ErrServerClosed` is recognised as the expected result of a stopped
  listener and no longer terminates the process by itself.
- A listener error that is *not* a shutdown — a listen address that cannot be
  bound, for example — still logs and exits non-zero, so a genuinely broken
  daemon is still visible as `failed` and still rate limited by the unit's
  existing `StartLimit*` directives.
- No change to what the daemon serves, to the signal set it handles, or to the
  10 second shutdown grace period.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `operability`: adds the process exit contract. The capability today covers
  `-version` reporting and the unit's restart rate limit but says nothing about
  how the daemon terminates, which is why this defect was never contradicted by
  a spec.

## Impact

- `main.go`: the listener goroutine's error handling, and the exit path after
  graceful shutdown.
- `internal/server/server.go`: `Server.Start` is the source of the returned
  error; whether the sentinel is filtered there or in `main.go` is a design
  decision.
- No change to `debian/vtpm-mds.service`. In particular this change does **not**
  add `SuccessExitStatus=1`, which would paper over the defect and also hide
  real failures.
- Operators and any tooling that reads `systemctl is-failed vtpm-mds`.
