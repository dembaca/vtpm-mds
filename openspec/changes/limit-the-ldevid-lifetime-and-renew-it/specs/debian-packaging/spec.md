## MODIFIED Requirements

### Requirement: Guest Package Ships The Client Only

The `devid-enroll` binary package SHALL install the guest enrollment client at
`/usr/bin/devid-enroll`, its man page, and the systemd oneshot and timer that
run the LDevID renewal check on boot and daily.

It SHALL NOT install the metadata daemon, the `[REDACTED]` systemd unit, host
configuration under `/etc/[REDACTED]`, or `/var/lib/[REDACTED]`.

Installing `devid-enroll` SHALL NOT enable or start an IMDS listener. Installing
it SHALL enable the renewal timer when a systemd manager is available.

#### Scenario: Guest package contents

- **WHEN** `devid-enroll` is installed on a guest
- **THEN** `/usr/bin/devid-enroll` is present, the renewal oneshot and timer
  units are present, and `/usr/sbin/[REDACTED]` is absent

#### Scenario: Guest install does not start the host daemon

- **WHEN** `devid-enroll` is installed on a machine with no `[REDACTED]` package
- **THEN** no `[REDACTED]` systemd unit is installed and nothing binds
  `169.254.169.1:80` as a result of that install

#### Scenario: Renewal timer is enabled

- **GIVEN** a guest with a running systemd manager
- **WHEN** `devid-enroll` is installed or upgraded
- **THEN** the renewal timer is enabled
