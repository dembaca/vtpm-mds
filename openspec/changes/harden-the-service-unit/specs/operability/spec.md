# Spec Delta

## ADDED Requirements

### Requirement: Confine The Daemon To What It Uses

The shipped systemd unit SHALL confine `vtpm-mds` to the resources it uses. It
SHALL limit the capability bounding set to `CAP_NET_BIND_SERVICE`, so that the
daemon, although started as root, cannot read or write files that root does not
own and are not readable by everyone, cannot load kernel modules, and cannot
change kernel tunables. It SHALL mount the file system read-only for the daemon
except for its state directory, deny it devices, new namespaces, set-user-ID
programs and real-time scheduling, restrict its system calls to the
`@system-service` set without `@privileged` and `@resources`, and restrict its
sockets to `AF_INET`, `AF_INET6`, `AF_UNIX` and `AF_NETLINK`. It SHALL deny
traffic to and from every address except link-local IPv4 and loopback.

`systemd-analyze security` SHALL rate the installed unit at **4.0 or lower**.

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

#### Scenario: The address filter is in effect

- **GIVEN** the running service
- **WHEN** an operator runs `systemctl show -p IPAddressDeny -p IPAddressAllow vtpm-mds`
- **THEN** every address is denied except `169.254.0.0/16` and loopback
