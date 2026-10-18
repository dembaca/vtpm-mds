# Tasks

Written on 2026-10-18 against `main` at `407bfca` by `agent-mac-vtpm-mds-3502f6`,
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record.**

## 1. Decide

- [x] 1.1 **Coordinator** — ACK the record and the delta, and verify the ACK is on the bus before any code is written
      **Done, 2026-10-18:** ACK on the bus for #52 at `74c0f35`.

## 2. Implement

- [x] 2.1 Write a test that opens and closes many connections through the real listener and checks that the live heap does not grow with their number, run it against the code before this change, and verify it fails there
      **Done, 2026-10-18** (`d78932a`): `TestClosedConnectionsAreNotRetained` in
      `internal/server/connection_leak_test.go` binds the caller through a test ARP
      table (D-d), checks it is bound to VM 100 (`i-100`), then opens and closes
      3000 connections without keep-alive and compares the live heap objects after
      two collections. **On the tree before the change it fails:** the heap grew by
      25506 objects (25711 in a second run) over 3000 connections, about eight per
      connection; on the new code it grows by 545, which is noise.
- [x] 2.2 Remove `connectionMap` and its `Store` call, and verify the test passes, that a search of the repository finds no `connectionMap`, and that the tests that check which VM a request is bound to still pass
      **Done, 2026-10-18:** `connectionMap` and its `Store` are removed. The test
      passes, a search of the repository for `connectionMap` finds nothing, and every
      other test of the touched packages still passes.
- [x] 2.3 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass
      **Done, 2026-10-18:** `go vet` and `go test` for `.`, `./cmd/devid-enroll`,
      `./imds` and `./internal/...` pass; `openspec validate --all --strict` →
      6 passed; a trial `openspec archive` on a scratch copy applies the delta (+1)
      and leaves 5 passed.

## 3. Verify on the real systems

- [x] 3.1 **Hogan** — install the git-stamped package, note the service's resident size from `/proc/<pid>/status`, open a few thousand connections from VM 399 (each bound), and verify the resident size afterwards is flat within the noise of a garbage collection
      Needs the maintainer's go in the session of whoever runs it, for the load on
      VM 399.
      Done 2026-10-18 on hogan with the maintainer's go, package
      `0.2.0+git20261018.38bd29d4400c` (clean tree). 14 rounds of 5000 bound
      connections from VM 399 (`PUT /latest/api/token`, `Connection: close`,
      all `200`), 70000 in all. `VmRSS` 9668 kB at start, 10100 kB after the
      first requests, then 15856 → 20448 kB over rounds 1–5 while the heap and
      buffers grew, and 22452–26068 kB over rounds 6–14 with drops of about
      3 MB (rounds 7 and 12) where the collector returned memory; `VmHWM`
      26420 kB, threads 9 → 10. Over the last 45000 connections the size is
      flat within that band, not growing with the connection count.
- [x] 3.2 Say what was not verified: the old build under the same load, any other source of growth
      Not verified: the old build (`493063dcb393`, with the map) under the
      same load, so there is no measured growth rate to compare against; no
      heap profile, only `VmRSS`, so caches, threads and config reloads are not
      separated as sources; only the token path, no metadata reads; one guest,
      sequential connections, nothing concurrent.

## 4. Archive

- [x] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists operability, and verify `openspec validate --all --strict` is still clean
      Done 2026-10-18 from `main` at `c2b0617`: `openspec archive` applied the
      operability delta (+1); `AGENTS.md` records the archive.
