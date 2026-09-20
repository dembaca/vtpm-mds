# Spec Delta

## ADDED Requirements

### Requirement: Expire Session Tokens On A Validated TTL

The token store SHALL take its lifetime from `mds.token_ttl`, parsed as a Go
duration once when the store is constructed, and SHALL stamp each token with an
expiry of its creation time plus that lifetime. The shipped default is `60s`.

Validation SHALL compare the current time against the stored expiry, so a token
stops being accepted the moment it expires. A background sweep SHALL run every
minute and delete expired entries from the store; that sweep is a memory
reclaim and SHALL NOT be what makes an expired token invalid.

A `mds.token_ttl` that is present but does not parse as a Go duration SHALL be
a fatal configuration error. Loading the configuration SHALL fail, the error
SHALL name the setting and the offending value, and the service SHALL NOT
start. The service SHALL NOT start with a zero token lifetime.

A configuration file that omits `token_ttl` SHALL take the built-in default,
so omission is not a way to reach a zero lifetime either.

#### Scenario: Token is accepted inside its lifetime

- **GIVEN** `mds.token_ttl` is `60s`
- **WHEN** a guest presents a token it minted a moment earlier
- **THEN** the service responds `200` with the metadata value

#### Scenario: Token is refused after its lifetime

- **GIVEN** `mds.token_ttl` is `1ms`
- **WHEN** a guest presents a token 10 milliseconds after minting it
- **THEN** the token no longer validates and the metadata read responds `401`

#### Scenario: Unparseable TTL stops the service from starting

- **GIVEN** a configuration file whose `mds.token_ttl` is a value such as `60`
  or `sixty` that is not a Go duration
- **WHEN** the service is started with that file
- **THEN** it exits non-zero with an error naming `token_ttl` and the offending
  value, and never binds its listen address

#### Scenario: Omitted TTL takes the built-in default

- **GIVEN** a configuration file that does not mention `mds.token_ttl`
- **WHEN** a guest mints a token and immediately presents it on a metadata read
- **THEN** the read responds `200`, because the store was built with the
  built-in `60s` lifetime

### Requirement: Gate The EC2 Metadata Surface On The Effective Configuration

The service SHALL register the `/latest/meta-data/` tree and the
`/latest/dynamic/instance-identity/` endpoints only when
`mds.enable_ec2_compat` is true. When it is false those routes SHALL NOT be
registered, and requests for them SHALL fall through to the catch-all handler,
which SHALL log the unmatched route and respond `404` with the body
`404 page not found`.

Disabling EC2 compatibility SHALL NOT affect `PUT /latest/api/token` or
`GET /health`, both of which are registered unconditionally.

Configuration loaded from a file SHALL start from the service's built-in
defaults and overlay the file's values on top. A setting the file does not
mention SHALL keep its built-in default; a setting the file states explicitly
SHALL win, including one stated as the zero value. A configuration file that
omits `enable_ec2_compat` SHALL therefore leave the metadata tree registered,
because the built-in default is true, and only an explicit
`enable_ec2_compat: false` SHALL disable it.

`listen_addr` SHALL continue to be validated as non-empty. Because it now has
a built-in default, that validation SHALL reject only a value the file states
explicitly as empty.

#### Scenario: Metadata tree is absent when compatibility is off

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `GET /latest/meta-data/instance-id` with a valid token
- **THEN** the service responds `404` with the body `404 page not found`

#### Scenario: Token and health endpoints survive the gate

- **GIVEN** `mds.enable_ec2_compat` is false
- **WHEN** a guest sends `PUT /latest/api/token` and `GET /health`
- **THEN** both respond `200`

#### Scenario: Omitting the setting keeps the tree registered

- **GIVEN** a configuration file that sets `listen_addr` but does not mention
  `enable_ec2_compat`
- **WHEN** the service is started with that file
- **THEN** the metadata tree is registered and a metadata read with a valid
  token responds `200`

#### Scenario: An explicit false still disables the tree

- **GIVEN** a configuration file that sets `enable_ec2_compat: false`
- **WHEN** the service is started with that file
- **THEN** the metadata tree is not registered and metadata reads respond `404`

#### Scenario: An explicitly empty listen address is rejected

- **GIVEN** a configuration file that sets `listen_addr: ""`
- **WHEN** the service is started with that file
- **THEN** loading fails with the `listen_addr is required` error and the
  service does not start

## REMOVED Requirements

### Requirement: Expire Session Tokens On The Configured TTL

**Reason**: Its final paragraph and the scenario
`Unparseable TTL makes every token dead on arrival` specify the defect — an
unparseable or absent `token_ttl` yielding a zero lifetime with the parse error
discarded, so the service starts and then rejects every metadata read. That
behaviour no longer exists, so the scenario cannot be carried forward. Replaced
by `Expire Session Tokens On A Validated TTL`, which keeps the lifetime,
stamping and sweep rules verbatim.

**Migration**: None for a configuration whose `token_ttl` is a valid Go
duration or absent — the TTL behaviour is unchanged. A configuration whose
`token_ttl` does not parse must be corrected before the upgrade, because the
service now refuses to start instead of starting and answering `401`.

### Requirement: Gate The EC2 Metadata Surface On Configuration

**Reason**: Its final paragraph and the scenario
`Omitting the setting from a config file disables the tree` specify the defect
— a file-loaded configuration ignoring the built-in defaults, so an omitted
`enable_ec2_compat` silently disabled the whole metadata tree. The replacement
inverts that outcome, so the scenario cannot be carried forward. Replaced by
`Gate The EC2 Metadata Surface On The Effective Configuration`, which keeps the
gating and the unconditional token and health routes verbatim.

**Migration**: A deployment that relied on omitting `enable_ec2_compat` to keep
the metadata tree off must now state `enable_ec2_compat: false` explicitly. The
shipped `/etc/vtpm-mds/config.yaml` already states it, so a host installed from
the package needs no change.
