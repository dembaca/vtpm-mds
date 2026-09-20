# Spec Delta

## ADDED Requirements

### Requirement: A Requested Shutdown Is Not A Failure

When the daemon receives `SIGTERM` or `SIGINT` it SHALL run its graceful
shutdown path and SHALL terminate with exit status 0.

`http.ErrServerClosed` is the defined result of serving a listener that has
been shut down. The daemon SHALL treat it as the expected outcome of a
requested shutdown and SHALL NOT terminate the process on account of it. In
particular the goroutine that serves the listener SHALL NOT end the process
when it observes that sentinel, so it cannot race the shutdown path and
pre-empt the exit status.

The shipped systemd unit SHALL therefore reach `inactive (dead)` after
`systemctl stop`, and `systemctl restart` SHALL complete without an
intervening failed state. The unit SHALL achieve this without declaring a
non-zero exit status to be a success.

#### Scenario: Stopping the service leaves it inactive

- **GIVEN** a running `vtpm-mds` unit
- **WHEN** an operator runs `systemctl stop vtpm-mds`
- **THEN** the service state is `inactive (dead)`, the result is `success`,
  and `systemctl is-failed vtpm-mds` reports `inactive`

#### Scenario: Restart does not pass through failed

- **GIVEN** a running `vtpm-mds` unit
- **WHEN** an operator runs `systemctl restart vtpm-mds`
- **THEN** the unit is active afterwards and its recorded result is never
  `exit-code`

#### Scenario: Graceful shutdown completes before exit

- **GIVEN** a running daemon
- **WHEN** it receives `SIGTERM`
- **THEN** it logs that it is shutting down, logs that the server stopped
  gracefully, and exits 0

### Requirement: A Listener Failure Is Still A Failure

An error from serving the listener that is not the shutdown sentinel SHALL be
logged and SHALL terminate the process with a non-zero exit status, so that a
daemon which cannot serve is reported as `failed` and is rate limited by the
unit's start-limit directives.

#### Scenario: Listen address cannot be bound

- **GIVEN** a configuration whose `listen_addr` is an address the host cannot
  bind
- **WHEN** the daemon is started
- **THEN** it logs the error, exits non-zero, and the unit enters `failed`

#### Scenario: Repeated startup failure is rate limited

- **GIVEN** a daemon that exits non-zero on every start because it cannot bind
  its listen address
- **WHEN** systemd restarts it under `Restart=on-failure`
- **THEN** the unit's `StartLimitIntervalSec` / `StartLimitBurst` take effect
  and systemd stops retrying
