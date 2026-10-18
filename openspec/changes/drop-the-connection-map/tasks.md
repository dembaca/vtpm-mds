# Tasks

Written on 2026-10-18 against `main` at `407bfca` by `agent-mac-vtpm-mds-3502f6`,
at the coordinator's assignment. **Code only after the coordinator's ACK of this
record.**

## 1. Decide

- [ ] 1.1 **Coordinator** — ACK the record and the delta, and verify the ACK is on the bus before any code is written

## 2. Implement

- [ ] 2.1 Write a test that opens and closes many connections through the real listener and checks that the live heap does not grow with their number, run it against the code before this change, and verify it fails there
- [ ] 2.2 Remove `connectionMap` and its `Store` call, and verify the test passes, that a search of the repository finds no `connectionMap`, and that the tests that check which VM a request is bound to still pass
- [ ] 2.3 Run `go vet` and `go test` on the packages touched, and `openspec validate --all --strict`, and verify all pass

## 3. Verify on the real systems

- [ ] 3.1 **Hogan** — install the git-stamped package, note the service's resident size from `/proc/<pid>/status`, open a few thousand connections from VM 399 (each bound), and verify the resident size afterwards is flat within the noise of a garbage collection
      Needs the maintainer's go in the session of whoever runs it, for the load on
      VM 399.
- [ ] 3.2 Say what was not verified: the old build under the same load, any other source of growth

## 4. Archive

- [ ] 4.1 Archive with `openspec archive`, update `AGENTS.md` where it lists operability, and verify `openspec validate --all --strict` is still clean
