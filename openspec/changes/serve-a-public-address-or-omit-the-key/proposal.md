# Proposal

## Why

The metadata index advertises a key that no caller can ever read.

`GET /latest/meta-data/` lists `public-ipv4`, and
`GET /latest/meta-data/public-ipv4` answers `404` with an empty body to every
caller, always. The reason is in `imds/handlers.go`:

```go
func getPublicIP(r *http.Request) string {
	// TODO: Extract from Proxmox VM config
	return ""
}
```

`instance-metadata` records both halves as the current contract — the index
listing in `Serve The EC2 Metadata Tree`, and the permanent `404` in the same
requirement, with the scenario `Public address is never available`. So this is
not a contradiction in the spec. It is a missing feature that the spec
faithfully describes, and the describing is what makes the defect visible: a
discovery index whose purpose is to tell a caller what it can read is naming
something it cannot.

The cost is small but real. cloud-init and the AWS SDKs walk the index, and a
listed key that always `404`s is indistinguishable from a transient failure. A
guest cannot tell "this instance has no public address" from "the metadata
service is broken", and the natural reaction to the latter is to retry.

Two things are worth separating, because only one of them is in question:

- **That the service must not advertise what it cannot serve** is not in
  question. Either the key has a source, or it leaves the index.
- **Where a public address would come from** is genuinely open, and it is a
  design question rather than an implementation detail. `design.md` sets out
  the options; it is not settled here.

Real EC2 is the precedent for the first point: an instance with no public
address does not list `public-ipv4` in its index at all. The key's presence is
itself the signal.

## What Changes

- **BREAKING** `GET /latest/meta-data/` no longer lists `public-ipv4`
  unconditionally. The key is listed only when the service has an address to
  serve for the asking caller, so the index again means "these are the keys you
  can read".
- `GET /latest/meta-data/public-ipv4` keeps answering `404` with an empty body
  and `Content-Type: text/plain` when no address is known. That response is
  unchanged, and it stays distinct from the catch-all's `404 page not found`
  and from the `422` that `fix-client-ip-fallback` introduces for an
  underivable instance id.
- A source for the address is added, and **which** source is the open question
  in `design.md`. The recommendation is an explicit per-VM inventory field, not
  a value the service infers.
- The index becomes caller-dependent for the first time. Every other key it
  lists is listed for everyone; this is a deliberate break with that, and
  `design.md` records why it is preferred to the alternative of dropping the
  key entirely.
- No change to any other metadata key, to the token flow, to the index's
  subtree behaviour, or to what a bound caller reads today apart from the index
  line itself.
- `getAvailabilityZone` and `getRegion` carry the same `TODO: Extract from
  Proxmox cluster config` comment but **return real fixed values** that the
  spec pins (`proxmox`, `local`). They are not defective and are explicitly out
  of scope.

## Capabilities

### Modified Capabilities

- `instance-metadata`: `Serve The EC2 Metadata Tree` is modified. Its index
  clause gains the condition under which `public-ipv4` is listed, and its
  `public-ipv4` clause gains the source and the condition under which an
  address is served. The scenario `Public address is never available` inverts,
  and the index scenario changes shape, so both are replaced rather than
  carried forward.

## Impact

- `imds/handlers.go`: `getPublicIP` and `HandleMetaDataIndex`. The index
  handler currently builds a fixed slice and must become caller-dependent,
  which means it needs the same VM-record lookup the other handlers already do.
- `internal/inventory/inventory.go`: the per-VM field, if the recommendation in
  `design.md` is accepted. `VMConfig` already carries
  `RawConfig map[string]string`, described as "Optional key/value metadata", so
  a source may exist without a struct change.
- `internal/proxmox/vmconfig.go`: only if the maintainer picks the Proxmox
  config as the source instead.
- `imds/handlers_test.go`: the index test and the `public-ipv4` test both
  change.
- `config.yaml`, `debian/config.yaml` and `debian/vtpm-mds.8`: only if the
  chosen source is a configuration setting rather than an inventory field.
- An operator whose tooling parses the index and expects a fixed six-line
  listing: it becomes five lines for a VM with no declared public address.
- Ordering: this change edits `imds/handlers.go`, as `fix-client-ip-fallback`
  does. It must be applied **after** that change. The two touch different
  functions — `getPublicIP` and `HandleMetaDataIndex` here, `getClientIP` and
  `InstanceID` there — so the conflict is mechanical rather than semantic, but
  they must not be implemented in parallel.
