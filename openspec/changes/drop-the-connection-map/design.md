# Design

## Context

Assigned by the coordinator on 2026-10-18 from his list of what is missing for
production readiness (point 6), for `agent-mac-vtpm-mds-3502f6`. It may stand in
the same pull request as `quiet-the-log-and-record-issuance`, but has a record of
its own.

### Evidence read in this repository (2026-10-18, `main@407bfca`)

- `var connectionMap sync.Map` and `connectionMap.Store(c.TCPConn, vmConfig.VMID)`
  in `internal/server/server.go` are the only two mentions of the name in the
  repository, tests included.
- `vmConnection` carries `vmid` and `vmConfig`, set in `extractVMConfigByRouting`
  right before the `Store`; `connContext` puts `vmConfig` into the request
  context. Handlers read it with `inventory.GetVMConfigFromRequest`.
- The key is the `*net.TCPConn` of the connection, so the map holds a pointer to
  every connection that was ever bound.

## Decided by this record's author, for the maintainer to overrule

- **D-a. Remove the map, do not bound it.** Nothing reads it, so there is nothing to
  bound. A cleanup on close would keep a map that has no reader.
- **D-b. The proof is a test that fails on the old code.** A test opens many
  connections through the real listener and closes them, then checks that the live
  heap did not grow by anything like the number of connections. It is a
  measurement and so has a margin: it would catch a retained connection per
  accept, not a single stray allocation. It is run against the code before this
  change and must fail there.
- **D-c. The measurement on hogan** uses the process's resident size, read from
  `/proc/<pid>/status`, before and after a few thousand connections from VM 399. A
  growth on the old build is not required to be shown (it would need the old
  package installed); the unit test is the counter-proof, the hogan run shows the
  new build is flat.

- **D-d. One seam for the test.** The ARP table is read from the fixed path
  `/proc/net/arp`, which a test on any other host cannot bind a caller through; the
  test needs the caller bound, or the leak does not show. The path becomes a package
  variable, `arpTablePath`, with its present value, so the test can point it at a
  file of its own. No behaviour changes.

## Non-Goals

- **Limiting connections**, `throttle-guest-requests`.
- **Any other cache** in the package: the VM config cache is bounded by the
  inventory and refreshed by SIGHUP.

## Risks

- **None for callers:** the value was never read.
- **The test is statistical.** A generous margin keeps it from failing on a normal
  garbage collection; it would not notice a leak of a few objects.
