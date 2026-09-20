# Proposal

## Why

`getLocalIP` derives the caller's address with
`strings.Split(r.RemoteAddr, ":")[0]`. `RemoteAddr` for an IPv6 peer is
`[fe80::1]:5000`, so splitting on every colon and taking the first field yields
the literal string `[fe80`. A guest reaching the service over IPv6 is served
`[fe80` as its `local-ipv4`, and the same value lands in `privateIp` of the
instance identity document.

That is not a rounding error in a debug field: `privateIp` is one of the two
caller-varying values in a document cloud-init and the AWS SDKs read, and
`[fe80` is neither a valid address nor recognisably a failure. Anything that
parses it fails in the guest, with no indication that the metadata service
produced nonsense.

The same textual split is used wherever the peer address is taken, so the
defect is in one helper rather than in one endpoint.

## What Changes

- The peer address is parsed as a host/port pair rather than split on the first
  colon, so an IPv6 peer yields its actual address and an IPv4 peer is
  unchanged.
- An IPv4-mapped IPv6 peer (`::ffff:10.0.0.5`) is served the IPv4 form
  `10.0.0.5`, because that is the address the guest has.
- **BREAKING** `GET /latest/meta-data/local-ipv4` responds `404` with an empty
  body when the peer has no IPv4 address, matching what `public-ipv4` already
  does when no address is available. It no longer serves a malformed string.
- `privateIp` in the instance identity document is the empty string when the
  peer has no IPv4 address. The key stays present, so the document keeps its
  fixed key set.
- No change to which callers are served, to the token flow, to the index
  listing, or to any other metadata value.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `instance-metadata`: `Serve The EC2 Metadata Tree` — the `local-ipv4`
  definition, which today records the textual truncation and its `[fe80`
  outcome as current behaviour; `Serve An Unsigned Instance Identity Document`
  — the `privateIp` definition, for the same reason.

## Impact

- `imds/handlers.go`: `getLocalIP`, and the peer-address half of `getClientIP`.
- `identity/handlers.go`: its own copy of `getClientIP`, which has the same
  split.
- `imds/handlers_test.go`: gains IPv6 and IPv4-mapped cases.
- Guests that reach the service over IPv6. They cannot be bound to a VM record
  in any case, because `vm-inventory` resolves callers through `/proc/net/arp`,
  which holds IPv4 neighbours only — so in practice this affects what an
  unbound IPv6 caller is served.
- Ordering: `refuse-unbound-metadata-callers` also modifies
  `Serve An Unsigned Instance Identity Document`. That change is written
  against the text this one leaves behind and must be applied after it.
