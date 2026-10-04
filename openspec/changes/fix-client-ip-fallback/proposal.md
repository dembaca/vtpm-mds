# Proposal

## Why

`imds.getClientIP` returns the literal `"127.0.0.1"` when `PeerIP` cannot parse
`r.RemoteAddr`. With `mds.require_vm_identity` set to false, `InstanceID` builds
the instance id of an unbound caller from that value, so a caller whose peer
address is unparseable is served the invented id `i-127-0-0-1`.

That id is indistinguishable from the one a real caller on the loopback address
gets, and the `instance-metadata` spec records the loopback case as expected
behaviour. A value the service made up because it had nothing to report is
presented as the name of a connection. The same invented id reaches `instanceId`
in the instance identity document and the `sub` and `instance_id` claims of
`/latest/identity`.

`parse-peer-address-correctly` replaced a textual split that produced a
fragment (`[fe80`) with a parser that produces nothing when it cannot parse. The
`127.0.0.1` fallback is the last place where "nothing" is turned back into a
plausible value. It is reachable only with `require_vm_identity: false` — the
default refuses an unbound caller first — so it is a defect of the opt-out
path, found and reproduced while specifying the queue and still unwritten.

## What Changes

- `InstanceID` reports that no id can be derived, instead of inventing one, when
  no VM record is bound and the peer address cannot be parsed.
- **BREAKING** With `mds.require_vm_identity` false, `GET
  /latest/meta-data/instance-id`, `GET /latest/dynamic/instance-identity/document`
  and `GET /latest/identity` answer `404` with the body `404 page not found` to
  such a caller — the same bytes the catch-all and the `require_vm_identity`
  refusal return — instead of `200` with `i-127-0-0-1`.
- `getClientIP` no longer returns `127.0.0.1` for a peer it cannot parse.
- A caller that genuinely connects from `127.0.0.1` is unchanged and still
  receives `i-127-0-0-1`.
- No change to bound callers, to the default configuration, to `local-ipv4`,
  `privateIp`, the signature endpoint or the token flow.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `instance-metadata`: `Derive The Instance Id From The Bound VM Record` — the
  fallback branch, which gains the unparseable-peer outcome;
  `Serve An Unsigned Instance Identity Document` — what the document does when
  no instance id exists.
- `workload-identity`: not modified in text. `/latest/identity` already derives
  its subject "exactly as the metadata instance id is", so it follows; the
  design records this so it is not a surprise.

## Impact

- `imds/handlers.go`: `getClientIP`, `InstanceID`, `HandleInstanceID`,
  `HandleInstanceIdentityDocument`.
- `identity/handlers.go`: the `/latest/identity` handler, which calls
  `imds.InstanceID`.
- `imds/handlers_test.go` and the identity tests: unparseable-peer cases.
- Only deployments that set `mds.require_vm_identity: false` *and* receive a
  connection whose `RemoteAddr` does not parse. Over the TCP listener this
  service uses, that is not expected to happen in production, which is why the
  fix is about not asserting something false rather than about a live outage.
- Ordering: independent of the other open work, but it edits `imds/handlers.go`
  like the unwritten `public-ipv4` change; the two implementations must not run
  in parallel.
