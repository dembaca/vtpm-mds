# Spec Delta

## MODIFIED Requirements

### Requirement: Confine The Daemon To What It Uses

The shipped systemd unit SHALL confine `vtpm-mds` to the resources it uses. It
SHALL run the daemon as the system user `vtpm-mds` with the supplementary group
`www-data`, never as root, and SHALL limit the capability bounding set to
`CAP_NET_BIND_SERVICE`, so that the daemon reads only what that user or its
groups may read: the Proxmox cluster files (`root:www-data 0640`) but not
`/etc/pve/priv/`. It cannot load kernel modules or change kernel tunables. The
CA key SHALL reach it as a systemd credential, without a change to the key
file's owner or mode. It SHALL mount the file system read-only for the daemon
except for its state directory, deny it devices, new namespaces, set-user-ID
programs and real-time scheduling, restrict its system calls to the
`@system-service` set without `@privileged` and `@resources`, and restrict its
sockets to `AF_INET`, `AF_INET6` and `AF_UNIX`; a system call the filter blocks
SHALL fail with `EPERM` rather than end the process. It SHALL deny
traffic to and from every address except link-local IPv4 and loopback.

`systemd-analyze security` of systemd 257 SHALL rate the installed unit at
**4.0 or lower**. The unit SHALL be part of `pve-cluster.service`, so that a
restart of the cluster file system restarts the daemon with a fresh read-only
view of `/etc/pve`.

#### Scenario: The installed unit is rated OK

- **GIVEN** the host package installed and the service started
- **WHEN** an operator runs `systemd-analyze security vtpm-mds`
- **THEN** the overall exposure is 4.0 or lower

#### Scenario: The daemon cannot use root's file privileges

- **GIVEN** the running service
- **WHEN** its capability sets are read from `/proc/<pid>/status`
- **THEN** the bounding set holds `CAP_NET_BIND_SERVICE` and nothing else

#### Scenario: Metadata and enrollment still work under the confinement

- **GIVEN** the confined service on a Proxmox host with a bound guest
- **WHEN** the guest reads the metadata tree and enrolls a DevID
- **THEN** both succeed as before, and a reload rereads the guest configs

#### Scenario: The daemon runs as its own user

- **GIVEN** the running service
- **WHEN** its credentials are read from `/proc/<pid>/status`
- **THEN** its user is `vtpm-mds`, its groups are `vtpm-mds` and `www-data`, and
  it cannot list `/etc/pve/priv/`

#### Scenario: The CA key stays readable

- **GIVEN** the confined service on a host whose CA key is `root:root 0600` and
  is passed as a systemd credential
- **WHEN** a bound guest enrolls
- **THEN** a certificate is issued, and the journal holds no `DevID enrollment
  disabled` line

#### Scenario: The address filter is configured

- **GIVEN** the running service
- **WHEN** an operator runs `systemctl show -p IPAddressDeny -p IPAddressAllow vtpm-mds`
- **THEN** every address is denied except `169.254.0.0/16` and loopback
