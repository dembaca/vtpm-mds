# Design

## Context

See proposal.md — Why. The mechanics that matter here:

`main.go` starts the listener in a goroutine and calls `log.Fatalf` on the
error from `srv.Start()`. `log.Fatalf` is `os.Exit(1)`, which runs no deferred
functions and cannot be recovered. Meanwhile the main goroutine is blocked on
the signal channel and, on `SIGTERM`, calls `srv.Shutdown(ctx)`.

`Server.Start` is `http.Server.Serve(listener)`, and `Serve` returns
`http.ErrServerClosed` immediately once `Shutdown` closes the listener. So on
every stop both goroutines run at once: `Shutdown` drains connections while the
listener goroutine is already on its way to `os.Exit(1)`. The `log.Fatalf`
almost always wins, because `Shutdown` waits for in-flight requests and then
for `main` to log and return.

This is why the failure is total rather than intermittent, and why the unit is
`failed` even though the shutdown itself is correct.

## Goals / Non-Goals

**Goals:**

- A requested stop exits 0 and the unit ends `inactive (dead)`.
- A listener error that is not a shutdown still exits non-zero.
- The distinction is made from the error itself, not from a flag that the
  shutdown path sets, so there is no second race to get wrong.

**Non-Goals:**

- Changing the shutdown grace period, the signal set, or the `SIGHUP` reload.
- Changing `debian/vtpm-mds.service`. `SuccessExitStatus=1` would make the
  symptom disappear while also making a real crash look like a clean stop.
- Adding a `-foreground`/`-daemon` mode, a PID file, or readiness notification
  (`sd_notify`). Those are separate operability work.
- Draining or bounding in-flight enrollments differently from today.

## Decisions

### Filter the sentinel in `main.go`, not in `Server.Start`

`errors.Is(err, http.ErrServerClosed)` is checked where the goroutine decides
whether to end the process. `Server.Start` keeps returning exactly what
`Serve` returned.

The alternative — having `Server.Start` return `nil` on `ErrServerClosed` —
was rejected because it hides the sentinel from every other caller, including
tests, and turns "the listener stopped" and "the listener was never asked to
stop" into the same result. Keeping the sentinel and interpreting it at the
one place that acts on it is both smaller and more honest.

### Let the main goroutine own the exit

On `ErrServerClosed` the listener goroutine logs at most a debug-level line and
returns. The process then exits through `main` returning after
`srv.Shutdown(ctx)` — the path that already logs `Server stopped gracefully`.
One goroutine owning termination removes the race rather than narrowing it.

### Keep `log.Fatalf` for a real listener error

A daemon that cannot bind its address has nothing useful to do, and the unit's
`Restart=on-failure` plus the `StartLimit*` directives restored by
`restore-binary-version-reporting` are the intended response. Exiting non-zero
from the goroutine is correct there, so that branch is unchanged.

## Risks / Trade-offs

- **[Risk] `Shutdown` returns an error (a connection outlives the 10 second
  context) and the process still exits 0** → Accepted, and unchanged from
  today: the existing code already logs that error and returns. A stop the
  operator asked for is not a service failure just because a client held a
  connection open. The log line remains the record.
- **[Risk] A future `srv.Start()` implementation returns a wrapped
  `ErrServerClosed`** → Mitigation: the check is `errors.Is`, not `==`.
- **[Risk] The fix is verified by reading unit state, which needs root on the
  lab host** → Mitigation: the exit status itself is verifiable without root by
  running the daemon in the foreground on an unprivileged port and sending it
  `SIGTERM`. The `systemctl` scenarios are verified by the maintainer, and
  tasks.md says so rather than implying the check ran.

## Migration Plan

None. No configuration, unit file, or wire behaviour changes; the new exit
status takes effect the first time the new binary is stopped.

## Open Questions

None.
