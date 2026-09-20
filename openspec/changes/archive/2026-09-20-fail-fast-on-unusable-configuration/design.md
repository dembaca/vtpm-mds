# Design

## Context

See proposal.md — Why. The two defects share a shape: an input the operator got
wrong is absorbed instead of reported.

`config.Load` declares `var cfg Config` and unmarshals into it, so every key the
file omits keeps Go's zero value — `false` for the two feature gates, `""` for
every path and duration. `DefaultConfig()` is only ever used on the other
branch in `main.go`, when no `-config` is given.

`NewTokenStore` writes `ttl, _ := time.ParseDuration(cfg.MDS.TokenTTL)`. It has
no error return, and it is called from `server.New`, which does have one — but
the value is already lost by then.

## Goals / Non-Goals

**Goals:**

- A configuration file expresses differences from the defaults, not the whole
  configuration.
- A setting an operator wrote wrong stops the service, at startup, with a
  message that names the setting.
- An operator can still state a zero value explicitly and have it honoured.

**Non-Goals:**

- Validating settings beyond `listen_addr` and `token_ttl`. `jwt_ttl` in
  particular is parsed nowhere today — the identity handler hardcodes five
  minutes — so validating it would reject configurations that currently work
  for a value nothing reads. That belongs with the unwritten work on real
  signing keys for the identity JWTs.
- Checking that paths exist or are readable at load time. The DevID and EK CA
  paths are already checked where they are used, and they are allowed to be
  absent.
- Environment variable or command-line overrides of individual settings.
- Moving the configuration-loading contract out of the `instance-metadata`
  capability, although it governs all of them. Worth its own change.

## Decisions

### Unmarshal onto `DefaultConfig()` rather than merging field by field

`Load` builds `cfg := DefaultConfig()` and unmarshals the file into it.
`gopkg.in/yaml.v3` writes only the keys the document actually contains, so an
omitted key keeps the default and a present key overwrites it — including
`enable_ec2_compat: false`, which is exactly the case a field-by-field merge
cannot express.

The alternative, unmarshalling into a zero value and then copying non-zero
fields over the defaults, was rejected for that reason: it cannot distinguish
"absent" from "explicitly false" or "explicitly empty", which is the same class
of bug in a new place.

### Validate `token_ttl` in `Load`, not in `NewTokenStore`

`Load` already returns an error and `main.go` already aborts on it, so the
error reaches the operator with no new plumbing and with the configuration file
in hand. `NewTokenStore` has no error return, and giving it one would push the
failure through `server.New` into a path whose job is not configuration.

The parse therefore happens twice — once to validate, once to build the store.
That is a negligible cost at startup and keeps each function's contract intact.

`DefaultConfig()` bypasses `Load` entirely when no `-config` is given. Its
`token_ttl` is the constant `60s`, so that path needs no validation.

### `listen_addr` keeps its validation but changes meaning

Today an omitted `listen_addr` fails the load. After the merge it takes the
built-in `169.254.169.1:80`, and the check fires only for an explicitly empty
value. Keeping the check costs nothing and still catches `listen_addr: ""`,
which is otherwise a daemon that binds every interface on port 0.

## Risks / Trade-offs

- **[Risk] An existing deployment that relies on omitting `enable_ec2_compat`
  to keep the metadata tree off would silently gain it on upgrade** →
  Mitigation: the shipped `/etc/vtpm-mds/config.yaml` states the setting
  explicitly, so a host installed from the package is unaffected; the change is
  marked BREAKING in the proposal so it reaches the changelog. Anyone who
  disabled the tree by omission did so without being able to tell, since
  `/health` reported ok either way.
- **[Risk] A host whose `token_ttl` is wrong today starts fine and will refuse
  to start after the upgrade** → Accepted. That host is already serving `401`
  to every metadata read, so it is not working; failing at startup with a
  message naming `token_ttl` converts an invisible outage into a visible one
  with the fix in the error text. `dpkg -i --force-confold` keeps the
  operator's file, so the failure is reproducible and the fix is a one-line
  edit.
- **[Trade-off] Defaults now reach settings the operator never wrote, including
  the DevID and EK CA paths** → Accepted, and it is what the built-in defaults
  are for. The paths are checked at use, and a missing DevID CA already
  disables the enroll routes with a logged warning rather than failing.

## Migration Plan

Deploy with `dpkg -i --force-confold` as usual, then confirm the daemon came
back up. If it did not, the log names the setting to fix. Rollback is
reinstalling the previous package; no state, on-disk format or wire format
changes, so a downgrade needs no cleanup.

## Open Questions

None.
