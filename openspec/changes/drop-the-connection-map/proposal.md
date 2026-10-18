# Proposal

## Why

**Every connection from a bound guest leaves an entry behind that nothing reads and
nothing removes.** `extractVMConfigByRouting` in `internal/server/server.go` ends a
successful lookup with `connectionMap.Store(c.TCPConn, vmConfig.VMID)`. The map
`connectionMap` is a package-level `sync.Map` and these two lines are the only
places it appears: no `Load`, no `Range`, no `Delete` (found by the coordinator in
his production-readiness pass of 2026-10-18, point 6, and confirmed by a search of
the repository). The key is the connection itself, so a closed connection is kept
reachable, with its descriptor object, for the life of the process. A guest that
opens connections in a loop makes the service grow until it is restarted, and
nothing in the journal says so.

The map is also dead weight: the bound VM is already carried by the connection
(`vmConnection.vmid` and `vmConnection.vmConfig`) and handed to handlers through the
request context.

## What Changes

- **The `connectionMap` and its `Store` call are removed.** Nothing else changes: the
  record is still bound to the connection and still reaches handlers through the
  request context.
- **The ARP table's path becomes a package variable** so a test can bind a caller
  on any host; its value is unchanged.
- **A new requirement says what was always intended:** the service keeps no
  per-connection state once a connection closes.

## Capabilities

### Modified Capabilities

- `operability`: one requirement added, no state outlives the connection that
  created it.

## Impact

- **Code, after the coordinator's ACK:** `internal/server/server.go` (the variable
  and one line) and a test. No behaviour a caller can see.
- **Not in scope:** the per-peer limits that bound how fast connections can be
  opened (`throttle-guest-requests`), the access line and the debug lines
  (`quiet-the-log-and-record-issuance`), and the token map's size, which is
  throttling's.
